package consensus

import (
    "context"
    "fmt"
    "os"
    "path"
    "sync"
    "testing"
    "time"
    
    "github.com/stretchr/testify/require"
    dbm "github.com/baron-chain/cometbft-bc-db"
    
    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    abci "github.com/baron-chain/cometbft-bc/abci/types"
    bcevidence "github.com/baron-chain/cometbft-bc/evidence"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
    mempl "github.com/baron-chain/cometbft-bc/mempool"
    cfg "github.com/baron-chain/cometbft-bc/config"
    mempoolv0 "github.com/baron-chain/cometbft-bc/mempool/v0"
    "github.com/baron-chain/cometbft-bc/p2p"
    bccons "github.com/baron-chain/cometbft-bc/proto/consensus"
    bcproto "github.com/baron-chain/cometbft-bc/proto/types"
    sm "github.com/baron-chain/cometbft-bc/state"
    "github.com/baron-chain/cometbft-bc/store"
    "github.com/baron-chain/cometbft-bc/types"
)

const (
    testByzantinePrecommitHeight = int64(2)
    defaultNumValidators = 4
    maxTestDuration = 20 * time.Second
)

type byzantineTestSetup struct {
    nValidators     int
    byzantineNode   int
    css            []*State
    reactors       []*Reactor 
    blocksSubs     []types.Subscription
    eventBuses     []*types.EventBus
    switches       []*p2p.Switch
}

func newByzantineTest(t *testing.T, nValidators int) *byzantineTestSetup {
    t.Helper()
    
    testName := "byzantine_test"
    logger := log.TestingLogger().With("test", "byzantine")
    
    // Create test validator set
    genDoc, privVals := randGenesisDoc(nValidators, false, 30)
    css := make([]*State, nValidators)
    
    // Initialize states for each validator
    for i := 0; i < nValidators; i++ {
        css[i] = createTestState(t, testName, i, genDoc, privVals[i], logger)
    }
    
    setup := &byzantineTestSetup{
        nValidators: nValidators,
        byzantineNode: 0,
        css: css,
    }
    
    setup.initNetworking(t)
    
    return setup
}

func createTestState(t *testing.T, testName string, index int, genDoc *types.GenesisDoc, privVal types.PrivValidator, logger log.Logger) *State {
    config := cfg.ResetTestRoot(fmt.Sprintf("%s_%d", testName, index))
    defer os.RemoveAll(config.RootDir)
    
    // Create state DB and store
    stateDB := dbm.NewMemDB()
    stateStore := sm.NewStore(stateDB, sm.StoreOptions{
        DiscardABCIResponses: false,
    })
    
    state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
    require.NoError(t, err)
    
    // Create app connection
    app := newKVStore()
    proxyApp := abcicli.NewLocalClient(new(bcsync.Mutex), app)
    
    // Initialize app with validators
    app.InitChain(abci.RequestInitChain{
        Validators: types.TM2PB.ValidatorUpdates(state.Validators),
    })
    
    // Create block store
    blockStore := store.NewBlockStore(dbm.NewMemDB())
    
    // Create mempool
    mempool := mempoolv0.NewCListMempool(
        config.Mempool,
        proxyApp,
        state.LastBlockHeight,
        mempoolv0.WithPreCheck(sm.TxPreCheck(state)),
        mempoolv0.WithPostCheck(sm.TxPostCheck(state)),
    )
    
    if config.Consensus.WaitForTxs() {
        mempool.EnableTxsAvailable()
    }
    
    // Create evidence pool
    evidencePool, err := bcevidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
    require.NoError(t, err)
    
    // Create block executor and state
    blockExec := sm.NewBlockExecutor(stateStore, logger, proxyApp, mempool, evidencePool)
    cs := NewState(config.Consensus, state, blockExec, blockStore, mempool, evidencePool)
    
    cs.SetLogger(logger.With("validator", index))
    cs.SetPrivValidator(privVal)
    
    eventBus := types.NewEventBus()
    eventBus.SetLogger(logger.With("module", "events"))
    err = eventBus.Start()
    require.NoError(t, err)
    
    cs.SetEventBus(eventBus)
    cs.SetTimeoutTicker(newMockTickerFunc(true)())
    
    return cs
}

func (bt *byzantineTestSetup) setupByzantine() {
    bcs := bt.css[bt.byzantineNode]
    
    bcs.doPrevote = func(height int64, round int32) {
        if height == testByzantinePrecommitHeight {
            bt.sendByzantinePrevotes(bcs)
        } else {
            bcs.defaultDoPrevote(height, round)
        }
    }
}

func (bt *byzantineTestSetup) sendByzantinePrevotes(bcs *State) {
    prevote1, err := bcs.signVote(bcproto.PrevoteType, bcs.ProposalBlock.Hash(), bcs.ProposalBlockParts.Header())
    if err != nil {
        panic(err)
    }
    
    prevote2, err := bcs.signVote(bcproto.PrevoteType, nil, types.PartSetHeader{})
    if err != nil {
        panic(err)
    }
    
    // Split votes between peers
    peers := bt.reactors[bt.byzantineNode].Switch.Peers().List()
    midIdx := len(peers) / 2
    
    for i, peer := range peers {
        vote := prevote1
        if i >= midIdx {
            vote = prevote2
        }
        peer.SendEnvelope(p2p.Envelope{
            Message:   &bccons.Vote{Vote: vote.ToProto()},
            ChannelID: VoteChannel,
        })
    }
}

func (bt *byzantineTestSetup) cleanup() {
    for _, cs := range bt.css {
        if err := cs.Stop(); err != nil {
            panic(err)
        }
    }
}

func TestByzantinePrevoteEquivocation(t *testing.T) {
    bt := newByzantineTest(t, defaultNumValidators)
    defer bt.cleanup()
    
    bt.setupByzantine()
    
    // Start consensus
    for i := 0; i < bt.nValidators; i++ {
        bt.reactors[i].SwitchToConsensus(bt.css[i].GetState(), false)
    }
    
    // Wait for and verify evidence
    evidenceFromEachValidator := make([]types.Evidence, bt.nValidators)
    
    var wg sync.WaitGroup
    wg.Add(bt.nValidators)
    
    for i := 0; i < bt.nValidators; i++ {
        go func(idx int) {
            defer wg.Done()
            for msg := range bt.blocksSubs[idx].Out() {
                if block := msg.Data().(types.EventDataNewBlock).Block; len(block.Evidence.Evidence) > 0 {
                    evidenceFromEachValidator[idx] = block.Evidence.Evidence[0]
                    return
                }
            }
        }(i)
    }
    
    // Wait for evidence or timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        verifyByzantineEvidence(t, bt, evidenceFromEachValidator)
    case <-time.After(maxTestDuration):
        t.Fatal("Timed out waiting for validators to commit evidence")
    }
}

func verifyByzantineEvidence(t *testing.T, bt *byzantineTestSetup, evidence []types.Evidence) {
    pubkey, err := bt.css[bt.byzantineNode].privValidator.GetPubKey()
    require.NoError(t, err)
    
    for idx, ev := range evidence {
        if ev == nil {
            t.Errorf("Missing evidence from validator %d", idx)
            continue
        }
        
        devidence, ok := ev.(*types.DuplicateVoteEvidence) 
        require.True(t, ok, "Wrong evidence type from validator %d", idx)
        
        require.Equal(t, pubkey.Address(), devidence.VoteA.ValidatorAddress)
        require.Equal(t, testByzantinePrecommitHeight, devidence.Height())
    }
}
