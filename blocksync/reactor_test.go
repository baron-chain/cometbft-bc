package blocksync

import (
   "fmt"
   "os"
   "sort"
   "testing"
   "time"

   "github.com/stretchr/testify/assert" 
   "github.com/stretchr/testify/mock"
   "github.com/stretchr/testify/require"

   dbm "github.com/baron-chain/cometbft-bc-db"
   abci "github.com/baron-chain/cometbft-bc/abci/types"
   cfg "github.com/baron-chain/cometbft-bc/config"
   "github.com/baron-chain/cometbft-bc/libs/log"
   mpmocks "github.com/baron-chain/cometbft-bc/mempool/mocks"
   "github.com/baron-chain/cometbft-bc/p2p"
   "github.com/baron-chain/cometbft-bc/proxy"
   sm "github.com/baron-chain/cometbft-bc/state"
   "github.com/baron-chain/cometbft-bc/store" 
   "github.com/baron-chain/cometbft-bc/types"
   cmttime "github.com/baron-chain/cometbft-bc/types/time"
)

var config *cfg.Config

func randGenesisDoc(numValidators int, randPower bool, minPower int64) (*types.GenesisDoc, []types.PrivValidator) {
   validators := make([]types.GenesisValidator, numValidators)
   privValidators := make([]types.PrivValidator, numValidators)
   
   for i := 0; i < numValidators; i++ {
       val, privVal := types.RandValidator(randPower, minPower)
       validators[i] = types.GenesisValidator{
           PubKey: val.PubKey,
           Power:  val.VotingPower,
       }
       privValidators[i] = privVal
   }
   
   sort.Sort(types.PrivValidatorsByAddress(privValidators))

   return &types.GenesisDoc{
       GenesisTime: cmttime.Now(),
       ChainID:     config.ChainID(),
       Validators:  validators,
   }, privValidators
}

type ReactorPair struct {
   reactor *Reactor
   app     proxy.AppConns
}

func newReactor(t *testing.T, logger log.Logger, genDoc *types.GenesisDoc, privVals []types.PrivValidator, maxBlockHeight int64) ReactorPair {
   require.Equal(t, 1, len(privVals), "only single validator supported")

   app := &testApp{}
   cc := proxy.NewLocalClientCreator(app)
   proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
   require.NoError(t, proxyApp.Start())

   blockDB := dbm.NewMemDB()
   stateDB := dbm.NewMemDB()
   stateStore := sm.NewStore(stateDB, sm.StoreOptions{DiscardABCIResponses: false})
   blockStore := store.NewBlockStore(blockDB)

   state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
   require.NoError(t, err)

   mp := &mpmocks.Mempool{}
   mp.On("Lock").Return()
   mp.On("Unlock").Return()
   mp.On("FlushAppConn", mock.Anything).Return(nil)
   mp.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

   db := dbm.NewMemDB()
   stateStore = sm.NewStore(db, sm.StoreOptions{DiscardABCIResponses: false})
   blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(), mp, sm.EmptyEvidencePool{})
   require.NoError(t, stateStore.Save(state))

   for height := int64(1); height <= maxBlockHeight; height++ {
       lastCommit := types.NewCommit(height-1, 0, types.BlockID{}, nil)
       if height > 1 {
           lastMeta := blockStore.LoadBlockMeta(height - 1)
           lastBlock := blockStore.LoadBlock(height - 1)
           
           vote, err := types.MakeVote(
               lastBlock.Header.Height,
               lastMeta.BlockID,
               state.Validators, 
               privVals[0],
               lastBlock.Header.ChainID,
               time.Now(),
           )
           require.NoError(t, err)
           
           lastCommit = types.NewCommit(
               vote.Height,
               vote.Round,
               lastMeta.BlockID,
               []types.CommitSig{vote.CommitSig()},
           )
       }

       block := state.MakeBlock(height, nil, lastCommit, nil, state.Validators.Proposer.Address)
       parts, err := block.MakePartSet(types.BlockPartSizeBytes)
       require.NoError(t, err)
       
       blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
       state, _, err = blockExec.ApplyBlock(state, blockID, block)
       require.NoError(t, err)

       blockStore.SaveBlock(block, parts, lastCommit)
   }

   reactor := NewReactor(state.Copy(), blockExec, blockStore, true)
   reactor.SetLogger(logger.With("module", "blockchain"))

   return ReactorPair{reactor, proxyApp}
}

