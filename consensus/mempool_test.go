package consensus

import (
    "encoding/binary"
    "fmt"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    dbm "github.com/baron-chain/cometbft-bc-db"

    "github.com/baron-chain/cometbft-bc/abci/example/code"
    abci "github.com/baron-chain/cometbft-bc/abci/types"
    mempl "github.com/baron-chain/cometbft-bc/mempool"
    sm "github.com/baron-chain/cometbft-bc/state"
    "github.com/baron-chain/cometbft-bc/types"
)

const (
    defaultTestTimeout = 30 * time.Second
    numConcurrentTxs  = 3000
)

// CounterApplication implements a test application that tracks transaction counts
type CounterApplication struct {
    abci.BaseApplication
    txCount        int
    mempoolTxCount int
}

func NewCounterApplication() *CounterApplication {
    return &CounterApplication{}
}

func (app *CounterApplication) Info(req abci.RequestInfo) abci.ResponseInfo {
    return abci.ResponseInfo{Data: fmt.Sprintf("txs:%v", app.txCount)}
}

func (app *CounterApplication) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
    txValue := decodeTx(req.Tx)
    if txValue != uint64(app.txCount) {
        return abci.ResponseDeliverTx{
            Code: code.CodeTypeBadNonce,
            Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue),
        }
    }
    app.txCount++
    return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
    txValue := decodeTx(req.Tx)
    if txValue != uint64(app.mempoolTxCount) {
        return abci.ResponseCheckTx{
            Code: code.CodeTypeBadNonce,
            Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.mempoolTxCount, txValue),
        }
    }
    app.mempoolTxCount++
    return abci.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) Commit() abci.ResponseCommit {
    app.mempoolTxCount = app.txCount
    if app.txCount == 0 {
        return abci.ResponseCommit{}
    }
    return abci.ResponseCommit{Data: encodeTx(uint64(app.txCount))}
}

// Mempool test suite
type mempoolTestSuite struct {
    t          *testing.T
    config     *cfg.Config
    state      sm.State
    privVal    types.PrivValidator
    app        *CounterApplication
    cs         *State
    blockStore dbm.DB
}

func newMempoolTestSuite(t *testing.T) *mempoolTestSuite {
    config := ResetConfig("consensus_mempool_test")
    state, privVals := randGenesisState(1, false, 10)
    blockStore := dbm.NewMemDB()
    app := NewCounterApplication()
    
    cs := newStateWithConfigAndBlockStore(
        config,
        state, 
        privVals[0],
        app,
        blockStore,
    )

    return &mempoolTestSuite{
        t:          t,
        config:     config,
        state:      state,
        privVal:    privVals[0],
        app:        app,
        cs:         cs,
        blockStore: blockStore,
    }
}

func (suite *mempoolTestSuite) cleanup() {
    os.RemoveAll(suite.config.RootDir)
}

// Test Cases

func TestMempoolNoProgressUntilTxsAvailable(t *testing.T) {
    suite := newMempoolTestSuite(t)
    defer suite.cleanup()

    suite.config.Consensus.CreateEmptyBlocks = false
    mp := assertMempool(suite.cs.txNotifier)
    mp.EnableTxsAvailable()

    newBlockCh := subscribe(suite.cs.eventBus, types.EventQueryNewBlock)
    startTestRound(suite.cs, suite.cs.Height, suite.cs.Round)

    // Should create first block
    ensureNewEventOnChannel(newBlockCh)
    ensureNoNewEventOnChannel(newBlockCh)

    // Deliver tx and expect new blocks
    deliverTxsRange(suite.cs, 0, 1)
    ensureNewEventOnChannel(newBlockCh) // commit tx
    ensureNewEventOnChannel(newBlockCh) // commit updated app hash
    ensureNoNewEventOnChannel(newBlockCh)
}

func TestMempoolTxConcurrent(t *testing.T) {
    suite := newMempoolTestSuite(t)
    defer suite.cleanup()

    stateStore := sm.NewStore(suite.blockStore, sm.StoreOptions{
        DiscardABCIResponses: false,
    })
    require.NoError(t, stateStore.Save(suite.state))

    newBlockHeaderCh := subscribe(suite.cs.eventBus, types.EventQueryNewBlockHeader)

    // Start concurrent tx delivery
    go deliverTxsRange(suite.cs, 0, int(numConcurrentTxs))

    startTestRound(suite.cs, suite.cs.Height, suite.cs.Round)
    
    // Wait for all txs to be committed
    committedTxs := int64(0)
    timer := time.NewTimer(defaultTestTimeout)
    defer timer.Stop()

    for committedTxs < numConcurrentTxs {
        select {
        case msg := <-newBlockHeaderCh:
            headerEvent := msg.Data().(types.EventDataNewBlockHeader)
            committedTxs += headerEvent.NumTxs
        case <-timer.C:
            t.Fatal("Timed out waiting for tx commits")
        }
    }
}

// Helper Functions

func assertMempool(txn txNotifier) mempl.Mempool {
    return txn.(mempl.Mempool)
}

func deliverTxsRange(cs *State, start, end int) {
    for i := start; i < end; i++ {
        tx := encodeTx(uint64(i))
        err := assertMempool(cs.txNotifier).CheckTx(tx, nil, mempl.TxInfo{})
        if err != nil {
            panic(fmt.Sprintf("CheckTx failed: %v", err))
        }
    }
}

func encodeTx(value uint64) []byte {
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, value)
    return buf
}

func decodeTx(tx []byte) uint64 {
    buf := make([]byte, 8)
    copy(buf[len(buf)-len(tx):], tx)
    return binary.BigEndian.Uint64(buf)
}
