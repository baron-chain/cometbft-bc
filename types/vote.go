package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

func TestVoteProto(t *testing.T) {
	timestamp := time.Now().Round(0).UTC()
	blockID := makeBlockID([]byte("hash"), 1, []byte("part_set_hash"))

	tests := []struct {
		name        string
		vote       *Vote
		expectPass bool
	}{
		{
			name: "valid vote",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           1,
				Round:            0,
				BlockID:          blockID,
				Timestamp:        timestamp,
				ValidatorAddress: []byte("validator_address"),
				ValidatorIndex:   1,
				Signature:        []byte("signature"),
			},
			expectPass: true,
		},
		{
			name:        "nil vote",
			vote:       nil,
			expectPass: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			protoVote := tc.vote.ToProto()
			if tc.expectPass {
				require.NotNil(t, protoVote)
				voteFromProto, err := VoteFromProto(protoVote)
				require.NoError(t, err)
				require.Equal(t, tc.vote, voteFromProto)
			} else {
				_, err := VoteFromProto(protoVote)
				require.Error(t, err)
			}
		})
	}
}

func TestVoteValidateBasic(t *testing.T) {
	timestamp := time.Now()
	validBlockID := makeBlockID([]byte("hash"), 1, []byte("part_set_hash"))

	tests := []struct {
		name      string
		vote      *Vote
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid vote",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           1,
				Round:            0,
				BlockID:          validBlockID,
				Timestamp:        timestamp,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				ValidatorIndex:   0,
				Signature:        []byte("signature"),
			},
			expectErr: false,
		},
		{
			name: "invalid vote type",
			vote: &Vote{
				Type:             cmtproto.SignedMsgType(999),
				Height:           1,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        []byte("signature"),
			},
			expectErr: true,
			errMsg:    "invalid Type",
		},
		{
			name: "negative height",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           -1,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        []byte("signature"),
			},
			expectErr: true,
			errMsg:    "negative or zero Height",
		},
		{
			name: "negative round",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           1,
				Round:            -1,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        []byte("signature"),
			},
			expectErr: true,
			errMsg:    "negative Round",
		},
		{
			name: "invalid validator address",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           1,
				ValidatorAddress: []byte("too-short"),
				Signature:        []byte("signature"),
			},
			expectErr: true,
			errMsg:    "expected ValidatorAddress size to be",
		},
		{
			name: "missing signature",
			vote: &Vote{
				Type:             cmtproto.PrevoteType,
				Height:           1,
				ValidatorAddress: make([]byte, crypto.AddressSize),
			},
			expectErr: true,
			errMsg:    "signature is missing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vote.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVoteVerify(t *testing.T) {
	privVal := NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	
	vote := &Vote{
		Type:             cmtproto.PrevoteType,
		Height:           1,
		Round:            0,
		BlockID:          makeBlockID([]byte("hash"), 1, []byte("part_set_hash")),
		Timestamp:        time.Now().UTC(),
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   0,
	}

	chainID := "test_chain_id"
	
	t.Run("valid signature", func(t *testing.T) {
		signBytes := VoteSignBytes(chainID, vote.ToProto())
		sig, err := privVal.Sign(signBytes)
		require.NoError(t, err)
		vote.Signature = sig
		
		err = vote.Verify(chainID, pubKey)
		require.NoError(t, err)
	})

	t.Run("invalid validator address", func(t *testing.T) {
		invalidVote := vote.Copy()
		invalidVote.ValidatorAddress = []byte("wrong_address")
		
		err = invalidVote.Verify(chainID, pubKey)
		require.Error(t, err)
		require.Equal(t, ErrVoteInvalidValidatorAddress, err)
	})

	t.Run("invalid signature", func(t *testing.T) {
		invalidVote := vote.Copy()
		invalidVote.Signature = []byte("wrong_signature")
		
		err = invalidVote.Verify(chainID, pubKey)
		require.Error(t, err)
		require.Equal(t, ErrVoteInvalidSignature, err)
	})
}

func TestVoteString(t *testing.T) {
	timestamp := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	vote := &Vote{
		Type:             cmtproto.PrevoteType,
		Height:           12345,
		Round:            1,
		BlockID:          makeBlockID([]byte("hash"), 1, []byte("part_set_hash")),
		Timestamp:        timestamp,
		ValidatorAddress: []byte("validator_address"),
		ValidatorIndex:   2,
		Signature:        []byte("signature"),
	}

	str := vote.String()
	require.Contains(t, str, "Prevote")
	require.Contains(t, str, "12345")
	require.Contains(t, str, "01") // Round
	require.NotEqual(t, str, nilVoteStr)

	nilVote := (*Vote)(nil)
	require.Equal(t, nilVoteStr, nilVote.String())
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) BlockID {
	return BlockID{
		Hash: hash,
		PartSetHeader: PartSetHeader{
			Total: partSetSize,
			Hash:  partSetHash,
		},
	}
}
