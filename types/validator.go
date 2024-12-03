package types

import (
   "bytes"
   "errors"
   "fmt"
   "strings"
   
   "github.com/baron-chain/cometbft-bc/crypto"
   ce "github.com/baron-chain/cometbft-bc/crypto/encoding" 
   bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
   bcproto "github.com/baron-chain/cometbft-bc/proto/baronchain/types"
)

type Validator struct {
   Address          Address       `json:"address"`
   PubKey          crypto.PubKey `json:"pub_key"`
   VotingPower     int64         `json:"voting_power"`
   ProposerPriority int64        `json:"proposer_priority"`
   ReputationScore float64       `json:"reputation_score"`
}

func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator {
   return &Validator{
       Address:          pubKey.Address(),
       PubKey:          pubKey,
       VotingPower:     votingPower,
       ProposerPriority: 0,
       ReputationScore: 1.0,
   }
}

func (v *Validator) ValidateBasic() error {
   if v == nil {
       return errors.New("nil validator")
   }
   if v.PubKey == nil {
       return errors.New("missing public key")
   }
   if v.VotingPower < 0 {
       return errors.New("negative voting power")
   }
   if v.ReputationScore < 0 || v.ReputationScore > 1 {
       return errors.New("reputation score must be between 0 and 1")
   }
   if len(v.Address) != crypto.AddressSize {
       return fmt.Errorf("invalid address size: %v", v.Address)
   }
   return nil
}

func (v *Validator) Copy() *Validator {
   vCopy := *v
   return &vCopy
}

func (v *Validator) CompareProposerPriority(other *Validator) *Validator {
   if v == nil {
       return other
   }
   
   switch {
   case v.ProposerPriority > other.ProposerPriority:
       return v
   case v.ProposerPriority < other.ProposerPriority:
       return other
   default:
       if bytes.Compare(v.Address, other.Address) < 0 {
           return v
       }
       return other
   }
}

func (v *Validator) String() string {
   if v == nil {
       return "nil-Validator"
   }
   return fmt.Sprintf("Validator{%v PubKey:%v Power:%v Priority:%v Rep:%.2f}",
       v.Address, v.PubKey, v.VotingPower, v.ProposerPriority, v.ReputationScore)
}

func ValidatorListString(vals []*Validator) string {
   chunks := make([]string, len(vals))
   for i, val := range vals {
       chunks[i] = fmt.Sprintf("%s:%d:%.2f", val.Address, val.VotingPower, val.ReputationScore)
   }
   return strings.Join(chunks, ",")
}

func (v *Validator) ToProto() (*bcproto.Validator, error) {
   if v == nil {
       return nil, errors.New("nil validator")
   }
   
   pk, err := ce.PubKeyToProto(v.PubKey)
   if err != nil {
       return nil, fmt.Errorf("converting pubkey: %w", err)
   }
   
   return &bcproto.Validator{
       Address:          v.Address,
       PubKey:          pk,
       VotingPower:     v.VotingPower, 
       ProposerPriority: v.ProposerPriority,
       ReputationScore:  v.ReputationScore,
   }, nil
}
