package blocksync

import (
   "fmt"
   "testing"
   "time"

   "github.com/stretchr/testify/assert"
   "github.com/stretchr/testify/require"

   "github.com/baron-chain/cometbft-bc/libs/log"
   cmtrand "github.com/baron-chain/cometbft-bc/libs/rand"
   "github.com/baron-chain/cometbft-bc/p2p"
   "github.com/baron-chain/cometbft-bc/types"
)

func init() {
   peerTimeout = 2 * time.Second
}

type testPeer struct {
   id        p2p.ID
   base      int64
   height    int64
   inputChan chan inputData
}

type inputData struct {
   t       *testing.T
   pool    *BlockPool
   request BlockRequest
}

func (p testPeer) runInputRoutine() {
   go func() {
       for input := range p.inputChan {
           block := &types.Block{Header: types.Header{Height: input.request.Height}}
           input.pool.AddBlock(input.request.PeerID, block, 123)
       }
   }()
}

type testPeers map[p2p.ID]testPeer

func (ps testPeers) start() {
   for _, p := range ps {
       p.runInputRoutine()
   }
}

func (ps testPeers) stop() {
   for _, p := range ps {
       close(p.inputChan)
   }
}

func makePeers(numPeers int, minHeight, maxHeight int64) testPeers {
   peers := make(testPeers, numPeers)
   for i := 0; i < numPeers; i++ {
       peerID := p2p.ID(cmtrand.Str(12))
       height := minHeight + cmtrand.Int63n(maxHeight-minHeight)
       base := minHeight + int64(i)
       if base > height {
           base = height
       }
       peers[peerID] = testPeer{
           id:        peerID,
           base:      base,
           height:    height,
           inputChan: make(chan inputData, 10),
       }
   }
   return peers
}

func TestBlockPoolBasic(t *testing.T) {
   const startHeight = 42
   peers := makePeers(10, startHeight+1, 1000)
   errorsCh := make(chan peerError, 1000)
   requestsCh := make(chan BlockRequest, 1000)
   
   pool := NewBlockPool(startHeight, requestsCh, errorsCh)
   pool.SetLogger(log.TestingLogger())
   require.NoError(t, pool.Start())
   
   t.Cleanup(func() {
       require.NoError(t, pool.Stop())
   })

   peers.start()
   defer peers.stop()

   go func() {
       for _, peer := range peers {
           pool.SetPeerRange(peer.id, peer.base, peer.height)
       }
   }()

   go func() {
       for pool.IsRunning() {
           if first, second := pool.PeekTwoBlocks(); first != nil && second != nil {
               pool.PopRequest()
           } else {
               time.Sleep(time.Second)
           }
       }
   }()

   for {
       select {
       case err := <-errorsCh:
           t.Error(err)
       case request := <-requestsCh:
           if request.Height == 300 {
               return
           }
           peers[request.PeerID].inputChan <- inputData{t, pool, request}
       }
   }
}

func TestBlockPoolTimeout(t *testing.T) {
   const startHeight = 42
   peers := makePeers(10, startHeight+1, 1000)
   errorsCh := make(chan peerError, 1000)
   requestsCh := make(chan BlockRequest, 1000)
   
   pool := NewBlockPool(startHeight, requestsCh, errorsCh)
   pool.SetLogger(log.TestingLogger())
   require.NoError(t, pool.Start())
   
   t.Cleanup(func() {
       require.NoError(t, pool.Stop())
   })

   go func() {
       for _, peer := range peers {
           pool.SetPeerRange(peer.id, peer.base, peer.height)
       }
   }()

   go func() {
       for pool.IsRunning() {
           if first, second := pool.PeekTwoBlocks(); first != nil && second != nil {
               pool.PopRequest()
           } else {
               time.Sleep(time.Second)
           }
       }
   }()

   timedOutPeers := make(map[p2p.ID]struct{})
   for timeouts := 0; timeouts < len(peers); {
       select {
       case err := <-errorsCh:
           if _, ok := timedOutPeers[err.peerID]; !ok {
               timedOutPeers[err.peerID] = struct{}{}
               timeouts++
           }
       case request := <-requestsCh:
           t.Logf("Received request: %+v", request)
       }
   }
}

func TestBlockPoolRemovePeer(t *testing.T) {
   peers := make(testPeers, 10)
   for i := 0; i < 10; i++ {
       peerID := p2p.ID(fmt.Sprintf("%d", i+1))
       peers[peerID] = testPeer{
           id:        peerID,
           base:      0,
           height:    int64(i + 1),
           inputChan: make(chan inputData),
       }
   }

   pool := NewBlockPool(1, make(chan BlockRequest), make(chan peerError))
   pool.SetLogger(log.TestingLogger())
   require.NoError(t, pool.Start())
   
   t.Cleanup(func() {
       require.NoError(t, pool.Stop())
   })

   for id, peer := range peers {
       pool.SetPeerRange(id, peer.base, peer.height)
   }
   assert.Equal(t, int64(10), pool.MaxPeerHeight())

   assert.NotPanics(t, func() { pool.RemovePeer("non-existing-peer") })

   pool.RemovePeer("10")
   assert.Equal(t, int64(9), pool.MaxPeerHeight())

   for id := range peers {
       pool.RemovePeer(id)
   }
   assert.Equal(t, int64(0), pool.MaxPeerHeight())
}
