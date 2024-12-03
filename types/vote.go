package types

import (
   "testing"
   "time"

   "github.com/stretchr/testify/assert"
   "github.com/stretchr/testify/require"
   bcproto "github.com/baron-chain/cometbft-bc/proto/baronchain/types"
)

func TestVoteProto(t *testing.T) {
   ts := time.Now().UTC()
   blockID := makeBlockID([]byte("hash"), 1, []byte("ph"))

   tests := []struct {
       name string
       vote *Vote 
       pass bool
   }{
       {
           name: "valid quantum-safe vote",
           vote: &Vote{
               Type:             bcproto.PrevoteType,
               Height:           1,
               Round:            0,
               BlockID:          blockID,
               Timestamp:        ts,
               ValidatorAddress: []byte("val_addr"),
               ValidatorIndex:   1,
               Signature:        []byte("quantum_sig"),
               AiConfidence:     0.95,
           },
           pass: true,
       },
       {
           name: "nil vote",
           vote: nil,
           pass: false,
       },
   }

   for _, tc := range tests {
       t.Run(tc.name, func(t *testing.T) {
           pb := tc.vote.ToProto()
           if tc.pass {
               require.NotNil(t, pb)
               vote, err := VoteFromProto(pb)
               require.NoError(t, err)
               require.Equal(t, tc.vote, vote)
           } else {
               _, err := VoteFromProto(pb)
               require.Error(t, err)
           }
       })
   }
}

func TestVoteBasics(t *testing.T) {
   ts := time.Now()
   validBlockID := makeBlockID([]byte("hash"), 1, []byte("ph"))

   tests := []struct {
       name string
       vote *Vote
       err  string
   }{
       {
           name: "valid quantum-safe vote",
           vote: &Vote{
               Type:             bcproto.PrevoteType,
               Height:           1,
               Round:            0,
               BlockID:          validBlockID,
               Timestamp:        ts,
               ValidatorAddress: make([]byte, crypto.AddressSize),
               ValidatorIndex:   0, 
               Signature:        []byte("quantum_sig"),
               AiConfidence:     0.8,
           },
       },
       {
           name: "invalid type",
           vote: &Vote{
               Type: bcproto.SignedMsgType(999),
           },
           err: "invalid Type",
       },
       {
           name: "negative height",
           vote: &Vote{Height: -1},
           err:  "negative Height",
       },
       {
           name: "negative round",
           vote: &Vote{Height: 1, Round: -1},
           err:  "negative Round", 
       },
       {
           name: "invalid address",
           vote: &Vote{
               Height:           1,
               ValidatorAddress: []byte("short"),
           },
           err: "invalid address size",
       },
       {
           name: "missing signature",
           vote: &Vote{
               Height: 1,
               ValidatorAddress: make([]byte, crypto.AddressSize),
           },
           err: "missing signature",
       },
       {
           name: "invalid AI confidence", 
           vote: &Vote{
               Height: 1,
               AiConfidence: 1.5,
           },
           err: "AI confidence must be between 0 and 1",
       },
   }

   for _, tc := range tests {
       t.Run(tc.name, func(t *testing.T) {
           err := tc.vote.ValidateBasic()
           if tc.err != "" {
               require.Error(t, err)
               require.Contains(t, err.Error(), tc.err)
           } else {
               require.NoError(t, err)
           }
       })
   }
}

func TestVoteVerification(t *testing.T) {
   priv := NewMockPV()
   pub, err := priv.GetPubKey()
   require.NoError(t, err)
   
   vote := &Vote{
       Type:             bcproto.PrevoteType,
       Height:           1, 
       BlockID:          makeBlockID([]byte("hash"), 1, []byte("ph")),
       ValidatorAddress: pub.Address(),
       AiConfidence:     0.9,
   }

   chainID := "testchain"

   t.Run("valid quantum signature", func(t *testing.T) {
       signBytes := VoteSignBytes(chainID, vote.ToProto())
       sig, err := priv.Sign(signBytes)
       require.NoError(t, err)
       vote.Signature = sig
       
       err = vote.Verify(chainID, pub)
       require.NoError(t, err)
   })

   t.Run("invalid address", func(t *testing.T) {
       badVote := vote.Copy()
       badVote.ValidatorAddress = []byte("wrong")
       err = badVote.Verify(chainID, pub) 
       require.Equal(t, ErrVoteInvalidValidatorAddress, err)
   })

   t.Run("invalid signature", func(t *testing.T) {
       badVote := vote.Copy()
       badVote.Signature = []byte("wrong")
       err = badVote.Verify(chainID, pub)
       require.Equal(t, ErrVoteInvalidSignature, err)
   })
}
