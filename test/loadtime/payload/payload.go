package payload

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewBytes(t *testing.T) {
	timestamp := timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name        string
		payload     *Payload
		wantErr     bool
		errContains string
	}{
		{
			name: "valid payload with minimum size",
			payload: &Payload{
				Time:        timestamp,
				Size:        100,
				Connections: 1,
				Rate:        1000,
			},
			wantErr: false,
		},
		{
			name: "valid payload with maximum size",
			payload: &Payload{
				Time:        timestamp,
				Size:        maxPayloadSize,
				Connections: math.MaxUint64,
				Rate:        math.MaxUint64,
			},
			wantErr: false,
		},
		{
			name: "payload too large",
			payload: &Payload{
				Time:        timestamp,
				Size:        maxPayloadSize + 1,
				Connections: 1,
				Rate:        1000,
			},
			wantErr:     true,
			errContains: "too large",
		},
		{
			name: "size too small for payload",
			payload: &Payload{
				Time:        timestamp,
				Size:        10,
				Connections: math.MaxUint64,
				Rate:        math.MaxUint64,
			},
			wantErr:     true,
			errContains: "not large enough",
		},
		{
			name: "nil timestamp",
			payload: &Payload{
				Time:        nil,
				Size:        1000,
				Connections: 1,
				Rate:        1000,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NewBytes(tc.payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.True(t, bytes.HasPrefix(result, []byte(keyPrefix)))
			
			// Verify the result can be decoded back
			decoded, err := FromBytes(result)
			require.NoError(t, err)
			assert.Equal(t, tc.payload.Size, decoded.Size)
			assert.Equal(t, tc.payload.Connections, decoded.Connections)
			assert.Equal(t, tc.payload.Rate, decoded.Rate)
		})
	}
}

func TestFromBytes(t *testing.T) {
	validPayload := &Payload{
		Time:        timestamppb.Now(),
		Size:        1000,
		Connections: 1,
		Rate:        1000,
		Padding:     make([]byte, 1),
	}
	validBytes, err := proto.Marshal(validPayload)
	require.NoError(t, err)
	validHex := []byte(keyPrefix + hex.EncodeToString(validBytes))

	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid payload",
			input:   validHex,
			wantErr: false,
		},
		{
			name:        "missing prefix",
			input:       []byte("1234"),
			wantErr:     true,
			errContains: "missing key prefix",
		},
		{
			name:        "invalid hex",
			input:       []byte(keyPrefix + "ZZ"),
			wantErr:     true,
			errContains: "encoding/hex",
		},
		{
			name:        "invalid protobuf",
			input:       []byte(keyPrefix + "1234"),
			wantErr:     true,
			errContains: "proto:",
		},
		{
			name:        "empty input",
			input:       []byte{},
			wantErr:     true,
			errContains: "missing key prefix",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := FromBytes(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, result.Time)
		})
	}
}

func TestMaxUnpaddedSize(t *testing.T) {
	size, err := MaxUnpaddedSize()
	require.NoError(t, err)
	assert.Greater(t, size, 0)

	// Verify the size is sufficient for maximum values
	maxPayload := &Payload{
		Time:        timestamppb.Now(),
		Connections: math.MaxUint64,
		Rate:        math.MaxUint64,
		Size:        math.MaxUint64,
		Padding:     make([]byte, 1),
	}

	calcSize, err := CalculateUnpaddedSize(maxPayload)
	require.NoError(t, err)
	assert.Equal(t, size, calcSize)
}

func TestCalculateUnpaddedSize(t *testing.T) {
	timestamp := timestamppb.Now()

	tests := []struct {
		name        string
		payload     *Payload
		wantErr     bool
		errContains string
	}{
		{
			name: "valid payload",
			payload: &Payload{
				Time:        timestamp,
				Size:        1000,
				Connections: 1,
				Rate:        1000,
				Padding:     make([]byte, 1),
			},
			wantErr: false,
		},
		{
			name: "no padding",
			payload: &Payload{
				Time:        timestamp,
				Size:        1000,
				Connections: 1,
				Rate:        1000,
				Padding:     []byte{},
			},
			wantErr:     true,
			errContains: "expected length of padding to be 1",
		},
		{
			name: "too much padding",
			payload: &Payload{
				Time:        timestamp,
				Size:        1000,
				Connections: 1,
				Rate:        1000,
				Padding:     make([]byte, 10),
			},
			wantErr:     true,
			errContains: "expected length of padding to be 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			size, err := CalculateUnpaddedSize(tc.payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Greater(t, size, 0)
			assert.True(t, size > len(keyPrefix))
		})
	}
}

func BenchmarkNewBytes(b *testing.B) {
	payload := &Payload{
		Time:        timestamppb.Now(),
		Size:        1000,
		Connections: 1,
		Rate:        1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewBytes(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFromBytes(b *testing.B) {
	payload := &Payload{
		Time:        timestamppb.Now(),
		Size:        1000,
		Connections: 1,
		Rate:        1000,
	}
	bytes, err := NewBytes(payload)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FromBytes(bytes)
		if err != nil {
			b.Fatal(err)
		}
	}
}
