package consensus

import (
    "context"
    "fmt"
    "os"
    "path"
    "path/filepath"
    "sort"
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    dbm "github.com/baron-chain/cometbft-bc-db"

    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    abci "github.com/baron-chain/cometbft-bc/abci/types" 
    cfg "github.com/baron-chain/cometbft-bc/config"
    "github.com/baron-chain/cometbft-bc/crypto"
    bcbytes "github.com/baron-chain/cometbft-bc/libs/bytes"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcos "github.com/baron-chain/cometbft-bc/libs/os"
    bcpubsub "github.com/baron-chain/cometbft-bc/libs/pubsub"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
    mempl "github.com/baron-chain/cometbft-bc/mempool"
    "github.com/baron-chain/cometbft-bc/p2p"
    bcproto "github.com/baron-chain/cometbft-bc/proto/types"
    sm "github.com/baron-chain/cometbft-bc/state"
    "github.com/baron-chain/cometbft-bc/store"
    "github.com/baron-chain/cometbft-bc/types"
    bctime "github.com/baron-chain/cometbft-bc/types/time"
)

const (
    testSubscriber = "test_client"
    ensureTimeout = 200 * time.Millisecond 
    testMinPower = 10
)

// Test setup utilities

type cleanupFunc func()

type validatorStub struct {
    Index         int32
    Height        int64  
    Round         int32
    VotingPower   int64
    PrivValidator types.PrivValidator
    lastVote     *types.Vote
}

func newValidatorStub(privVal types.PrivValidator, valIndex int32) *validatorStub {
    return &validatorStub{
        Index:         valIndex,
        PrivValidator: privVal, 
        VotingPower:   testMinPower,
    }
}

// Test State creation

func newTestState(config *cfg.Config, state sm.State, privVal types.PrivValidator, app abci.Application) *State {
    // Create block store
    blockDB := dbm.NewMemDB()
    blockStore := store.NewBlockStore(blockDB)

    // Create proxy app connections
    proxyApp := abcicli.NewLocalClient(new(bcsync.Mutex), app)
    proxyMempool := abcicli.NewLocalClient(new(bcsync.Mutex), app) 

    // Initialize mempool
    mempool := mempl.NewCListMempool(
        config.Mempool,
        proxyMempool,
        state.LastBlockHeight,
        mempl.WithPreCheck(sm.TxPreCheck(state)),
        mempl.WithPostCheck(sm.TxPostCheck(state)),
    )

    if config.Consensus.WaitForTxs() {
        mempool.EnableTxsAvailable()
    }

    // Create and initialize consensus state 
    stateDB := dbm.NewMemDB()
    stateStore := sm.NewStore(stateDB, sm.StoreOptions{
        DiscardABCIResponses: false,
    })
    if err := stateStore.Save(state); err != nil {
        panic(err) 
    }

    blockExec := sm.NewBlockExecutor(
        stateStore,
        log.TestingLogger(),
        proxyApp,
        mempool,
        sm.EmptyEvidencePool{},
    )

    cs := NewState(
        config.Consensus,
        state, 
        blockExec,
        blockStore,
        mempool,
        sm.EmptyEvidencePool{},
    )

    cs.SetLogger(log.TestingLogger().With("module", "consensus"))
    cs.SetPrivValidator(privVal)

    eventBus := types.NewEventBus()
    eventBus.SetLogger(log.TestingLogger().With("module", "events"))
    if err := eventBus.Start(); err != nil {
        panic(err)
    }
    cs.SetEventBus(eventBus)

    return cs
}

// Create test network of consensus states
func newTestConsensusNet(t *testing.T, nValidators int, testName string) ([]*State, cleanupFunc) {
    t.Helper()
    
    // Generate test genesis state
    genDoc, privVals := randGenesisState(nValidators, false, testMinPower)
    states := make([]*State, nValidators)
    rootDirs := make([]string, nValidators)

    // Create consensus state for each validator
    for i := 0; i < nValidators; i++ {
        config := cfg.ResetTestRoot(fmt.Sprintf("%s_%d", testName, i))
        rootDirs[i] = config.RootDir

        stateDB := dbm.NewMemDB()
        stateStore := sm.NewStore(stateDB, sm.StoreOptions{
            DiscardABCIResponses: false,
        })

        state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
        require.NoError(t, err)

        app := newKVStoreApp()
        states[i] = newTestState(config, state, privVals[i], app)
    }

    cleanup := func() {
        for _, dir := range rootDirs {
            os.RemoveAll(dir)
        }
    }

    return states, cleanup
}

// Helper functions for event verification

func ensureNewEvent(ch <-chan bcpubsub.Message, height int64, round int32, timeout time.Duration) error {
    timer := time.NewTimer(timeout)
    defer timer.Stop()

    select {
    case <-timer.C:
        return fmt.Errorf("timeout waiting for event")
    case msg := <-ch:
        event, ok := msg.Data().(types.EventDataRoundState)
        if !ok {
            return fmt.Errorf("expected EventDataRoundState, got %T", msg.Data())
        }
        if event.Height != height {
            return fmt.Errorf("expected height %v, got %v", height, event.Height)
        }
        if event.Round != round {
            return fmt.Errorf("expected round %v, got %v", round, event.Round)
        }
        return nil
    }
}

// Additional test utilities abbreviated for length...
