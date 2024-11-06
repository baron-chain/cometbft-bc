package report

import (
	"testing"
	"time"

	"github.com/cometbft/cometbft/test/loadtime/payload"
	"github.com/cometbft/cometbft/types"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MockBlockStore implements BlockStore interface for testing
type MockBlockStore struct {
	blocks map[int64]*types.Block
	base   int64
	height int64
}

func NewMockBlockStore(base, height int64) *MockBlockStore {
	return &MockBlockStore{
		blocks: make(map[int64]*types.Block),
		base:   base,
		height: height,
	}
}

func (m *MockBlockStore) Height() int64 { return m.height }
func (m *MockBlockStore) Base() int64   { return m.base }
func (m *MockBlockStore) LoadBlock(height int64) *types.Block {
	return m.blocks[height]
}

func (m *MockBlockStore) AddBlock(height int64, block *types.Block) {
	m.blocks[height] = block
}

func TestReportsAddDataPoint(t *testing.T) {
	tests := []struct {
		name         string
		duration     time.Duration
		expectedAvg  time.Duration
		expectedMax  time.Duration
		expectedMin  time.Duration
		connections  uint64
		rate         uint64
		size         uint64
		expectedNeg  int
		multipleData bool
	}{
		{
			name:        "single positive duration",
			duration:    100 * time.Millisecond,
			expectedAvg: 100 * time.Millisecond,
			expectedMax: 100 * time.Millisecond,
			expectedMin: 100 * time.Millisecond,
			connections: 1,
			rate:       1000,
			size:       1024,
			expectedNeg: 0,
		},
		{
			name:        "single negative duration",
			duration:    -100 * time.Millisecond,
			expectedAvg: -100 * time.Millisecond,
			expectedMax: -100 * time.Millisecond,
			expectedMin: -100 * time.Millisecond,
			connections: 1,
			rate:       1000,
			size:       1024,
			expectedNeg: 1,
		},
		{
			name:         "multiple durations",
			duration:     100 * time.Millisecond,
			expectedAvg:  150 * time.Millisecond,
			expectedMax:  200 * time.Millisecond,
			expectedMin:  100 * time.Millisecond,
			connections:  2,
			rate:        2000,
			size:        2048,
			expectedNeg:  0,
			multipleData: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reports := &Reports{
				s: make(map[uuid.UUID]Report),
			}

			id := uuid.Must(uuid.NewV4())
			blockTime := time.Now()
			hash := []byte("test_hash")

			reports.addDataPoint(id, tc.duration, blockTime, hash, tc.connections, tc.rate, tc.size)
			
			if tc.multipleData {
				reports.addDataPoint(id, 200*time.Millisecond, blockTime, hash, tc.connections, tc.rate, tc.size)
			}

			reports.calculateAll()
			require.Len(t, reports.List(), 1)

			report := reports.List()[0]
			assert.Equal(t, id, report.ID)
			assert.Equal(t, tc.connections, report.Connections)
			assert.Equal(t, tc.rate, report.Rate)
			assert.Equal(t, tc.size, report.Size)
			assert.Equal(t, tc.expectedNeg, report.NegativeCount)
			assert.Equal(t, tc.expectedAvg, report.Avg)
			assert.Equal(t, tc.expectedMax, report.Max)
			assert.Equal(t, tc.expectedMin, report.Min)
		})
	}
}

func TestGenerateFromBlockStore(t *testing.T) {
	store := NewMockBlockStore(1, 4)

	// Create test transactions
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	
	// Create payload data
	testTime := time.Now()
	p1 := &payload.Payload{
		Id:          id1[:],
		Time:        timestamppb.New(testTime),
		Connections: 1,
		Rate:        1000,
		Size:        1024,
	}
	p2 := &payload.Payload{
		Id:          id2[:],
		Time:        timestamppb.New(testTime.Add(time.Second)),
		Connections: 2,
		Rate:        2000,
		Size:        2048,
	}

	tx1, err := payload.ToBytes(p1)
	require.NoError(t, err)
	tx2, err := payload.ToBytes(p2)
	require.NoError(t, err)

	// Create blocks with transactions
	block1 := &types.Block{
		Header: types.Header{
			Height: 1,
			Time:   testTime,
		},
		Data: types.Data{
			Txs: []types.Tx{tx1},
		},
	}
	block2 := &types.Block{
		Header: types.Header{
			Height: 2,
			Time:   testTime.Add(2 * time.Second),
		},
		Data: types.Data{
			Txs: []types.Tx{tx2},
		},
	}
	block3 := &types.Block{
		Header: types.Header{
			Height: 3,
			Time:   testTime.Add(3 * time.Second),
		},
	}

	store.AddBlock(1, block1)
	store.AddBlock(2, block2)
	store.AddBlock(3, block3)

	reports, err := GenerateFromBlockStore(store)
	require.NoError(t, err)

	// Verify reports
	require.Len(t, reports.List(), 2)
	assert.Equal(t, 0, reports.ErrorCount())

	for _, report := range reports.List() {
		if report.ID == id1 {
			assert.Equal(t, uint64(1), report.Connections)
			assert.Equal(t, uint64(1000), report.Rate)
			assert.Equal(t, uint64(1024), report.Size)
		} else if report.ID == id2 {
			assert.Equal(t, uint64(2), report.Connections)
			assert.Equal(t, uint64(2000), report.Rate)
			assert.Equal(t, uint64(2048), report.Size)
		} else {
			t.Errorf("unexpected report ID: %v", report.ID)
		}
	}
}

func TestReportErrorHandling(t *testing.T) {
	store := NewMockBlockStore(1, 3)
	
	// Create an invalid transaction
	invalidTx := types.Tx("invalid payload data")
	
	block1 := &types.Block{
		Header: types.Header{
			Height: 1,
			Time:   time.Now(),
		},
		Data: types.Data{
			Txs: []types.Tx{invalidTx},
		},
	}
	block2 := &types.Block{
		Header: types.Header{
			Height: 2,
			Time:   time.Now().Add(time.Second),
		},
	}

	store.AddBlock(1, block1)
	store.AddBlock(2, block2)

	reports, err := GenerateFromBlockStore(store)
	require.NoError(t, err)
	assert.Equal(t, 1, reports.ErrorCount())
	assert.Empty(t, reports.List())
}

func TestToFloat(t *testing.T) {
	dataPoints := []DataPoint{
		{Duration: 100 * time.Millisecond},
		{Duration: 200 * time.Millisecond},
		{Duration: -50 * time.Millisecond},
	}

	result := toFloat(dataPoints)
	require.Len(t, result, len(dataPoints))
	assert.Equal(t, float64(100*time.Millisecond), result[0])
	assert.Equal(t, float64(200*time.Millisecond), result[1])
	assert.Equal(t, float64(-50*time.Millisecond), result[2])
}

func TestEmptyReportHandling(t *testing.T) {
	reports := &Reports{
		s: make(map[uuid.UUID]Report),
	}
	id := uuid.Must(uuid.NewV4())
	
	// Add empty report
	reports.s[id] = Report{
		ID:          id,
		Connections: 1,
		Rate:        1000,
		Size:        1024,
	}

	reports.calculateAll()
	require.Len(t, reports.List(), 1)
	
	report := reports.List()[0]
	assert.Equal(t, time.Duration(0), report.Min)
	assert.Equal(t, time.Duration(0), report.Max)
	assert.Equal(t, time.Duration(0), report.Avg)
	assert.Equal(t, time.Duration(0), report.StdDev)
}
