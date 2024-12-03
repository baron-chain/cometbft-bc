package types

import (
   "fmt"
   "testing"
   
   "github.com/stretchr/testify/assert"
   "github.com/stretchr/testify/require"
   
   "github.com/baron-chain/cometbft-bc/crypto"
)

type validatorTestCase struct {
   desc   string
   val    *Validator 
   expErr string
}

func TestValidatorProtoBuf(t *testing.T) {
   validator, err := RandValidator(true, 100)
   require.NoError(t, err)
   validator.ReputationScore = 0.9 // Add reputation score for Baron Chain

   tests := []struct {
       desc        string
       val         *Validator
       expectProto bool
       expectDe    bool
   }{
       {
           desc:        "valid quantum-safe validator",
           val:         validator,
           expectProto: true,
           expectDe:    true,
       },
       {
           desc:        "empty validator", 
           val:         &Validator{},
           expectProto: false,
           expectDe:    false,
       },
       {
           desc:        "nil validator",
           val:         nil, 
           expectProto: false,
           expectDe:    false,
       },
   }

   for _, tc := range tests {
       t.Run(tc.desc, func(t *testing.T) {
           pb, err := tc.val.ToProto()
           if tc.expectProto {
               require.NoError(t, err)
               assert.NotNil(t, pb.GetPubKey())
               assert.NotZero(t, pb.GetVotingPower())
               assert.NotZero(t, pb.GetReputationScore())
           } else {
               require.Error(t, err)
               return
           }

           val, err := ValidatorFromProto(pb)
           if tc.expectDe {
               require.NoError(t, err)
               require.Equal(t, tc.val, val)
           } else {
               require.Error(t, err)
           }
       })
   }
}

func TestValidatorValidateBasic(t *testing.T) {
   priv := NewMockPV()
   pubKey, err := priv.GetPubKey()
   require.NoError(t, err)

   tests := []validatorTestCase{
       {
           desc:   "valid quantum-safe validator",
           val:    NewValidator(pubKey, 1),
           expErr: "",
       },
       {
           desc:   "nil validator",
           val:    nil,
           expErr: "nil validator",
       },
       {
           desc:   "missing pub key",
           val:    &Validator{PubKey: nil},
           expErr: "validator does not have a public key",
       },
       {
           desc:   "invalid voting power",
           val:    NewValidator(pubKey, -1),
           expErr: "validator has negative voting power",
       },
       {
           desc: "missing address",
           val: &Validator{
               PubKey:  pubKey,
               Address: nil,
           },
           expErr: "validator address is the wrong size: ",
       },
       {
           desc: "invalid reputation",
           val: &Validator{
               PubKey:          pubKey,
               ReputationScore: 2.0,
           },
           expErr: "reputation score must be between 0 and 1",
       },
   }

   for _, tc := range tests {
       t.Run(tc.desc, func(t *testing.T) {
           err := tc.val.ValidateBasic()
           if tc.expErr == "" {
               assert.NoError(t, err)
           } else {
               assert.Error(t, err)
               assert.Contains(t, err.Error(), tc.expErr) 
           }
       })
   }
}
