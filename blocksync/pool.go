package blocksync

import (
   "errors"
   "fmt"
   "math"
   "sync/atomic"
   "time"

   flow "github.com/baron-chain/cometbft-bc/libs/flowrate" 
   "github.com/baron-chain/cometbft-bc/libs/log"
   "github.com/baron-chain/cometbft-bc/libs/service"
   cmtsync "github.com/baron-chain/cometbft-bc/libs/sync"
   "github.com/baron-chain/cometbft-bc/p2p"
   "github.com/baron-chain/cometbft-bc/types"
)

const (
   requestIntervalMS = 2
   maxTotalRequesters = 600
   maxPendingRequests = maxTotalRequesters 
   maxPendingRequestsPerPeer = 20
   requestRetrySeconds = 30
   minRecvRate = 7680
   maxDiffBetweenCurrentAndReceivedBlockHeight = 100
)

var peerTimeout = 15 * time.Second

type BlockPool struct {
   service.BaseService
   startTime time.Time

   mtx cmtsync.Mutex
   requesters map[int64]*bpRequester
   height int64
   peers map[p2p.ID]*bpPeer
   maxPeerHeight int64

   numPending int32
   requestsCh chan<- BlockRequest
   errorsCh chan<- peerError
}

func NewBlockPool(start int64, requestsCh chan<- BlockRequest, errorsCh chan<- peerError) *BlockPool {
   bp := &BlockPool{
       peers: make(map[p2p.ID]*bpPeer),
       requesters: make(map[int64]*bpRequester),
       height: start,
       numPending: 0,
       requestsCh: requestsCh, 
       errorsCh: errorsCh,
   }
   bp.BaseService = *service.NewBaseService(nil, "BlockPool", bp)
   return bp
}

func (pool *BlockPool) OnStart() error {
   go pool.makeRequestersRoutine()
   pool.startTime = time.Now()
   return nil
}

func (pool *BlockPool) makeRequestersRoutine() {
   for pool.IsRunning() {
       _, numPending, lenRequesters := pool.GetStatus()
       switch {
       case numPending >= maxPendingRequests:
           time.Sleep(requestIntervalMS * time.Millisecond)
           pool.removeTimedoutPeers()
       case lenRequesters >= maxTotalRequesters:
           time.Sleep(requestIntervalMS * time.Millisecond)
           pool.removeTimedoutPeers()
       default:
           pool.makeNextRequester()
       }
   }
}

func (pool *BlockPool) removeTimedoutPeers() {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   for _, peer := range pool.peers {
       if !peer.didTimeout && peer.numPending > 0 {
           curRate := peer.recvMonitor.Status().CurRate
           if curRate != 0 && curRate < minRecvRate {
               err := errors.New("peer sending data too slowly")
               pool.sendError(err, peer.id)
               peer.didTimeout = true
           }
       }
       if peer.didTimeout {
           pool.removePeer(peer.id)
       }
   }
}

func (pool *BlockPool) GetStatus() (height int64, numPending int32, lenRequesters int) {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()
   return pool.height, atomic.LoadInt32(&pool.numPending), len(pool.requesters)
}

func (pool *BlockPool) IsCaughtUp() bool {
   pool.mtx.Lock() 
   defer pool.mtx.Unlock()

   if len(pool.peers) == 0 {
       return false
   }

   receivedBlockOrTimedOut := pool.height > 0 || time.Since(pool.startTime) > 5*time.Second
   ourChainIsLongestAmongPeers := pool.maxPeerHeight == 0 || pool.height >= (pool.maxPeerHeight-1)
   return receivedBlockOrTimedOut && ourChainIsLongestAmongPeers
}

func (pool *BlockPool) PeekTwoBlocks() (first *types.Block, second *types.Block) {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   if r := pool.requesters[pool.height]; r != nil {
       first = r.getBlock()
   }
   if r := pool.requesters[pool.height+1]; r != nil {
       second = r.getBlock()
   }
   return
}

func (pool *BlockPool) PopRequest() {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   if r := pool.requesters[pool.height]; r != nil {
       if err := r.Stop(); err != nil {
           pool.Logger.Error("Error stopping requester", "err", err)
       }
       delete(pool.requesters, pool.height)
       pool.height++
   } else {
       panic(fmt.Sprintf("Expected requester at height %v", pool.height))
   }
}

