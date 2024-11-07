package v0

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/config"
	mempl "github.com/cometbft/cometbft/mempool"
	mempoolv0 "github.com/cometbft/cometbft/mempool/v0"
	"github.com/cometbft/cometbft/proxy"
)

const (
	// Initial blockchain height for testing
	initialHeight = 0
)

var (
	// Error definitions
	ErrMempoolInit = errors.New("failed to initialize mempool")
	ErrClientStart = errors.New("failed to start ABCI client")

	// FuzzResult represents possible fuzzing outcomes
	FuzzInvalid  int = -1
	FuzzIgnore   int = 0
	FuzzInterest int = 1
)

// MempoolFuzzer handles mempool fuzzing operations
type MempoolFuzzer struct {
	mempool mempl.Mempool
	config  *config.MempoolConfig
	client  proxy.AppConnMempool
}

// Global fuzzer instance with thread-safe initialization
var (
	fuzzer     *MempoolFuzzer
	initOnce   sync.Once
	initError  error
)

// newMempoolConfig creates a new mempool configuration for fuzzing
func newMempoolConfig() *config.MempoolConfig {
	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false // Disable broadcasting for fuzzing
	return cfg
}

// newABCIClient creates and starts a new ABCI client
func newABCIClient() (proxy.AppConnMempool, error) {
	app := kvstore.NewApplication()
	creator := proxy.NewLocalClientCreator(app)
	
	client, err := creator.NewABCIClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create ABCI client: %w", err)
	}

	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientStart, err)
	}

	return client, nil
}

// NewMempoolFuzzer creates a new mempool fuzzer instance
func NewMempoolFuzzer() (*MempoolFuzzer, error) {
	// Initialize configuration
	cfg := newMempoolConfig()

	// Initialize ABCI client
	client, err := newABCIClient()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMempoolInit, err)
	}

	// Create mempool instance
	mempool := mempoolv0.NewCListMempool(
		cfg,
		client,
		initialHeight,
	)

	return &MempoolFuzzer{
		mempool: mempool,
		config:  cfg,
		client:  client,
	}, nil
}

// initializeFuzzer ensures thread-safe singleton initialization
func initializeFuzzer() error {
	initOnce.Do(func() {
		var err error
		fuzzer, err = NewMempoolFuzzer()
		if err != nil {
			initError = fmt.Errorf("failed to initialize fuzzer: %w", err)
		}
	})
	return initError
}

// CheckTransaction attempts to add a transaction to the mempool
func (mf *MempoolFuzzer) CheckTransaction(tx []byte) error {
	if len(tx) == 0 {
		return fmt.Errorf("empty transaction")
	}

	txInfo := mempl.TxInfo{} // Empty TxInfo for fuzzing
	return mf.mempool.CheckTx(tx, nil, txInfo)
}

// Close cleans up the fuzzer resources
func (mf *MempoolFuzzer) Close() error {
	// Add cleanup if needed in future
	return nil
}

// Fuzz implements the fuzzing entry point
func Fuzz(data []byte) int {
	// Initialize fuzzer if needed
	if err := initializeFuzzer(); err != nil {
		return FuzzInvalid
	}

	// Validate input
	if len(data) == 0 {
		return FuzzIgnore
	}

	// Check transaction
	err := fuzzer.CheckTransaction(data)
	if err != nil {
		// Known errors are uninteresting
		return FuzzIgnore
	}

	// Successfully added to mempool
	return FuzzInterest
}
