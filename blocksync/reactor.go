package blocksync

import (
   "fmt"
   "reflect"
   "time"

   "github.com/baron-chain/cometbft-bc/libs/log"
   "github.com/baron-chain/cometbft-bc/p2p"
   bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/blocksync"
   sm "github.com/baron-chain/cometbft-bc/state"
   "github.com/baron-chain/cometbft-bc/store"
   "github.com/baron-chain/cometbft-bc/types"
)

const (
   BlocksyncChannel = byte(0x40)
   trySyncIntervalMS = 10
   statusUpdateIntervalSeconds = 10
   switchToConsensusIntervalSeconds = 1
)

type consensusReactor interface {
   SwitchToConsensus(state sm.State, skipWAL bool)
}

type peerError struct {
   err    error
   peerID p2p.ID
}

func (e peerError) Error() string {
   return fmt.Sprintf("peer %v error: %s", e.peerID, e.err.Error())
}

type Reactor struct {
   p2p.BaseReactor

   initialState sm.State
   blockExec   *sm.BlockExecutor
   store       *store.BlockStore
   pool        *BlockPool
   blockSync   bool

   requestsCh  <-chan BlockRequest
   errorsCh    <-chan peerError
}

func NewReactor(state sm.State, blockExec *sm.BlockExecutor, store *store.BlockStore, blockSync bool) *Reactor {
   if state.LastBlockHeight != store.Height() {
       panic(fmt.Sprintf("state height %v != store height %v", state.LastBlockHeight, store.Height()))
   }

   requestsCh := make(chan BlockRequest, maxTotalRequesters)
   errorsCh := make(chan peerError, 1000)

   startHeight := store.Height() + 1
   if startHeight == 1 {
       startHeight = state.InitialHeight
   }

   pool := NewBlockPool(startHeight, requestsCh, errorsCh)

   bcR := &Reactor{
       initialState: state,
       blockExec:    blockExec,
       store:        store,
       pool:         pool,
       blockSync:    blockSync,
       requestsCh:   requestsCh,
       errorsCh:     errorsCh,
   }
   bcR.BaseReactor = *p2p.NewBaseReactor("Reactor", bcR)
   return bcR
}

func (bcR *Reactor) SetLogger(l log.Logger) {
   bcR.BaseService.Logger = l
   bcR.pool.Logger = l
}

func (bcR *Reactor) OnStart() error {
   if bcR.blockSync {
       if err := bcR.pool.Start(); err != nil {
           return err
       }
       go bcR.poolRoutine(false)
   }
   return nil
}

func (bcR *Reactor) SwitchToBlockSync(state sm.State) error {
   bcR.blockSync = true
   bcR.initialState = state
   bcR.pool.height = state.LastBlockHeight + 1

   if err := bcR.pool.Start(); err != nil {
       return err
   }
   go bcR.poolRoutine(true)
   return nil
}

func (bcR *Reactor) OnStop() {
   if bcR.blockSync {
       if err := bcR.pool.Stop(); err != nil {
           bcR.Logger.Error("Error stopping pool", "err", err)
       }
   }
}

func (bcR *Reactor) GetChannels() []*p2p.ChannelDescriptor {
   return []*p2p.ChannelDescriptor{
       {
           ID:                  BlocksyncChannel,
           Priority:           5,
           SendQueueCapacity:  1000,
           RecvBufferCapacity: 50 * 4096,
           RecvMessageCapacity: MaxMsgSize,
           MessageType:        &bcproto.Message{},
       },
   }
}

func (bcR *Reactor) AddPeer(peer p2p.Peer) {
   peer.SendEnvelope(p2p.Envelope{
       ChannelID: BlocksyncChannel,
       Message: &bcproto.StatusResponse{
           Base:   bcR.store.Base(),
           Height: bcR.store.Height(),
       },
   })
}

func (bcR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
   bcR.pool.RemovePeer(peer.ID())
}

