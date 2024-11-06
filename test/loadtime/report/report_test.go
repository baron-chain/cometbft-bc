package report_test

import (
	"testing"
	"time"

	"github.com/cometbft/cometbft/test/loadtime/payload"
	"github.com/cometbft/cometbft/test/loadtime/report"
	"github.com/cometbft/cometbft/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockBlockStore implements a simple mock for testing
type mockBlockStore struct {
	base   int64
	blocks []*types.Block
}

func (m *mockBlockStore) Height() int64 { return m.base + int64(len(m.blocks)) }
func (m *mockBlockStore) Base() int64   { return m.base }
func (m *mockBlockStore) LoadBlock(i int64) *types.Block { return m.blocks[i-m.base] }

// helper functions
func createPayload(t *testing.T, id uuid.UUID, timestamp time.Time, size uint64) types.Tx {
	t.Helper()
	pb, err := payload.NewBytes(&payload.Payload{
		Id:          id[:],
		Time:        timestamppb.New(timestamp),
		Size:        size,
		Rate:        1000,
		Connections: 1,
	})
	require.NoError(t, err, "Failed to create payload")
	return pb
}

func createMockBlockStore(blocks []*types.Block) *mockBlockStore {
	return &mockBlockStore{
		base:   1,
		blocks: blocks,
	}
}

func TestGenerateReport(t *testing.T) {
	baseTime := time.Now()
	testID := uuid.New()

	tests := []struct {
		name           string
		setupBlocks    func() []*types.Block
		expectedReport struct {
			dataPoints     int
			negativeCount int
			errorCount    int
			avg           time.Duration
			min           time.Duration
			max           time.Duration
			stdDev        time.Duration
		}
	}{
		{
			name: "basic report generation",
			setupBlocks: func() []*types.Block {
				return []*types.Block{
					{
						Data: types.Data{
							Txs: []types.Tx{
								createPayload(t, testID, baseTime.Add(-10*time.Second), 1024),
								createPayload(t, testID, baseTime.Add(-4*time.Second), 1024),
							},
						},
					},
					{
						Header: types.Header{Time: baseTime},
						Data: types.Data{
							Txs: []types.Tx{[]byte("error")},
						},
					},
					{
						Data: types.Data{
							Txs: []types.Tx{
								createPayload(t, testID, baseTime.Add(2*time.Second), 1024),
								createPayload(t, testID, baseTime.Add(2*time.Second), 1024),
							},
						},
					},
					{
						Header: types.Header{Time: baseTime.Add(time.Second)},
						Data:   types.Data{},
					},
				}
			},
			expectedReport: struct {
				dataPoints     int
				negativeCount int
				errorCount    int
				avg           time.Duration
				min           time.Duration
				max           time.Duration
				stdDev        time.Duration
			}{
				dataPoints:     4,
				negativeCount: 2,
				errorCount:    1,
				avg:           3 * time.Second,
				min:           -time.Second,
				max:           10 * time.Second,
				stdDev:        5228129047 * time.Nanosecond,
			},
		},
		{
			name: "empty blocks",
			setupBlocks: func() []*types.Block {
				return []*types.Block{
					{
						Data: types.Data{},
					},
					{
						Header: types.Header{Time: baseTime},
						Data:   types.Data{},
					},
				}
			},
			expectedReport: struct {
				dataPoints     int
				negativeCount int
				errorCount    int
				avg           time.Duration
				min           time.Duration
				max           time.Duration
				stdDev        time.Duration
			}{
				dataPoints:  0,
				errorCount: 0,
			},
		},
		{
			name: "all invalid transactions",
			setupBlocks: func() []*types.Block {
				return []*types.Block{
					{
						Data: types.Data{
							Txs: []types.Tx{[]byte("error1"), []byte("error2")},
						},
					},
					{
						Header: types.Header{Time: baseTime},
						Data:   types.Data{},
					},
				}
			},
			expectedReport: struct {
				dataPoints     int
				negativeCount int
				errorCount    int
				avg           time.Duration
				min           time.Duration
				max           time.Duration
				stdDev        time.Duration
			}{
				dataPoints:  0,
				errorCount: 2,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := createMockBlockStore(tc.setupBlocks())
			rs, err := report.GenerateFromBlockStore(store)
			require.NoError(t, err, "Unexpected error generating report")

			assert.Equal(t, tc.expectedReport.errorCount, rs.ErrorCount(), "Error count mismatch")

			reports := rs.List()
			if tc.expectedReport.dataPoints == 0 {
				assert.Empty(t, reports, "Expected empty report list")
				return
			}

			require.Len(t, reports, 1, "Expected single report")
			r := reports[0]

			assert.Len(t, r.All, tc.expectedReport.dataPoints, "Data points count mismatch")
			assert.Equal(t, tc.expectedReport.negativeCount, r.NegativeCount, "Negative count mismatch")
			assert.Equal(t, tc.expectedReport.avg, r.Avg, "Average duration mismatch")
			assert.Equal(t, tc.expectedReport.min, r.Min, "Minimum duration mismatch")
			assert.Equal(t, tc.expectedReport.max, r.Max, "Maximum duration mismatch")
			assert.Equal(t, tc.expectedReport.stdDev, r.StdDev, "Standard deviation mismatch")
		})
	}
}

func TestGenerateReport_MultipleIDs(t *testing.T) {
	baseTime := time.Now()
	id1 := uuid.New()
	id2 := uuid.New()

	blocks := []*types.Block{
		{
			Data: types.Data{
				Txs: []types.Tx{
					createPayload(t, id1, baseTime.Add(-5*time.Second), 1024),
					createPayload(t, id2, baseTime.Add(-3*time.Second), 2048),
				},
			},
		},
		{
			Header: types.Header{Time: baseTime},
			Data:   types.Data{},
		},
	}

	store := createMockBlockStore(blocks)
	rs, err := report.GenerateFromBlockStore(store)
	require.NoError(t, err)

	reports := rs.List()
	require.Len(t, reports, 2, "Expected two reports")

	// Reports are not guaranteed to be in any specific order
	for _, r := range reports {
		if r.ID == id1 {
			assert.Equal(t, uint64(1024), r.Size)
			assert.Equal(t, 5*time.Second, r.Max)
		} else {
			assert.Equal(t, uint64(2048), r.Size)
			assert.Equal(t, 3*time.Second, r.Max)
		}
	}
}

func TestGenerateReport_EdgeCases(t *testing.T) {
	t.Run("single block chain", func(t *testing.T) {
		store := createMockBlockStore([]*types.Block{
			{
				Data: types.Data{
					Txs: []types.Tx{createPayload(t, uuid.New(), time.Now(), 1024)},
				},
			},
		})
		rs, err := report.GenerateFromBlockStore(store)
		require.NoError(t, err)
		assert.Empty(t, rs.List(), "Expected no reports for single block chain")
	})

	t.Run("nil block store", func(t *testing.T) {
		_, err := report.GenerateFromBlockStore(nil)
		assert.Error(t, err, "Expected error for nil block store")
	})
}

func BenchmarkGenerateReport(b *testing.B) {
	baseTime := time.Now()
	testID := uuid.New()
	blocks := []*types.Block{
		{
			Data: types.Data{
				Txs: []types.Tx{
					createPayload(b, testID, baseTime.Add(-10*time.Second), 1024),
					createPayload(b, testID, baseTime.Add(-4*time.Second), 1024),
				},
			},
		},
		{
			Header: types.Header{Time: baseTime},
			Data: types.Data{
				Txs: []types.Tx{createPayload(b, testID, baseTime.Add(2*time.Second), 1024)},
			},
		},
	}
	store := createMockBlockStore(blocks)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = report.GenerateFromBlockStore(store)
	}
}