func (pool *BlockPool) RedoRequest(height int64) p2p.ID {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   if request := pool.requesters[height]; request != nil {
       if peerID := request.getPeerID(); peerID != p2p.ID("") {
           pool.removePeer(peerID)
           return peerID
       }
   }
   return p2p.ID("")
}

func (pool *BlockPool) AddBlock(peerID p2p.ID, block *types.Block, blockSize int) {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   requester := pool.requesters[block.Height]
   if requester == nil {
       diff := pool.height - block.Height
       if diff < 0 {
           diff *= -1
       }
       if diff > maxDiffBetweenCurrentAndReceivedBlockHeight {
           pool.sendError(errors.New("unexpected block height too far ahead/behind"), peerID)
       }
       return
   }

   if requester.setBlock(block, peerID) {
       atomic.AddInt32(&pool.numPending, -1)
       if peer := pool.peers[peerID]; peer != nil {
           peer.decrPending(blockSize)
       }
   } else {
       pool.sendError(errors.New("invalid peer"), peerID)
   }
}

func (pool *BlockPool) MaxPeerHeight() int64 {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()
   return pool.maxPeerHeight
}

func (pool *BlockPool) SetPeerRange(peerID p2p.ID, base int64, height int64) {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   peer := pool.peers[peerID]
   if peer != nil {
       peer.base = base
       peer.height = height
   } else {
       peer = newBPPeer(pool, peerID, base, height)
       peer.setLogger(pool.Logger.With("peer", peerID))
       pool.peers[peerID] = peer
   }

   if height > pool.maxPeerHeight {
       pool.maxPeerHeight = height
   }
}

func (pool *BlockPool) RemovePeer(peerID p2p.ID) {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()
   pool.removePeer(peerID)
}

func (pool *BlockPool) removePeer(peerID p2p.ID) {
   for _, requester := range pool.requesters {
       if requester.getPeerID() == peerID {
           requester.redo(peerID)
       }
   }

   if peer, ok := pool.peers[peerID]; ok {
       if peer.timeout != nil {
           peer.timeout.Stop()
       }

       delete(pool.peers, peerID)

       if peer.height == pool.maxPeerHeight {
           pool.updateMaxPeerHeight()
       }
   }
}

func (pool *BlockPool) updateMaxPeerHeight() {
   var max int64
   for _, peer := range pool.peers {
       if peer.height > max {
           max = peer.height
       }
   }
   pool.maxPeerHeight = max
}

func (pool *BlockPool) pickIncrAvailablePeer(height int64) *bpPeer {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   for _, peer := range pool.peers {
       if peer.didTimeout {
           pool.removePeer(peer.id)
           continue
       }
       if peer.numPending >= maxPendingRequestsPerPeer {
           continue
       }
       if height < peer.base || height > peer.height {
           continue
       }
       peer.incrPending()
       return peer
   }
   return nil
}

func (pool *BlockPool) makeNextRequester() {
   pool.mtx.Lock()
   defer pool.mtx.Unlock()

   nextHeight := pool.height + pool.requestersLen()
   if nextHeight > pool.maxPeerHeight {
       return
   }

   request := newBPRequester(pool, nextHeight)
   pool.requesters[nextHeight] = request
   atomic.AddInt32(&pool.numPending, 1)

   if err := request.Start(); err != nil {
       request.Logger.Error("Error starting request", "err", err)
   }
}

func (pool *BlockPool) requestersLen() int64 {
   return int64(len(pool.requesters))
}

func (pool *BlockPool) sendRequest(height int64, peerID p2p.ID) {
   if pool.IsRunning() {
       pool.requestsCh <- BlockRequest{height, peerID}
   }
}

func (pool *BlockPool) sendError(err error, peerID p2p.ID) {
   if pool.IsRunning() {
       pool.errorsCh <- peerError{err, peerID}
   }
}

type bpPeer struct {
   didTimeout bool
   numPending int32
   height int64
   base int64 
   pool *BlockPool
   id p2p.ID
   recvMonitor *flow.Monitor
   timeout *time.Timer
   logger log.Logger
}

func newBPPeer(pool *BlockPool, peerID p2p.ID, base int64, height int64) *bpPeer {
   return &bpPeer{
       pool: pool,
       id: peerID,
       base: base,
       height: height,
       numPending: 0,
       logger: log.NewNopLogger(),
   }
}

func (peer *bpPeer) setLogger(l log.Logger) {
   peer.logger = l
}