func (bcR *Reactor) respondToPeer(msg *bcproto.BlockRequest, src p2p.Peer) bool {
   block := bcR.store.LoadBlock(msg.Height)
   if block == nil {
       bcR.Logger.Info("Peer requesting unavailable block", "peer", src, "height", msg.Height)
       return src.TrySendEnvelope(p2p.Envelope{
           ChannelID: BlocksyncChannel,
           Message:   &bcproto.NoBlockResponse{Height: msg.Height},
       })
   }

   bl, err := block.ToProto()
   if err != nil {
       bcR.Logger.Error("Failed converting block to proto", "err", err)
       return false
   }

   return src.TrySendEnvelope(p2p.Envelope{
       ChannelID: BlocksyncChannel,
       Message:   &bcproto.BlockResponse{Block: bl},
   })
}

func (bcR *Reactor) ReceiveEnvelope(e p2p.Envelope) {
   if err := ValidateMsg(e.Message); err != nil {
       bcR.Logger.Error("Invalid message", "peer", e.Src, "msg", e.Message, "err", err)
       bcR.Switch.StopPeerForError(e.Src, err)
       return
   }

   switch msg := e.Message.(type) {
   case *bcproto.BlockRequest:
       bcR.respondToPeer(msg, e.Src)

   case *bcproto.BlockResponse:
       block, err := types.BlockFromProto(msg.Block)
       if err != nil {
           bcR.Logger.Error("Invalid block", "err", err)
           return
       }
       bcR.pool.AddBlock(e.Src.ID(), block, msg.Block.Size())

   case *bcproto.StatusRequest:
       e.Src.TrySendEnvelope(p2p.Envelope{
           ChannelID: BlocksyncChannel,
           Message: &bcproto.StatusResponse{
               Height: bcR.store.Height(),
               Base:   bcR.store.Base(),
           },
       })

   case *bcproto.StatusResponse:
       bcR.pool.SetPeerRange(e.Src.ID(), msg.Base, msg.Height)

   case *bcproto.NoBlockResponse:
       bcR.Logger.Debug("Peer has no block", "peer", e.Src, "height", msg.Height)

   default:
       bcR.Logger.Error("Unknown message type", "type", reflect.TypeOf(msg))
   }
}

func (bcR *Reactor) poolRoutine(stateSynced bool) {
   trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
   defer trySyncTicker.Stop()

   statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
   defer statusUpdateTicker.Stop()

   switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)
   defer switchToConsensusTicker.Stop()

   blocksSynced := uint64(0)
   state := bcR.initialState
   chainID := state.ChainID
   
   lastHundred := time.Now()
   lastRate := 0.0

   didProcessCh := make(chan struct{}, 1)

   go bcR.handlePoolEvents(didProcessCh)

FOR_LOOP:
   for {
       select {
       case <-switchToConsensusTicker.C:
           if bcR.checkSwitchToConsensus(state, blocksSynced, stateSynced) {
               break FOR_LOOP
           }

       case <-trySyncTicker.C:
           select {
           case didProcessCh <- struct{}{}:
           default:
           }

       case <-didProcessCh:
           if err := bcR.processNextBlock(&state, chainID, &blocksSynced, &lastHundred, &lastRate); err != nil {
               bcR.Logger.Error("Error processing block", "err", err)
               continue FOR_LOOP
           }
           didProcessCh <- struct{}{}

       case <-bcR.Quit():
           break FOR_LOOP
       }
   }
}

func (bcR *Reactor) handlePoolEvents(didProcessCh chan struct{}) {
   for {
       select {
       case <-bcR.Quit():
           return
       case <-bcR.pool.Quit():
           return
       case request := <-bcR.requestsCh:
           bcR.handleBlockRequest(request)
       case err := <-bcR.errorsCh:
           bcR.handlePeerError(err)
       case <-time.After(statusUpdateIntervalSeconds * time.Second):
           go bcR.BroadcastStatusRequest()
       }
   }
}

func (bcR *Reactor) handleBlockRequest(request BlockRequest) {
   peer := bcR.Switch.Peers().Get(request.PeerID)
   if peer == nil {
       return
   }
   
   queued := peer.TrySendEnvelope(p2p.Envelope{
       ChannelID: BlocksyncChannel,
       Message:   &bcproto.BlockRequest{Height: request.Height},
   })
   
   if !queued {
       bcR.Logger.Debug("Send queue full", "peer", peer.ID(), "height", request.Height)
   }
}

