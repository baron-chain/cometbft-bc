package types

import (
   "bytes"
   "errors"
   "fmt"
   "strings"
   
   "github.com/cometbft/cometbft/crypto"
   ce "github.com/cometbft/cometbft/crypto/encoding"
   cmtrand "github.com/cometbft/cometbft/libs/rand"
   cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

type Validator struct {
   Address     Address       `json:"address"`
   PubKey      crypto.PubKey `json:"pub_key"`
   VotingPower int64        `json:"voting_power"`
   ProposerPriority int64   `json:"proposer_priority"`
}

func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator {
   return &Validator{
       Address:          pubKey.Address(),
       PubKey:          pubKey,
       VotingPower:     votingPower,
       ProposerPriority: 0,
   }
}

func (v *Validator) ValidateBasic() error {
   if v == nil {
       return errors.New("nil validator")
   }
   if v.PubKey == nil {
       return errors.New("validator does not have a public key")
   }
   if v.VotingPower < 0 {
       return errors.New("validator has negative voting power") 
   }
   if len(v.Address) != crypto.AddressSize {
       return fmt.Errorf("validator address is the wrong size: %v", v.Address)
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
   if v.ProposerPriority > other.ProposerPriority {
       return v
   }
   if v.ProposerPriority < other.ProposerPriority {
       return other
   }
   result := bytes.Compare(v.Address, other.Address)
   switch {
   case result < 0:
       return v
   case result > 0:
       return other
   default:
       panic("Cannot compare identical validators")
   }
}

func (v *Validator) String() string {
   if v == nil {
       return "nil-Validator"
   }
   return fmt.Sprintf("Validator{%v %v VP:%v A:%v}",
       v.Address, v.PubKey, v.VotingPower, v.ProposerPriority)
}

func ValidatorListString(vals []*Validator) string {
   chunks := make([]string, len(vals))
   for i, val := range vals {
       chunks[i] = fmt.Sprintf("%s:%d", val.Address, val.VotingPower)
   }
   return strings.Join(chunks, ",")
}

func (v *Validator) Bytes() []byte {
   pk, err := ce.PubKeyToProto(v.PubKey)
   if err != nil {
       panic(err)
   }

   pbv := cmtprot
