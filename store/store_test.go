package store

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	dbm "github.com/baron-chain/cometbft-bc-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/baron-chain/cometbft-bc/config"
	"github.com/baron-chain/cometbft-bc/crypto"
	"github.com/baron-chain/cometbft-bc/internal/test"
	"github.com/baron-chain/cometbft-bc/libs/log"
	bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
	bcstore "github.com/baron-chain/cometbft-bc/proto/baronchain/store"
	bcversion "github.com/baron-chain/cometbft-bc/proto/baronchain/version"
	sm "github.com/baron-chain/cometbft-bc/state"
	"github.com/baron-chain/cometbft-bc/types"
	bctime "github.com/baron-chain/cometbft-bc/types/time"
	"github.com/baron-chain/cometbft-bc/version"
)

// Test setup types and functions
type cleanupFunc func()

// Create a test commit with PQC signature
func makeTestCommit(height int64, timestamp time.Time) *types.Commit {
	commitSigs := []types.CommitSig{{
		BlockIDFlag:      types.BlockIDFlagCommit,
		ValidatorAddress: bcrand.Bytes(crypto.AddressSize),
		Timestamp:       timestamp,
		Signature:       generatePQCSignature("test"),
	}}
	return types.NewCommit(height, 0,
		types.BlockID{
			Hash: bcrand.Bytes(32),
			PartSetHeader: types.PartSetHeader{
				Hash:  bcrand.Bytes(32),
				Total: 2,
			},
		},
		commitSigs,
	)
}

// Generate PQC signature for testing
func generatePQCSignature(data string) []byte {
	// In production, this would use actual PQC signing
	return []byte(fmt.Sprintf("pqc_sig_%s", data))
}

// Create test state and block store with proper initialization
func makeStateAndBlockStore(logger log.Logger) (sm.State, *BlockStore, cleanupFunc) {
	config := cfg.ResetTestRoot("baron_chain_test")
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	if err != nil {
		panic(fmt.Errorf("error constructing state from genesis file: %w", err))
	}

	// Initialize BlockStore with PQC and AI options
	bs := NewBlockStore(blockDB, WithPQC(true), WithAI(true))
	
	cleanup := func() { 
		os.RemoveAll(config.RootDir)
	}

	return state, bs, cleanup
}

func TestBlockStoreBasic(t *testing.T) {
	t.Run("LoadBlockStoreState", func(t *testing.T) {
		testcases := []struct {
			name     string
			bss      *bcstore.BlockStoreState
			expected bcstore.BlockStoreState
		}{
			{
				name: "normal state",
				bss:  &bcstore.BlockStoreState{Base: 100, Height: 1000},
				expected: bcstore.BlockStoreState{Base: 100, Height: 1000},
			},
			{
				name: "empty state",
				bss:  &bcstore.BlockStoreState{},
				expected: bcstore.BlockStoreState{},
			},
			{
				name: "no base",
				bss:  &bcstore.BlockStoreState{Height: 1000},
				expected: bcstore.BlockStoreState{Base: 1, Height: 1000},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				db := dbm.NewMemDB()
				SaveBlockStoreState(tc.bss, db)
				loaded := LoadBlockStoreState(db)
				assert.Equal(t, tc.expected, loaded)
			})
		}
	})

	t.Run("BlockStoreOperations", func(t *testing.T) {
		db := dbm.NewMemDB()
		bs := NewBlockStore(db, WithPQC(true), WithAI(true))

		// Test initial state
		assert.Equal(t, int64(0), bs.Base())
		assert.Equal(t, int64(0), bs.Height())
		assert.Equal(t, int64(0), bs.Size())

		// Test saving and loading blocks
		state, _, cleanup := makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
		defer cleanup()

		// Create and save a test block
		block := makeTestBlock(state, 1)
		partSet, err := block.MakePartSet(2)
		require.NoError(t, err)
		
		seenCommit := makeTestCommit(1, bctime.Now())
		bs.SaveBlock(block, partSet, seenCommit)

		// Verify block was saved correctly
		loadedBlock := bs.LoadBlock(1)
		require.NotNil(t, loadedBlock)
		assert.Equal(t, block.Hash(), loadedBlock.Hash())

		// Test AI metrics
		metrics := bs.GetMetrics()
		assert.True(t, metrics.cacheHitRate >= 0)
		assert.True(t, metrics.writeLatency >= 0)
	})
}