func (bcR *Reactor) handlePeerError(err peerError) {
   if peer := bcR.Switch.Peers().Get(err.peerID); peer != nil {
       bcR.Switch.StopPeerForError(peer, err)
   }
}

func (bcR *Reactor) checkSwitchToConsensus(state sm.State, blocksSynced uint64, stateSynced bool) bool {
   height, numPending, lenRequesters := bcR.pool.GetStatus()
   outbound, inbound, _ := bcR.Switch.NumPeers()
   
   bcR.Logger.Debug("Consensus ticker", 
       "numPending", numPending,
       "total", lenRequesters,
       "outbound", outbound, 
       "inbound", inbound)

   if !bcR.pool.IsCaughtUp() {
       return false
   }

   bcR.Logger.Info("Switching to consensus", "height", height)
   if err := bcR.pool.Stop(); err != nil {
       bcR.Logger.Error("Error stopping pool", "err", err)
   }

   if conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor); ok {
       conR.SwitchToConsensus(state, blocksSynced > 0 || stateSynced)
   }

   return true
}

func (bcR *Reactor) processNextBlock(state *sm.State, chainID string, blocksSynced *uint64, lastHundred *time.Time, lastRate *float64) error {
   first, second := bcR.pool.PeekTwoBlocks()
   if first == nil || second == nil {
       return nil
   }

   firstParts, err := first.MakePartSet(types.BlockPartSizeBytes)
   if err != nil {
       bcR.Logger.Error("Failed making part set", "height", first.Height, "err", err)
       return err
   }

   firstID := types.BlockID{Hash: first.Hash(), PartSetHeader: firstParts.Header()}

   if err := bcR.validateBlock(state, chainID, first, second, firstID); err != nil {
       bcR.handleValidationError(err, first, second)
       return err
   }

   bcR.pool.PopRequest()
   bcR.store.SaveBlock(first, firstParts, second.LastCommit)

   var newState sm.State
   newState, _, err = bcR.blockExec.ApplyBlock(*state, firstID, first)
   if err != nil {
       panic(fmt.Sprintf("Failed processing block %d:%X: %v", first.Height, first.Hash(), err))
   }
   *state = newState
   *blocksSynced++

   if *blocksSynced%100 == 0 {
       *lastRate = 0.9 * *lastRate + 0.1*(100/time.Since(*lastHundred).Seconds())
       bcR.Logger.Info("Sync rate", 
           "height", bcR.pool.height,
           "peer_height", bcR.pool.MaxPeerHeight(),
           "blocks/s", *lastRate)
       *lastHundred = time.Now()
   }

   return nil
}

func (bcR *Reactor) validateBlock(state *sm.State, chainID string, first *types.Block, second *types.Block, firstID types.BlockID) error {
   err := state.Validators.VerifyCommitLight(chainID, firstID, first.Height, second.LastCommit)
   if err != nil {
       return fmt.Errorf("commit verification failed: %w", err)
   }

   if err := bcR.blockExec.ValidateBlock(*state, first); err != nil {
       return fmt.Errorf("block validation failed: %w", err)
   }

   return nil
}

func (bcR *Reactor) handleValidationError(err error, first *types.Block, second *types.Block) {
   bcR.Logger.Error("Validation error", "err", err)

   for _, height := range []int64{first.Height, second.Height} {
       if peerID := bcR.pool.RedoRequest(height); peerID != "" {
           if peer := bcR.Switch.Peers().Get(peerID); peer != nil {
               bcR.Switch.StopPeerForError(peer, fmt.Errorf("validation error: %v", err))
           }
       }
   }
}

func (bcR *Reactor) BroadcastStatusRequest() {
   bcR.Switch.BroadcastEnvelope(p2p.Envelope{
       ChannelID: BlocksyncChannel,
       Message:   &bcproto.StatusRequest{},
   })
}
