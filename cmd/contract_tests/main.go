package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/snikch/goodman/hooks"
	"github.com/snikch/goodman/transaction"
	"github.com/baron-chain/cometbft-bc/crypto/kyber"
	"github.com/baron-chain/cometbft-bc/types"
)

type TestHooks struct {
	hooks     *hooks.Hooks
	server    *hooks.Server
	kyberKeys *kyber.KeyPair
	mu        sync.RWMutex
}

const (
	txPrefix           = "Tx"
	evidencePrefix    = "Info > /broadcast_evidence"
	abciQueryPrefix   = "ABCI > /abci_query"
	txInfoPrefix      = "Info > /tx"
)

func NewTestHooks() *TestHooks {
	h := hooks.NewHooks()
	return &TestHooks{
		hooks:  h,
		server: hooks.NewServer(hooks.NewHooksRunner(h)),
	}
}

func (th *TestHooks) initQuantumSafe() error {
	keys, err := kyber.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate quantum-safe keys: %w", err)
	}
	
	th.mu.Lock()
	th.kyberKeys = keys
	th.mu.Unlock()
	
	return nil
}

func (th *TestHooks) setupHooks(ctx context.Context) {
	th.hooks.BeforeAll(func(txs []*transaction.Transaction) {
		if len(txs) > 0 {
			log.Printf("Starting test suite: %s", txs[0].Name)
		}
	})

	th.hooks.BeforeEach(func(tx *transaction.Transaction) {
		if th.shouldSkipTransaction(tx) {
			tx.Skip = true
			log.Printf("Skipping test: %s", tx.Name)
		}
	})
}

func (th *TestHooks) shouldSkipTransaction(tx *transaction.Transaction) bool {
	return strings.HasPrefix(tx.Name, txPrefix) ||
		strings.HasPrefix(tx.Name, evidencePrefix) ||
		strings.HasPrefix(tx.Name, abciQueryPrefix) ||
		strings.HasPrefix(tx.Name, txInfoPrefix)
}

func (th *TestHooks) Start() error {
	ctx := context.Background()
	
	if err := th.initQuantumSafe(); err != nil {
		return fmt.Errorf("failed to initialize quantum-safe testing: %w", err)
	}

	th.setupHooks(ctx)

	go func() {
		if err := th.server.Serve(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	return nil
}

func (th *TestHooks) Stop() {
	if th.server != nil && th.server.Listener != nil {
		if err := th.server.Listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
	}
}

func main() {
	th := NewTestHooks()
	
	if err := th.Start(); err != nil {
		log.Fatalf("Failed to start test hooks: %v", err)
	}
	
	defer th.Stop()
	
	log.Println("Baron Chain test hooks initialized successfully")
	
	// Block until program termination
	select {}
}
