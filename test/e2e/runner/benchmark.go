package main

import (
	"context"
	"fmt"
	"math"
	"time"

	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/types"
)

const (
	blockTimeout    = 5 * time.Second
	maxBlockFetch   = 19 // Maximum blocks to fetch in one request
)

// TestnetStats contains benchmark statistics for the testnet
type TestnetStats struct {
	StartHeight int64
	EndHeight   int64
	Mean        time.Duration // Average time to produce a block
	StdDev      float64      // Standard deviation of block production
	Max         time.Duration // Longest time to produce a block
	Min         time.Duration // Shortest time to produce a block
}

func (t *TestnetStats) String() string {
	return fmt.Sprintf(`Benchmarked from height %v to %v
	Mean Block Interval: %v
	Standard Deviation: %f
	Max Block Interval: %v
	Min Block Interval: %v`,
		t.StartHeight,
		t.EndHeight,
		t.Mean,
		t.StdDev,
		t.Max,
		t.Min,
	)
}

// Benchmarker handles benchmark operations for the testnet
type Benchmarker struct {
	testnet *e2e.Testnet
	length  int64
}

// NewBenchmarker creates a new Benchmarker instance
func NewBenchmarker(testnet *e2e.Testnet, length int64) *Benchmarker {
	return &Benchmarker{
		testnet: testnet,
		length:  length,
	}
}

// Benchmark runs benchmark measurements on the testnet
func Benchmark(testnet *e2e.Testnet, benchmarkLength int64) error {
	benchmarker := NewBenchmarker(testnet, benchmarkLength)
	return benchmarker.Run()
}

func (b *Benchmarker) Run() error {
	block, err := b.waitForInitialBlock()
	if err != nil {
		return fmt.Errorf("failed to get initial block: %w", err)
	}

	logger.Info("Beginning benchmark period...", "height", block.Height)

	if err := b.waitForBenchmarkPeriod(block.Height); err != nil {
		return fmt.Errorf("benchmark period failed: %w", err)
	}

	stats, err := b.calculateStats()
	if err != nil {
		return fmt.Errorf("failed to calculate stats: %w", err)
	}

	logger.Info(stats.String())
	return nil
}

func (b *Benchmarker) waitForInitialBlock() (*types.Block, error) {
	block, _, err := waitForHeight(b.testnet, 0)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (b *Benchmarker) waitForBenchmarkPeriod(startHeight int64) error {
	waitTime := time.Duration(b.length*5) * time.Second
	endHeight, err := waitForAllNodes(b.testnet, startHeight+b.length, waitTime)
	if err != nil {
		return err
	}
	logger.Info("Ending benchmark period", "height", endHeight)
	return nil
}

func (b *Benchmarker) calculateStats() (*TestnetStats, error) {
	blocks, err := b.fetchBlockSample()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blocks: %w", err)
	}

	intervals := b.calculateBlockIntervals(blocks)
	stats := b.computeStatistics(intervals)
	stats.StartHeight = blocks[0].Header.Height
	stats.EndHeight = blocks[len(blocks)-1].Header.Height

	return stats, nil
}

func (b *Benchmarker) fetchBlockSample() ([]*types.BlockMeta, error) {
	archiveNode := b.testnet.ArchiveNodes()[0]
	client, err := archiveNode.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	status, err := client.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	toHeight := status.SyncInfo.LatestBlockHeight
	fromHeight := toHeight - b.length + 1

	if fromHeight <= b.testnet.InitialHeight {
		return nil, fmt.Errorf("testnet height insufficient for benchmarking (latest height %d)", toHeight)
	}

	return b.fetchBlockRange(ctx, client, fromHeight, toHeight)
}

func (b *Benchmarker) fetchBlockRange(ctx context.Context, client *rpchttp.HTTP, fromHeight, toHeight int64) ([]*types.BlockMeta, error) {
	var blocks []*types.BlockMeta
	currentHeight := fromHeight

	for currentHeight < toHeight {
		endHeight := min(currentHeight+maxBlockFetch, toHeight)
		resp, err := client.BlockchainInfo(ctx, currentHeight, endHeight)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch blockchain info: %w", err)
		}

		for i := len(resp.BlockMetas) - 1; i >= 0; i-- {
			block := resp.BlockMetas[i]
			if block.Header.Height != currentHeight {
				return nil, fmt.Errorf("unexpected block height: want %d, got %d", 
					currentHeight, block.Header.Height)
			}
			blocks = append(blocks, block)
			currentHeight++
		}
	}

	return blocks, nil
}

func (b *Benchmarker) calculateBlockIntervals(blocks []*types.BlockMeta) []time.Duration {
	intervals := make([]time.Duration, len(blocks)-1)
	lastTime := blocks[0].Header.Time

	for i, block := range blocks[1:] {
		intervals[i] = block.Header.Time.Sub(lastTime)
		lastTime = block.Header.Time
	}

	return intervals
}

func (b *Benchmarker) computeStatistics(intervals []time.Duration) *TestnetStats {
	if len(intervals) == 0 {
		return &TestnetStats{}
	}

	stats := &TestnetStats{
		Max: intervals[0],
		Min: intervals[0],
	}

	var sum time.Duration
	for _, interval := range intervals {
		sum += interval
		if interval > stats.Max {
			stats.Max = interval
		}
		if interval < stats.Min {
			stats.Min = interval
		}
	}

	stats.Mean = sum / time.Duration(len(intervals))
	stats.StdDev = b.calculateStdDev(intervals, stats.Mean)

	return stats
}

func (b *Benchmarker) calculateStdDev(intervals []time.Duration, mean time.Duration) float64 {
	var sumSquaredDiff float64
	for _, interval := range intervals {
		diff := (interval - mean).Seconds()
		sumSquaredDiff += math.Pow(diff, 2)
	}
	return math.Sqrt(sumSquaredDiff / float64(len(intervals)))
}

func min(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}
