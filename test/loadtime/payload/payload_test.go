package payload_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/cometbft/cometbft/test/loadtime/payload"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	payloadSizeTarget = 1024 // 1kb
	maxPayloadSize    = 4 * 1024 * 1024
)

func TestMaxUnpaddedSize(t *testing.T) {
	tests := []struct {
		name          string
		sizeTarget    uint64
		expectSuccess bool
	}{
		{
			name:          "standard size",
			sizeTarget:    payloadSizeTarget,
			expectSuccess: true,
		},
		{
			name:          "minimum size",
			sizeTarget:    1,
			expectSuccess: true,
		},
		{
			name:          "maximum size",
			sizeTarget:    maxPayloadSize,
			expectSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			size, err := payload.MaxUnpaddedSize()
			if tc.expectSuccess {
				require.NoError(t, err)
				assert.LessOrEqual(t, size, int(tc.sizeTarget), 
					"unpadded payload size exceeds target")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		input         *payload.Payload
		expectSuccess bool
		expectedErr   string
	}{
		{
			name: "standard payload",
			input: &payload.Payload{
				Size:        payloadSizeTarget,
				Connections: 512,
				Rate:       4,
				Id:         uuid.New().Bytes(),
				Time:       timestamppb.New(time.Now().UTC()),
			},
			expectSuccess: true,
		},
		{
			name: "maximum values",
			input: &payload.Payload{
				Size:        maxPayloadSize,
				Connections: ^uint64(0),
				Rate:       ^uint64(0),
				Id:         uuid.New().Bytes(),
				Time:       timestamppb.New(time.Now().UTC()),
			},
			expectSuccess: true,
		},
		{
			name: "minimum values",
			input: &payload.Payload{
				Size:        1,
				Connections: 1,
				Rate:       1,
				Id:         uuid.New().Bytes(),
				Time:       timestamppb.New(time.Now().UTC()),
			},
			expectSuccess: true,
		},
		{
			name: "zero values",
			input: &payload.Payload{
				Size:        0,
				Connections: 0,
				Rate:       0,
				Id:         uuid.New().Bytes(),
			},
			expectSuccess: false,
			expectedErr:   "not large enough",
		},
		{
			name: "missing ID",
			input: &payload.Payload{
				Size:        payloadSizeTarget,
				Connections: 512,
				Rate:       4,
			},
			expectSuccess: true,
		},
		{
			name: "oversized payload",
			input: &payload.Payload{
				Size:        maxPayloadSize + 1,
				Connections: 512,
				Rate:       4,
				Id:         uuid.New().Bytes(),
			},
			expectSuccess: false,
			expectedErr:   "too large",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create payload bytes
			b, err := payload.NewBytes(tc.input)
			if !tc.expectSuccess {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				return
			}
			require.NoError(t, err)

			// Verify size
			assert.GreaterOrEqual(t, len(b), int(tc.input.Size),
				"payload size less than expected")

			// Read back payload
			p, err := payload.FromBytes(b)
			require.NoError(t, err)

			// Verify fields
			assert.Equal(t, tc.input.Size, p.Size, "size mismatch")
			assert.Equal(t, tc.input.Connections, p.Connections, "connections mismatch")
			assert.Equal(t, tc.input.Rate, p.Rate, "rate mismatch")
			if tc.input.Id != nil {
				assert.True(t, bytes.Equal(tc.input.Id, p.Id), "ID mismatch")
			}
			if tc.input.Time != nil {
				assert.Equal(t, tc.input.Time.AsTime().Unix(), p.Time.AsTime().Unix(), 
					"timestamp mismatch")
			}
		})
	}
}

func TestPayloadBoundaries(t *testing.T) {
	t.Run("invalid prefix", func(t *testing.T) {
		_, err := payload.FromBytes([]byte("invalid data"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing key prefix")
	})

	t.Run("corrupt payload", func(t *testing.T) {
		b, err := payload.NewBytes(&payload.Payload{
			Size:        payloadSizeTarget,
			Id:         uuid.New().Bytes(),
		})
		require.NoError(t, err)
		
		// Corrupt the data
		b[len(b)-1] ^= 0xFF
		
		_, err = payload.FromBytes(b)
		require.Error(t, err)
	})
}

func BenchmarkPayloadRoundTrip(b *testing.B) {
	sizes := []int{
		1024,    // 1KB
		64*1024, // 64KB
		1<<20,   // 1MB
	}

	for _, size := range sizes {
		b.Run(formatSize(size), func(b *testing.B) {
			p := &payload.Payload{
				Size:        uint64(size),
				Connections: 512,
				Rate:       1000,
				Id:         uuid.New().Bytes(),
				Time:       timestamppb.Now(),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, err := payload.NewBytes(p)
				if err != nil {
					b.Fatal(err)
				}
				_, err = payload.FromBytes(data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper function to format sizes for benchmark names
func formatSize(size int) string {
	switch {
	case size >= 1<<20:
		return string(rune('0'+size/(1<<20))) + "MB"
	case size >= 1<<10:
		return string(rune('0'+size/(1<<10))) + "KB"
	default:
		return string(rune('0'+size)) + "B"
	}
}