func (peer *bpPeer) resetMonitor() {
   peer.recvMonitor = flow.New(time.Second, time.Second*40)
   initialValue := float64(minRecvRate) * math.E
   peer.recvMonitor.SetREMA(initialValue)
}

func (peer *bpPeer) resetTimeout() {
   if peer.timeout == nil {
       peer.timeout = time.AfterFunc(peerTimeout, peer.onTimeout)
   } else {
       peer.timeout.Reset(peerTimeout)
   }
}

func (peer *bpPeer) incrPending() {
   if peer.numPending == 0 {
       peer.resetMonitor()
       peer.resetTimeout()
   }
   peer.numPending++
}

func (peer *bpPeer) decrPending(recvSize int) {
   peer.numPending--
   if peer.numPending == 0 {
       peer.timeout.Stop()
   } else {
       peer.recvMonitor.Update(recvSize)
       peer.resetTimeout()
   }
}

func (peer *bpPeer) onTimeout() {
   peer.pool.mtx.Lock()
   defer peer.pool.mtx.Unlock()

   err := errors.New("peer timeout - no data received")
   peer.pool.sendError(err, peer.id)
   peer.didTimeout = true
}

type bpRequester struct {
   service.BaseService
   pool *BlockPool
   height int64
   gotBlockCh chan struct{}
   redoCh chan p2p.ID

   mtx cmtsync.Mutex
   peerID p2p.ID
   block *types.Block
}

func newBPRequester(pool *BlockPool, height int64) *bpRequester {
   bpr := &bpRequester{
       pool: pool,
       height: height,
       gotBlockCh: make(chan struct{}, 1),
       redoCh: make(chan p2p.ID, 1),
       peerID: "",
       block: nil,
   }
   bpr.BaseService = *service.NewBaseService(nil, "bpRequester", bpr)
   return bpr
}

func (bpr *bpRequester) OnStart() error {
   go bpr.requestRoutine()
   return nil
}

func (bpr *bpRequester) setBlock(block *types.Block, peerID p2p.ID) bool {
   bpr.mtx.Lock()
   if bpr.block != nil || bpr.peerID != peerID {
       bpr.mtx.Unlock()
       return false
   }
   bpr.block = block
   bpr.mtx.Unlock()

   select {
   case bpr.gotBlockCh <- struct{}{}:
   default:
   }
   return true
}

func (bpr *bpRequester) getBlock() *types.Block {
   bpr.mtx.Lock()
   defer bpr.mtx.Unlock()
   return bpr.block
}

func (bpr *bpRequester) getPeerID() p2p.ID {
   bpr.mtx.Lock()
   defer bpr.mtx.Unlock()
   return bpr.peerID
}

func (bpr *bpRequester) reset() {
   bpr.mtx.Lock()
   defer bpr.mtx.Unlock()

   if bpr.block != nil {
       atomic.AddInt32(&bpr.pool.numPending, 1)
   }

   bpr.peerID = ""
   bpr.block = nil
}

func (bpr *bpRequester) redo(peerID p2p.ID) {
   select {
   case bpr.redoCh <- peerID:
   default:
   }
}

func (bpr *bpRequester) requestRoutine() {
   for {
       var peer *bpPeer
       for {
           if !bpr.IsRunning() || !bpr.pool.IsRunning() {
               return
           }
           peer = bpr.pool.pickIncrAvailablePeer(bpr.height)
           if peer != nil {
               break
           }
           time.Sleep(requestIntervalMS * time.Millisecond)
       }

       bpr.mtx.Lock()
       bpr.peerID = peer.id
       bpr.mtx.Unlock()

       timeout := time.NewTimer(requestRetrySeconds * time.Second)
       bpr.pool.sendRequest(bpr.height, peer.id)

       for {
           select {
           case <-bpr.pool.Quit():
               if err := bpr.Stop(); err != nil {
                   bpr.Logger.Error("Error stopping requester", "err", err)
               }
               return 
           case <-bpr.Quit():
		   return
           case <-timeout.C:
               bpr.Logger.Debug("Request timeout - retrying", "height", bpr.height, "peer", bpr.peerID)
               bpr.reset()
               break

           case peerID := <-bpr.redoCh:
               if peerID == bpr.peerID {
                   bpr.reset()
                   break
               }
               continue

           case <-bpr.gotBlockCh:
               continue
           }
           break
       }
   }
}

type BlockRequest struct {
   Height int64
   PeerID p2p.ID
}

type peerError struct {
   err error
   peerID p2p.ID 
}