func TestNoBlockResponse(t *testing.T) {
   config = cfg.ResetTestRoot("blockchain_reactor_test")
   defer os.RemoveAll(config.RootDir)

   genDoc, privVals := randGenesisDoc(1, false, 30)
   pairs := []ReactorPair{
       newReactor(t, log.TestingLogger(), genDoc, privVals, 65),
       newReactor(t, log.TestingLogger(), genDoc, privVals, 0),
   }

   switches := p2p.MakeConnectedSwitches(config.P2P, 2, func(i int, s *p2p.Switch) *p2p.Switch {
       s.AddReactor("BLOCKCHAIN", pairs[i].reactor)
       return s
   }, p2p.Connect2Switches)

   defer func() {
       for _, pair := range pairs {
           require.NoError(t, pair.reactor.Stop())
           require.NoError(t, pair.app.Stop())
       }
   }()

   for !pairs[1].reactor.pool.IsCaughtUp() {
       time.Sleep(10 * time.Millisecond) 
   }

   assert.Equal(t, int64(65), pairs[0].reactor.store.Height())

   tests := []struct {
       height int64
       exists bool 
   }{
       {67, false},
       {10, true},
       {1, true},
       {100, false},
   }

   for _, tt := range tests {
       block := pairs[1].reactor.store.LoadBlock(tt.height)
       assert.Equal(t, tt.exists, block != nil)
   }
}

func TestBadBlockStopsPeer(t *testing.T) {
   config = cfg.ResetTestRoot("blockchain_reactor_test")
   defer os.RemoveAll(config.RootDir)

   genDoc, privVals := randGenesisDoc(1, false, 30)
   otherGenDoc, otherPrivVals := randGenesisDoc(1, false, 30)
   
   const maxHeight = 148
   otherChain := newReactor(t, log.TestingLogger(), otherGenDoc, otherPrivVals, maxHeight)
   defer func() {
       require.Error(t, otherChain.reactor.Stop())
       require.NoError(t, otherChain.app.Stop())
   }()

   pairs := make([]ReactorPair, 4)
   pairs[0] = newReactor(t, log.TestingLogger(), genDoc, privVals, maxHeight)
   for i := 1; i < 4; i++ {
       pairs[i] = newReactor(t, log.TestingLogger(), genDoc, privVals, 0)
   }

   switches := p2p.MakeConnectedSwitches(config.P2P, 4, func(i int, s *p2p.Switch) *p2p.Switch {
       s.AddReactor("BLOCKCHAIN", pairs[i].reactor)
       return s
   }, p2p.Connect2Switches)

   defer func() {
       for _, pair := range pairs {
           require.NoError(t, pair.reactor.Stop())
           require.NoError(t, pair.app.Stop())
       }
   }()

   for {
       if allCaughtUp := true; allCaughtUp {
           for _, pair := range pairs {
               if !pair.reactor.pool.IsCaughtUp() {
                   allCaughtUp = false
                   break
               }
           }
           if allCaughtUp {
               break
           }
       }
       time.Sleep(time.Second)
   }

   assert.Equal(t, 3, pairs[1].reactor.Switch.Peers().Size())

   pairs[3].reactor.store = otherChain.reactor.store

   lastPair := newReactor(t, log.TestingLogger(), genDoc, privVals, 0)
   pairs = append(pairs, lastPair)

   switches = append(switches, p2p.MakeConnectedSwitches(config.P2P, 1, func(i int, s *p2p.Switch) *p2p.Switch {
       s.AddReactor("BLOCKCHAIN", pairs[len(pairs)-1].reactor)
       return s
   }, p2p.Connect2Switches)...)

   for i := 0; i < len(pairs)-1; i++ {
       p2p.Connect2Switches(switches, i, len(pairs)-1)
   }

   for {
       if lastPair.reactor.pool.IsCaughtUp() || lastPair.reactor.Switch.Peers().Size() == 0 {
           break
       }
       time.Sleep(time.Second)
   }

   assert.True(t, lastPair.reactor.Switch.Peers().Size() < len(pairs)-1)
}

type testApp struct {
   abci.BaseApplication
}

func (app *testApp) Info(abci.RequestInfo) abci.ResponseInfo           { return abci.ResponseInfo{} }
func (app *testApp) BeginBlock(abci.RequestBeginBlock) abci.ResponseBeginBlock { return abci.ResponseBeginBlock{} }
func (app *testApp) EndBlock(abci.RequestEndBlock) abci.ResponseEndBlock       { return abci.ResponseEndBlock{} }
func (app *testApp) DeliverTx(abci.RequestDeliverTx) abci.ResponseDeliverTx   { return abci.ResponseDeliverTx{} }
func (app *testApp) CheckTx(abci.RequestCheckTx) abci.ResponseCheckTx         { return abci.ResponseCheckTx{} }
func (app *testApp) Commit() abci.ResponseCommit                             { return abci.ResponseCommit{} }
func (app *testApp) Query(abci.RequestQuery) abci.ResponseQuery             { return abci.ResponseQuery{} }
