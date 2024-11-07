package main

import (
	"fmt"
	"log"

	"github.com/cometbft/cometbft/test/loadtime/payload"
	"github.com/google/uuid"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
)

const (
	clientFactoryName = "loadtime-client"
	appName          = "loadtime"
	appShortDesc     = "Generate timestamped transaction load"
	appLongDesc      = "loadtime generates transaction load for measuring end-to-end latency " +
		"of transactions from submission to execution in a CometBFT network"
)

type (
	// ClientFactory creates and configures TxGenerator instances
	ClientFactory struct {
		ID []byte
	}

	// TxGenerator handles transaction generation with specified parameters
	TxGenerator struct {
		id         []byte
		conns      uint64
		rate       uint64
		size       uint64
		maxPayload uint64
	}
)

// Compile-time interface compliance checks
var (
	_ loadtest.ClientFactory = (*ClientFactory)(nil)
	_ loadtest.Client       = (*TxGenerator)(nil)
)

func main() {
	factory, err := initClientFactory()
	if err != nil {
		log.Fatalf("Failed to initialize client factory: %v", err)
	}

	runLoadTest(factory)
}

func initClientFactory() (*ClientFactory, error) {
	runID := uuid.New()
	factory := &ClientFactory{ID: runID[:]}

	if err := loadtest.RegisterClientFactory(clientFactoryName, factory); err != nil {
		return nil, fmt.Errorf("failed to register client factory: %w", err)
	}

	return factory, nil
}

func runLoadTest(factory *ClientFactory) {
	config := &loadtest.CLIConfig{
		AppName:              appName,
		AppShortDesc:        appShortDesc,
		AppLongDesc:         appLongDesc,
		DefaultClientFactory: clientFactoryName,
	}

	loadtest.Run(config)
}

// ValidateConfig ensures the provided configuration is valid
func (f *ClientFactory) ValidateConfig(cfg loadtest.Config) error {
	maxSize, err := payload.MaxUnpaddedSize()
	if err != nil {
		return fmt.Errorf("failed to get max unpadded size: %w", err)
	}

	if maxSize > cfg.Size {
		return fmt.Errorf("payload size (%d) exceeds configured size (%d)", maxSize, cfg.Size)
	}

	return nil
}

// NewClient creates a new TxGenerator with the provided configuration
func (f *ClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	maxSize, err := payload.MaxUnpaddedSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get max unpadded size: %w", err)
	}

	return &TxGenerator{
		id:         f.ID,
		conns:      uint64(cfg.Connections),
		rate:       uint64(cfg.Rate),
		size:       uint64(cfg.Size),
		maxPayload: maxSize,
	}, nil
}

// GenerateTx creates a new transaction with the configured parameters
func (c *TxGenerator) GenerateTx() ([]byte, error) {
	p := &payload.Payload{
		Connections: c.conns,
		Rate:       c.rate,
		Size:       c.size,
		Id:         c.id,
	}

	tx, err := payload.NewBytes(p)
	if err != nil {
		return nil, fmt.Errorf("failed to generate transaction: %w", err)
	}

	return tx, nil
}
