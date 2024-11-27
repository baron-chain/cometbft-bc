package types

import (
    "errors"
    "fmt"
    "time"

    "github.com/baron-chain/cometbft-bc/crypto/ed25519"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    "github.com/baron-chain/cometbft-bc/crypto/secp256k1"
    "github.com/baron-chain/cometbft-bc/crypto/tmhash"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
)

const (
    MaxBlockSizeBytes    = 104857600 // 100MB
    BlockPartSizeBytes   uint32 = 65536 // 64kB
    MaxBlockPartsCount   = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1
    
    // Supported key types
    PubKeyTypeEd25519   = ed25519.KeyType
    PubKeyTypeSecp256k1 = secp256k1.KeyType
    PubKeyTypeKyber     = kyber.KeyType
)

var PubKeyTypesToNames = map[string]string{
    PubKeyTypeEd25519:   ed25519.PubKeyName,
    PubKeyTypeSecp256k1: secp256k1.PubKeyName,
    PubKeyTypeKyber:     kyber.PubKeyName,
}

type ConsensusParams struct {
    Block        BlockParams     `json:"block"`
    Evidence     EvidenceParams  `json:"evidence"`
    Validator    ValidatorParams `json:"validator"`
    Version      VersionParams   `json:"version"`
    QuantumSafe  QuantumParams   `json:"quantum_safe,omitempty"`  // New quantum params
    AI           AIParams        `json:"ai,omitempty"`           // New AI params
}

type BlockParams struct {
    MaxBytes        int64 `json:"max_bytes"`
    MaxGas          int64 `json:"max_gas"`
    MaxTransactions int64 `json:"max_transactions"` // Added for transaction limiting
}

type EvidenceParams struct {
    MaxAgeNumBlocks int64         `json:"max_age_num_blocks"`
    MaxAgeDuration  time.Duration `json:"max_age_duration"`
    MaxBytes        int64         `json:"max_bytes"`
}

type ValidatorParams struct {
    PubKeyTypes []string `json:"pub_key_types"`
}

type VersionParams struct {
    App uint64 `json:"app"`
}

// New params for quantum-safe features
type QuantumParams struct {
    Enabled          bool     `json:"enabled"`
    MinKeySize       int      `json:"min_key_size"`
    RequiredKeyTypes []string `json:"required_key_types"`
}

// New params for AI features
type AIParams struct {
    Enabled            bool    `json:"enabled"`
    MinTrustScore      float64 `json:"min_trust_score"`
    ValidationInterval int64   `json:"validation_interval"`
}

func DefaultConsensusParams() *ConsensusParams {
    return &ConsensusParams{
        Block:       DefaultBlockParams(),
        Evidence:    DefaultEvidenceParams(),
        Validator:   DefaultValidatorParams(),
        Version:     DefaultVersionParams(),
        QuantumSafe: DefaultQuantumParams(),
        AI:          DefaultAIParams(),
    }
}

func DefaultBlockParams() BlockParams {
    return BlockParams{
        MaxBytes:        22020096, // 21MB
        MaxGas:         -1,
        MaxTransactions: 10000,
    }
}

func DefaultQuantumParams() QuantumParams {
    return QuantumParams{
        Enabled:          true,
        MinKeySize:       256,
        RequiredKeyTypes: []string{PubKeyTypeKyber},
    }
}

func DefaultAIParams() AIParams {
    return AIParams{
        Enabled:            true,
        MinTrustScore:      0.7,
        ValidationInterval: 100,
    }
}

func (params ConsensusParams) ValidateBasic() error {
    if err := validateBlockParams(params.Block); err != nil {
        return fmt.Errorf("invalid block params: %w", err)
    }

    if err := validateEvidenceParams(params.Evidence, params.Block); err != nil {
        return fmt.Errorf("invalid evidence params: %w", err)
    }

    if err := validateValidatorParams(params.Validator); err != nil {
        return fmt.Errorf("invalid validator params: %w", err)
    }

    if params.QuantumSafe.Enabled {
        if err := validateQuantumParams(params.QuantumSafe); err != nil {
            return fmt.Errorf("invalid quantum params: %w", err)
        }
    }

    if params.AI.Enabled {
        if err := validateAIParams(params.AI); err != nil {
            return fmt.Errorf("invalid AI params: %w", err)
        }
    }

    return nil
}

func validateQuantumParams(params QuantumParams) error {
    if params.MinKeySize < 256 {
        return fmt.Errorf("minimum key size must be at least 256 bits, got %d", params.MinKeySize)
    }

    for _, keyType := range params.RequiredKeyTypes {
        if _, ok := PubKeyTypesToNames[keyType]; !ok {
            return fmt.Errorf("unknown quantum key type: %s", keyType)
        }
    }
    return nil
}

func validateAIParams(params AIParams) error {
    if params.MinTrustScore < 0 || params.MinTrustScore > 1 {
        return fmt.Errorf("min trust score must be between 0 and 1, got %f", params.MinTrustScore)
    }
    if params.ValidationInterval <= 0 {
        return fmt.Errorf("validation interval must be positive, got %d", params.ValidationInterval)
    }
    return nil
}

// DefaultEvidenceParams returns a default EvidenceParams.
func DefaultEvidenceParams() EvidenceParams {
	return EvidenceParams{
		MaxAgeNumBlocks: 100000, // 27.8 hrs at 1block/s
		MaxAgeDuration:  48 * time.Hour,
		MaxBytes:        1048576, // 1MB
	}
}

// DefaultValidatorParams returns a default ValidatorParams, which allows
// only ed25519 pubkeys.
func DefaultValidatorParams() ValidatorParams {
	return ValidatorParams{
		PubKeyTypes: []string{ABCIPubKeyTypeEd25519},
	}
}

