package v1

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cometbft/cometbft/abci/example/kvstore"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/log"
	mempl "github.com/cometbft/cometbft/mempool"
	mempoolv1 "github.com/cometbft/cometbft/mempool/v1" //nolint:staticcheck // SA1019 Priority mempool deprecated but still supported in this release.
	"github.com/cometbft/cometbft/proxy"
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
	logger  log.Logger
}

// Global fuzzer instance with thread-safe initialization
var (
	fuzzer     *MempoolFuzzer
	initOnce   sync.Once
	initError  error
)

// NewMempoolFuzzer creates a new mempool fuzzer instance
func NewMempoolFuzzer() (*MempoolFuzzer, error) {
	// Initialize logger
	logger := log.NewNopLogger()

	// Create and configure application
	app := kvstore.NewApplication()
	clientCreator := proxy.NewLocalClientCreator(app)

	// Initialize ABCI client
	appConnMem, err := clientCreator.NewABCIClient()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create ABCI client: %v", 
			ErrMempoolInit, err)
	}

	// Start ABCI client
	if err := appConnMem.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientStart, err)
	}

	// Configure mempool
	cfg := config.DefaultMempoolConfig()
	cfg.Broadcast = false // Disable broadcasting for fuzzing

	// Create mempool instance
	mempool := mempoolv1.NewTxMempool(
		logger,
		cfg,
		appConnMem,
		0, // No height specified for fuzzing
	)

	return &MempoolFuzzer{
		mempool: mempool,
		logger:  logger,
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
	txInfo := mempl.TxInfo{} // Empty TxInfo for fuzzing
	return mf.mempool.CheckTx(tx, nil, txInfo)
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
