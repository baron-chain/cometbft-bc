package types

import (
	"math"
	"testing"
	"time"

	"github.com/baron-chain/gogoproto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/baron-chain/cometbft-bc/crypto/kyber"
	"github.com/baron-chain/cometbft-bc/crypto/tmhash"
	"github.com/baron-chain/cometbft-bc/libs/protoio"
	bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
	bcproto "github.com/baron-chain/cometbft-bc/proto/types"
)

var (
	testProposal *Proposal
	pbp          *bcproto.Proposal
)

func init() {
	stamp, err := time.Parse(time.RFC3339Nano, "2018-02-11T07:09:22.765Z")
	if err != nil {
		panic(err)
	}
	
	testProposal = &Proposal{
		Height:   12345,
		Round:    23456,
		BlockID:  BlockID{Hash: bcrand.Bytes(tmhash.Size), PartSetHeader: PartSetHeader{Total: 111, Hash: bcrand.Bytes(tmhash.Size)}},
		POLRound: -1,
		Timestamp: stamp,
	}
	pbp = testProposal.ToProto()
}

func TestProposalSignable(t *testing.T) {
	chainID := "baron_chain_test"
	signBytes := ProposalSignBytes(chainID, pbp)
	pb := CanonicalizeProposal(chainID, pbp)

	expected, err := protoio.MarshalDelimited(&pb)
	require.NoError(t, err)
	require.Equal(t, expected, signBytes)
}

func TestProposalVerifySignature(t *testing.T) {
	privVal := NewKyberPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	prop := NewProposal(
		4, 2, 2,
		BlockID{bcrand.Bytes(tmhash.Size), PartSetHeader{777, bcrand.Bytes(tmhash.Size)}})
	p := prop.ToProto()
	signBytes := ProposalSignBytes("baron_chain_test", p)

	err = privVal.SignProposal("baron_chain_test", p)
	require.NoError(t, err)
	prop.Signature = p.Signature

	valid := pubKey.VerifySignature(signBytes, prop.Signature)
	require.True(t, valid)

	newProp := new(bcproto.Proposal)
	pb := prop.ToProto()

	bs, err := proto.Marshal(pb)
	require.NoError(t, err)

	err = proto.Unmarshal(bs, newProp)
	require.NoError(t, err)

	np, err := ProposalFromProto(newProp)
	require.NoError(t, err)

	newSignBytes := ProposalSignBytes("baron_chain_test", pb)
	require.Equal(t, signBytes, newSignBytes)
	valid = pubKey.VerifySignature(newSignBytes, np.Signature)
	require.True(t, valid)
}

func BenchmarkProposalOperations(b *testing.B) {
	privVal := NewKyberPV()
	pubKey, _ := privVal.GetPubKey()

	b.Run("WriteSignBytes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ProposalSignBytes("baron_chain_test", pbp)
		}
	})

	b.Run("Sign", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := privVal.SignProposal("baron_chain_test", pbp); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("VerifySignature", func(b *testing.B) {
		signBytes := ProposalSignBytes("baron_chain_test", pbp)
		for i := 0; i < b.N; i++ {
			pubKey.VerifySignature(signBytes, testProposal.Signature)
		}
	})
}

func TestProposalValidateBasic(t *testing.T) {
	privVal := NewKyberPV()
	blockID := makeBlockID(tmhash.Sum([]byte("blockhash")), math.MaxInt32, tmhash.Sum([]byte("partshash")))

	testCases := []struct {
		name         string
		malleate    func(*Proposal)
		expectError bool
	}{
		{"Valid Proposal", func(p *Proposal) {}, false},
		{"Invalid Type", func(p *Proposal) { p.Type = bcproto.PrecommitType }, true},
		{"Invalid Height", func(p *Proposal) { p.Height = -1 }, true},
		{"Invalid Round", func(p *Proposal) { p.Round = -1 }, true},
		{"Invalid POLRound", func(p *Proposal) { p.POLRound = -2 }, true},
		{"Invalid BlockId", func(p *Proposal) {
			p.BlockID = BlockID{[]byte{1, 2, 3}, PartSetHeader{111, []byte("blockparts")}}
		}, true},
		{"Empty Signature", func(p *Proposal) { p.Signature = []byte{} }, true},
		{"Oversized Signature", func(p *Proposal) { p.Signature = make([]byte, MaxSignatureSize+1) }, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prop := NewProposal(4, 2, 2, blockID)
			p := prop.ToProto()
			err := privVal.SignProposal("baron_chain_test", p)
			require.NoError(t, err)
			prop.Signature = p.Signature
			tc.malleate(prop)
			assert.Equal(t, tc.expectError, prop.ValidateBasic() != nil)
		})
	}
}