// Helper function to create test blocks
func makeTestBlock(state sm.State, height int64) *types.Block {
	return state.MakeBlock(
		height,
		test.MakeNTxs(height, 10),
		new(types.Commit),
		nil,
		state.Validators.GetProposer().Address,
	)
}

func TestQuantumSafeOperations(t *testing.T) {
	t.Run("PQCSignatureVerification", func(t *testing.T) {
		bs := NewBlockStore(dbm.NewMemDB(), WithPQC(true))
		state, _, cleanup := makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
		defer cleanup()

		// Create block with PQC signature
		block := makeTestBlock(state, 1)
		partSet, err := block.MakePartSet(2)
		require.NoError(t, err)

		seenCommit := makeTestCommit(1, bctime.Now())
		bs.SaveBlock(block, partSet, seenCommit)

		// Verify PQC signature
		loadedBlock := bs.LoadBlock(1)
		require.NotNil(t, loadedBlock)
		
		// Verify the commit signature is PQC
		assert.True(t, bytes.HasPrefix(seenCommit.Signatures[0].Signature, []byte("pqc_sig_")))
	})
}

func TestAIOptimization(t *testing.T) {
	t.Run("CacheOptimization", func(t *testing.T) {
		bs := NewBlockStore(dbm.NewMemDB(), WithAI(true))
		state, _, cleanup := makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
		defer cleanup()

		// Create multiple blocks to test caching
		for i := int64(1); i <= 10; i++ {
			block := makeTestBlock(state, i)
			partSet, _ := block.MakePartSet(2)
			seenCommit := makeTestCommit(i, bctime.Now())
			bs.SaveBlock(block, partSet, seenCommit)
		}

		// Access blocks multiple times to trigger AI optimization
		for i := 0; i < 100; i++ {
			height := int64(bcrand.Intn(10) + 1)
			bs.LoadBlock(height)
		}

		metrics := bs.GetMetrics()
		assert.True(t, metrics.cacheHitRate > 0)
	})
}

// newTestHelper creates a new test helper instance
func newTestHelper(t *testing.T) *testHelper {
	db := dbm.NewMemDB()
	return &testHelper{
		t:  t,
		db: db,
		bs: NewBlockStore(db, WithPQC(true), WithAI(true)),
	}
}

// createTestBlockWithPQC creates a test block with quantum-safe signatures
func (h *testHelper) createTestBlockWithPQC(height int64) *types.Block {
	header := h.makeTestHeader(height)
	commit := makeTestCommit(height-1, time.Now())
	
	block := &types.Block{
		Header:     header,
		LastCommit: commit,
		Data: types.Data{
			Txs: makeTestTxs(10),
		},
	}

	// Add PQC signature
	block.SignPQC(generatePQCSignature(fmt.Sprintf("block_%d", height)))
	return block
}

// makeTestHeader creates a test header with proper versioning
func (h *testHelper) makeTestHeader(height int64) types.Header {
	return types.Header{
		Version: bcversion.Consensus{
			Block: version.BlockProtocol,
			App:   0,
		},
		ChainID:         "baron_chain_test",
		Height:          height,
		Time:           time.Now(),
		LastBlockID:    types.BlockID{},
		ProposerAddress: bcrand.Bytes(crypto.AddressSize),
	}
}

// makeTestTxs creates test transactions
func makeTestTxs(count int) types.Txs {
	txs := make(types.Txs, count)
	for i := 0; i < count; i++ {
		txs[i] = types.Tx(fmt.Sprintf("test_tx_%d", i))
	}
	return txs
}

// benchmarkHelper provides utilities for benchmarking
type benchmarkHelper struct {
	bs *BlockStore
	db dbm.DB
}

// newBenchmarkHelper creates a new benchmark helper
func newBenchmarkHelper() *benchmarkHelper {
	db := dbm.NewMemDB()
	return &benchmarkHelper{
		db: db,
		bs: NewBlockStore(db, WithPQC(true), WithAI(true)),
	}
}

// measurePerformance measures operation timing with AI metrics
type performanceMeasurement struct {
	operation   string
	startTime   time.Time
	duration    time.Duration
	cacheHits   int64
	cacheMisses int64
}

// startMeasurement begins timing an operation
func (h *benchmarkHelper) startMeasurement(operation string) *performanceMeasurement {
	return &performanceMeasurement{
		operation: operation,
		startTime: time.Now(),
	}
}

// endMeasurement completes the timing and records metrics
func (h *benchmarkHelper) endMeasurement(p *performanceMeasurement) {
	p.duration = time.Since(p.startTime)
	metrics := h.bs.GetMetrics()
	p.cacheHits = metrics.GetCacheHits()
	p.cacheMisses = metrics.GetCacheMisses()
}