func DefaultVersionParams() VersionParams {
	return VersionParams{
		App: 0,
	}
}

func IsValidPubkeyType(params ValidatorParams, pubkeyType string) bool {
	for i := 0; i < len(params.PubKeyTypes); i++ {
		if params.PubKeyTypes[i] == pubkeyType {
			return true
		}
	}
	return false
}

// Validate validates the ConsensusParams to ensure all values are within their
// allowed limits, and returns an error if they are not.
func (params ConsensusParams) ValidateBasic() error {
	if params.Block.MaxBytes <= 0 {
		return fmt.Errorf("block.MaxBytes must be greater than 0. Got %d",
			params.Block.MaxBytes)
	}
	if params.Block.MaxBytes > MaxBlockSizeBytes {
		return fmt.Errorf("block.MaxBytes is too big. %d > %d",
			params.Block.MaxBytes, MaxBlockSizeBytes)
	}

	if params.Block.MaxGas < -1 {
		return fmt.Errorf("block.MaxGas must be greater or equal to -1. Got %d",
			params.Block.MaxGas)
	}

	if params.Evidence.MaxAgeNumBlocks <= 0 {
		return fmt.Errorf("evidence.MaxAgeNumBlocks must be greater than 0. Got %d",
			params.Evidence.MaxAgeNumBlocks)
	}

	if params.Evidence.MaxAgeDuration <= 0 {
		return fmt.Errorf("evidence.MaxAgeDuration must be grater than 0 if provided, Got %v",
			params.Evidence.MaxAgeDuration)
	}

	if params.Evidence.MaxBytes > params.Block.MaxBytes {
		return fmt.Errorf("evidence.MaxBytesEvidence is greater than upper bound, %d > %d",
			params.Evidence.MaxBytes, params.Block.MaxBytes)
	}

	if params.Evidence.MaxBytes < 0 {
		return fmt.Errorf("evidence.MaxBytes must be non negative. Got: %d",
			params.Evidence.MaxBytes)
	}

	if len(params.Validator.PubKeyTypes) == 0 {
		return errors.New("len(Validator.PubKeyTypes) must be greater than 0")
	}

	// Check if keyType is a known ABCIPubKeyType
	for i := 0; i < len(params.Validator.PubKeyTypes); i++ {
		keyType := params.Validator.PubKeyTypes[i]
		if _, ok := ABCIPubKeyTypesToNames[keyType]; !ok {
			return fmt.Errorf("params.Validator.PubKeyTypes[%d], %s, is an unknown pubkey type",
				i, keyType)
		}
	}

	return nil
}

// Hash returns a hash of a subset of the parameters to store in the block header.
// Only the Block.MaxBytes and Block.MaxGas are included in the hash.
// This allows the ConsensusParams to evolve more without breaking the block
// protocol. No need for a Merkle tree here, just a small struct to hash.
func (params ConsensusParams) Hash() []byte {
	hasher := tmhash.New()

	hp := cmtproto.HashedParams{
		BlockMaxBytes: params.Block.MaxBytes,
		BlockMaxGas:   params.Block.MaxGas,
	}

	bz, err := hp.Marshal()
	if err != nil {
		panic(err)
	}

	_, err = hasher.Write(bz)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// Update returns a copy of the params with updates from the non-zero fields of p2.
// NOTE: note: must not modify the original
func (params ConsensusParams) Update(params2 *cmtproto.ConsensusParams) ConsensusParams {
	res := params // explicit copy

	if params2 == nil {
		return res
	}

	// we must defensively consider any structs may be nil
	if params2.Block != nil {
		res.Block.MaxBytes = params2.Block.MaxBytes
		res.Block.MaxGas = params2.Block.MaxGas
	}
	if params2.Evidence != nil {
		res.Evidence.MaxAgeNumBlocks = params2.Evidence.MaxAgeNumBlocks
		res.Evidence.MaxAgeDuration = params2.Evidence.MaxAgeDuration
		res.Evidence.MaxBytes = params2.Evidence.MaxBytes
	}
	if params2.Validator != nil {
		// Copy params2.Validator.PubkeyTypes, and set result's value to the copy.
		// This avoids having to initialize the slice to 0 values, and then write to it again.
		res.Validator.PubKeyTypes = append([]string{}, params2.Validator.PubKeyTypes...)
	}
	if params2.Version != nil {
		res.Version.App = params2.Version.App
	}
	return res
}

func (params *ConsensusParams) ToProto() cmtproto.ConsensusParams {
	return cmtproto.ConsensusParams{
		Block: &cmtproto.BlockParams{
			MaxBytes: params.Block.MaxBytes,
			MaxGas:   params.Block.MaxGas,
		},
		Evidence: &cmtproto.EvidenceParams{
			MaxAgeNumBlocks: params.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  params.Evidence.MaxAgeDuration,
			MaxBytes:        params.Evidence.MaxBytes,
		},
		Validator: &cmtproto.ValidatorParams{
			PubKeyTypes: params.Validator.PubKeyTypes,
		},
		Version: &cmtproto.VersionParams{
			App: params.Version.App,
		},
	}
}

func ConsensusParamsFromProto(pbParams cmtproto.ConsensusParams) ConsensusParams {
	return ConsensusParams{
		Block: BlockParams{
			MaxBytes: pbParams.Block.MaxBytes,
			MaxGas:   pbParams.Block.MaxGas,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: pbParams.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  pbParams.Evidence.MaxAgeDuration,
			MaxBytes:        pbParams.Evidence.MaxBytes,
		},
		Validator: ValidatorParams{
			PubKeyTypes: pbParams.Validator.PubKeyTypes,
		},
		Version: VersionParams{
			App: pbParams.Version.App,
		},
	}
}
