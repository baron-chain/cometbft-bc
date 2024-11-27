package types

import (
    "bytes"
    "errors"
    "fmt"

    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
)

// LightBlock represents a lightweight version of a block for light client verification
type LightBlock struct {
    *SignedHeader  `json:"signed_header"`
    ValidatorSet   *ValidatorSet `json:"validator_set"`
    PQCSignature   []byte       `json:"pqc_signature,omitempty"`   // Quantum-safe signature
    AITrustScore   float64      `json:"ai_trust_score,omitempty"` // AI-based trust score
}

func (lb LightBlock) ValidateBasic(chainID string) error {
    if lb.SignedHeader == nil {
        return errors.New("missing signed header")
    }
    if lb.ValidatorSet == nil {
        return errors.New("missing validator set")
    }

    if err := lb.SignedHeader.ValidateBasic(chainID); err != nil {
        return fmt.Errorf("invalid signed header: %w", err)
    }
    if err := lb.ValidatorSet.ValidateBasic(); err != nil {
        return fmt.Errorf("invalid validator set: %w", err)
    }

    // Validate quantum signature if present
    if len(lb.PQCSignature) > 0 {
        if err := lb.validatePQCSignature(); err != nil {
            return fmt.Errorf("invalid quantum signature: %w", err)
        }
    }

    // Validate validator set hash
    if valSetHash := lb.ValidatorSet.Hash(); !bytes.Equal(lb.SignedHeader.ValidatorsHash, valSetHash) {
        return fmt.Errorf("validator hash mismatch: header=%X set=%X", 
            lb.SignedHeader.ValidatorsHash, valSetHash)
    }

    return nil
}

func (lb LightBlock) validatePQCSignature() error {
    data := append(lb.SignedHeader.Hash(), lb.ValidatorSet.Hash()...)
    if !kyber.Verify(lb.ValidatorSet.GetProposer().PQCPublicKey, data, lb.PQCSignature) {
        return errors.New("invalid quantum signature")
    }
    return nil
}

type SignedHeader struct {
    *Header `json:"header"`
    Commit  *Commit `json:"commit"`
}

func (sh SignedHeader) ValidateBasic(chainID string) error {
    if sh.Header == nil {
        return errors.New("missing header")
    }
    if sh.Commit == nil {
        return errors.New("missing commit")
    }

    if err := sh.Header.ValidateBasic(); err != nil {
        return fmt.Errorf("invalid header: %w", err)
    }
    if err := sh.Commit.ValidateBasic(); err != nil {
        return fmt.Errorf("invalid commit: %w", err)
    }

    if sh.ChainID != chainID {
        return fmt.Errorf("chain ID mismatch: got %q, expected %q", sh.ChainID, chainID)
    }

    if sh.Commit.Height != sh.Height {
        return fmt.Errorf("height mismatch: header=%d commit=%d", sh.Height, sh.Commit.Height)
    }

    if hhash, chash := sh.Header.Hash(), sh.Commit.BlockID.Hash; !bytes.Equal(hhash, chash) {
        return fmt.Errorf("hash mismatch: header=%X commit=%X", hhash, chash)
    }

    return nil
}

func (lb *LightBlock) ToProto() (*bcproto.LightBlock, error) {
    if lb == nil {
        return nil, nil
    }

    pb := &bcproto.LightBlock{
        SignedHeader: lb.SignedHeader.ToProto(),
        PqcSignature: lb.PQCSignature,
        AiTrustScore: lb.AITrustScore,
    }

    var err error
    if lb.ValidatorSet != nil {
        if pb.ValidatorSet, err = lb.ValidatorSet.ToProto(); err != nil {
            return nil, fmt.Errorf("failed to convert validator set: %w", err)
        }
    }

    return pb, nil
}

func LightBlockFromProto(pb *bcproto.LightBlock) (*LightBlock, error) {
    if pb == nil {
        return nil, errors.New("nil light block")
    }

    sh, err := SignedHeaderFromProto(pb.SignedHeader)
    if err != nil {
        return nil, fmt.Errorf("invalid signed header: %w", err)
    }

    vs, err := ValidatorSetFromProto(pb.ValidatorSet)
    if err != nil {
        return nil, fmt.Errorf("invalid validator set: %w", err)
    }

    return &LightBlock{
        SignedHeader:  sh,
        ValidatorSet:  vs,
        PQCSignature:  pb.PqcSignature,
        AITrustScore: pb.AiTrustScore,
    }, nil
}

// Helper methods for string representation
func (lb LightBlock) String() string {
    return lb.StringIndented("")
}

func (lb LightBlock) StringIndented(indent string) string {
    return fmt.Sprintf(`LightBlock{
%s  SignedHeader: %v
%s  ValidatorSet: %v
%s  PQCSignature: %X
%s  AITrustScore: %.2f
%s}`,
        indent, lb.SignedHeader.StringIndented(indent+"  "),
        indent, lb.ValidatorSet.StringIndented(indent+"  "),
        indent, lb.PQCSignature,
        indent, lb.AITrustScore,
        indent)
}
