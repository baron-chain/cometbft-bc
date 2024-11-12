package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/baron-chain/cometbft-bc/libs/log"
	rpchttp "github.com/baron-chain/cometbft-bc/rpc/client/http"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/test/loadtime/payload"
	"github.com/baron-chain/cometbft-bc/types"
	"github.com/google/uuid"
)

const (
	workerPoolSize     = 16
	initialTimeout     = 1 * time.Minute
	stallTimeout       = 30 * time.Second
	batchTimeout       = 1 * time.Second
)

var (
	ErrNoTransactions = errors.New("failed to submit any transactions")
	ErrStallTimeout   = errors.New("unable to submit transactions due to stall")
)

type LoadTester struct {
	testnet    *e2e.Testnet
	runID      []byte
	txChan     chan types.Tx
	successChan chan struct{}
	startTime  time.Time
}

func NewLoadTester(testnet *e2e.Testnet) *LoadTester {
	return &LoadTester{
		testnet:     testnet,
		runID:       [16]byte(uuid.New())[:],
		txChan:      make(chan types.Tx),
		successChan: make(chan struct{}),
	}
}

func Load(ctx context.Context, testnet *e2e.Testnet) error {
	lt := NewLoadTester(testnet)
	return lt.Run(ctx)
}

func (lt *LoadTester) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lt.startTime = time.Now()
	logger.Info("load", "msg", log.NewLazySprintf("Starting transaction load (%v workers)...", workerPoolSize))

	go lt.generateTransactions(ctx)
	lt.startWorkers(ctx)

	return lt.monitorProgress(ctx)
}

func (lt *LoadTester) generateTransactions(ctx context.Context) {
	defer close(lt.txChan)

	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			batchCtx, cancel := context.WithTimeout(ctx, batchTimeout)
			lt.createTransactionBatch(batchCtx)
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (lt *LoadTester) createTransactionBatch(ctx context.Context) {
	workerPool := make(chan struct{}, workerPoolSize)
	var wg sync.WaitGroup

	for i := 0; i < lt.testnet.LoadTxBatchSize; i++ {
		select {
		case workerPool <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-workerPool }()
				
				lt.generateSingleTransaction(ctx)
			}()
		case <-ctx.Done():
			return
		}
	}

	wg.Wait()
}

func (lt *LoadTester) generateSingleTransaction(ctx context.Context) {
	tx, err := lt.createTransaction()
	if err != nil {
		logger.Error("failed to generate transaction", "error", err)
		return
	}

	select {
	case lt.txChan <- tx:
	case <-ctx.Done():
	}
}

func (lt *LoadTester) createTransaction() (types.Tx, error) {
	return payload.NewBytes(&payload.Payload{
		Id:          lt.runID,
		Size:        uint64(lt.testnet.LoadTxSizeBytes),
		Rate:        uint64(lt.testnet.LoadTxBatchSize),
		Connections: uint64(lt.testnet.LoadTxConnections),
	})
}

func (lt *LoadTester) startWorkers(ctx context.Context) {
	for _, node := range lt.testnet.Nodes {
		if node.SendNoLoad {
			continue
		}
		
		for w := 0; w < lt.testnet.LoadTxConnections; w++ {
			go lt.processTransactions(ctx, node)
		}
	}
}

func (lt *LoadTester) processTransactions(ctx context.Context, node *e2e.Node) {
	client, err := node.Client()
	if err != nil {
		logger.Info("failed to create node client", "error", err)
		return
	}

	for {
		select {
		case tx, ok := <-lt.txChan:
			if !ok {
				return
			}
			lt.sendTransaction(ctx, client, tx)
		case <-ctx.Done():
			return
		}
	}
}

func (lt *LoadTester) sendTransaction(ctx context.Context, client *rpchttp.HTTP, tx types.Tx) {
	if _, err := client.BroadcastTxSync(ctx, tx); err != nil {
		logger.Debug("failed to broadcast transaction", "error", err)
		return
	}
	
	select {
	case lt.successChan <- struct{}{}:
	case <-ctx.Done():
	}
}

func (lt *LoadTester) monitorProgress(ctx context.Context) error {
	successCount := 0
	timeout := initialTimeout

	for {
		select {
		case <-lt.successChan:
			successCount++
			timeout = stallTimeout

		case <-time.After(timeout):
			return fmt.Errorf("%w: %v", ErrStallTimeout, timeout)

		case <-ctx.Done():
			if successCount == 0 {
				return ErrNoTransactions
			}

			lt.logFinalStats(successCount)
			return nil
		}
	}
}

func (lt *LoadTester) logFinalStats(successCount int) {
	duration := time.Since(lt.startTime).Seconds()
	txPerSecond := float64(successCount) / duration
	
	logger.Info("load", "msg", log.NewLazySprintf(
		"Ending transaction load after %v txs (%.1f tx/s)...",
		successCount, txPerSecond))
}
