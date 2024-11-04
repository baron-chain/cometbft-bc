package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/baron-chain/cometbft-bc/crypto/tmhash"
	cmtrand "github.com/baron-chain/cometbft-bc/libs/rand"
	cmtproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
)

func TestCanonicalizeBlockID(t *testing.T) {
	// Helper function to create test BlockIDs
	makeBlockID := func(total uint32) cmtproto.BlockID {
		hash := cmtrand.Bytes(tmhash.Size)
		return cmtproto.BlockID{
			Hash: hash,
			PartSetHeader: cmtproto.PartSetHeader{
				Total: total,
				Hash:  hash,
			},
		}
	}

	// Helper function to create expected canonical BlockIDs
	makeCanonicalBlockID := func(bid cmtproto.BlockID) *cmtproto.CanonicalBlockID {
		return &cmtproto.CanonicalBlockID{
			Hash: bid.Hash,
			PartSetHeader: cmtproto.CanonicalPartSetHeader{
				Total: bid.PartSetHeader.Total,
				Hash:  bid.PartSetHeader.Hash,
			},
		}
	}

	testCases := []struct {
		name      string
		input     cmtproto.BlockID
		expected  *cmtproto.CanonicalBlockID
		wantError bool
	}{
		{
			name:     "valid block ID with 5 parts",
			input:    makeBlockID(5),
			expected: makeCanonicalBlockID(makeBlockID(5)),
		},
		{
			name:     "valid block ID with 10 parts",
			input:    makeBlockID(10),
			expected: makeCanonicalBlockID(makeBlockID(10)),
		},
		{
			name: "zero block ID",
			input: cmtproto.BlockID{
				Hash:          make([]byte, tmhash.Size),
				PartSetHeader: cmtproto.PartSetHeader{},
			},
			expected: nil,
		},
		{
			name:     "nil hash",
			input:    cmtproto.BlockID{},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalizeBlockID(tc.input)
			
			if tc.wantError {
				require.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			
			if tc.expected == nil {
				assert.Nil(t, got)
				return
			}
			
			require.NotNil(t, got)
			assert.Equal(t, tc.expected.Hash, got.Hash)
			assert.Equal(t, tc.expected.PartSetHeader.Total, got.PartSetHeader.Total)
			assert.Equal(t, tc.expected.PartSetHeader.Hash, got.PartSetHeader.Hash)
		})
	}
}

func TestMustCanonicalizeBlockID(t *testing.T) {
	t.Run("panics on error", func(t *testing.T) {
		// Create an invalid BlockID that would cause an error
		invalidBlockID := cmtproto.BlockID{
			Hash: []byte("invalid_length_hash"),
		}

		assert.Panics(t, func() {
			MustCanonicalizeBlockID(invalidBlockID)
		})
	})

	t.Run("succeeds with valid input", func(t *testing.T) {
		validHash := cmtrand.Bytes(tmhash.Size)
		validBlockID := cmtproto.BlockID{
			Hash: validHash,
			PartSetHeader: cmtproto.PartSetHeader{
				Total: 1,
				Hash:  validHash,
			},
		}

		assert.NotPanics(t, func() {
			result := MustCanonicalizeBlockID(validBlockID)
			require.NotNil(t, result)
			assert.Equal(t, validHash, result.Hash)
		})
	})
}