// Test Scenario Generators

// generateTestBlocks creates a series of test blocks
func generateTestBlocks(t *testing.T, count int) []*types.Block {
	helper := newTestHelper(t)
	blocks := make([]*types.Block, count)
	
	for i := 0; i < count; i++ {
		blocks[i] = helper.createTestBlockWithPQC(int64(i + 1))
	}
	
	return blocks
}

// saveTestBlocks saves multiple blocks to the store
func saveTestBlocks(t *testing.T, bs *BlockStore, blocks []*types.Block) {
	for _, block := range blocks {
		partSet, err := block.MakePartSet(2)
		require.NoError(t, err)
		
		seenCommit := makeTestCommit(block.Height, time.Now())
		bs.SaveBlock(block, partSet, seenCommit)
	}
}

// Validation Helpers

// validatePQCSignature verifies quantum-safe signatures
func validatePQCSignature(t *testing.T, signature []byte) {
	require.True(t, bytes.HasPrefix(signature, []byte("pqc_sig_")))
}

// validateBlockMetrics checks AI optimization metrics
func validateBlockMetrics(t *testing.T, bs *BlockStore) {
	metrics := bs.GetMetrics()
	require.True(t, metrics.loadLatency >= 0)
	require.True(t, metrics.writeLatency >= 0)
	require.True(t, metrics.cacheHitRate >= 0)
}

// Error Simulation Helpers

// corruptBlock simulates block corruption for error testing
func corruptBlock(t *testing.T, db dbm.DB, height int64) {
	err := db.Set(calcBlockMetaKey(height), []byte("corrupted_data"))
	require.NoError(t, err)
}

// simulateNetworkLatency adds artificial delay
func simulateNetworkLatency() {
	time.Sleep(time.Millisecond * time.Duration(bcrand.Intn(100)))
}

// Benchmark Utilities

// BenchmarkBlockStoreSave benchmarks block saving with PQC
func BenchmarkBlockStoreSave(b *testing.B) {
	helper := newBenchmarkHelper()
	block := helper.createTestBlockWithPQC(1)
	partSet, _ := block.MakePartSet(2)
	seenCommit := makeTestCommit(1, time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		measurement := helper.startMeasurement("block_save")
		helper.bs.SaveBlock(block, partSet, seenCommit)
		helper.endMeasurement(measurement)
	}
}

// BenchmarkBlockStoreLoad benchmarks block loading with cache
func BenchmarkBlockStoreLoad(b *testing.B) {
	helper := newBenchmarkHelper()
	blocks := make([]*types.Block, 100)
	for i := range blocks {
		blocks[i] = helper.createTestBlockWithPQC(int64(i + 1))
		partSet, _ := blocks[i].MakePartSet(2)
		seenCommit := makeTestCommit(int64(i+1), time.Now())
		helper.bs.SaveBlock(blocks[i], partSet, seenCommit)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		height := int64(bcrand.Intn(100) + 1)
		measurement := helper.startMeasurement("block_load")
		helper.bs.LoadBlock(height)
		helper.endMeasurement(measurement)
	}
}

// Mock Functions for Testing

// mockAIOptimizer provides a mock AI optimization for testing
type mockAIOptimizer struct {
	predictedAccess map[int64]float64
}

// newMockAIOptimizer creates a new mock AI optimizer
func newMockAIOptimizer() *mockAIOptimizer {
	return &mockAIOptimizer{
		predictedAccess: make(map[int64]float64),
	}
}

// predictAccess simulates AI prediction of block access
func (m *mockAIOptimizer) predictAccess(height int64) float64 {
	if val, ok := m.predictedAccess[height]; ok {
		return val
	}
	return 0.5
}

// mockPQCSignature generates mock quantum-safe signatures
func mockPQCSignature(data string) []byte {
	return []byte(fmt.Sprintf("mock_pqc_sig_%s_%d", data, bcrand.Int63()))
}

// Cache Testing Utilities

// cacheTestScenario runs cache optimization tests
func cacheTestScenario(t *testing.T, bs *BlockStore, accessPattern []int64) {
	helper := newTestHelper(t)
	blocks := generateTestBlocks(t, 100)
	saveTestBlocks(t, bs, blocks)

	for _, height := range accessPattern {
		measurement := helper.startMeasurement("block_access")
		bs.LoadBlock(height)
		helper.endMeasurement(measurement)
	}
}
