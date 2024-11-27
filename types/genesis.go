package types

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "time"

    "github.com/baron-chain/cometbft-bc/crypto"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    bcbytes "github.com/baron-chain/cometbft-bc/libs/bytes"
    bcjson "github.com/baron-chain/cometbft-bc/libs/json"
    bcos "github.com/baron-chain/cometbft-bc/libs/os"
    bctime "github.com/baron-chain/cometbft-bc/types/time"
)

const (
    MaxChainIDLen = 50
    MinPower      = 1
)

type GenesisValidator struct {
    Address       Address       `json:"address"`
    PubKey        crypto.PubKey `json:"pub_key"`
    Power         int64         `json:"power"`
    Name          string        `json:"name"`
    PQCPublicKey  []byte        `json:"pqc_pub_key,omitempty"`   // Quantum-safe public key
    AIScore       float64       `json:"ai_score,omitempty"`      // AI-based reputation score
}

type GenesisDoc struct {
    GenesisTime     time.Time          `json:"genesis_time"`
    ChainID         string             `json:"chain_id"`
    InitialHeight   int64              `json:"initial_height"`
    ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
    Validators      []GenesisValidator `json:"validators,omitempty"`
    AppHash         bcbytes.HexBytes   `json:"app_hash"`
    AppState        json.RawMessage    `json:"app_state,omitempty"`
    
    // Baron Chain specific fields
    PQCEnabled      bool               `json:"pqc_enabled"`
    AIValidation    bool               `json:"ai_validation"`
    SidechainConfig json.RawMessage    `json:"sidechain_config,omitempty"`
}

func (genDoc *GenesisDoc) SaveAs(file string) error {
    genDocBytes, err := bcjson.MarshalIndent(genDoc, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal genesis doc: %w", err)
    }
    return bcos.WriteFile(file, genDocBytes, 0644)
}

func (genDoc *GenesisDoc) ValidatorHash() []byte {
    vals := make([]*Validator, len(genDoc.Validators))
    for i, v := range genDoc.Validators {
        vals[i] = NewValidator(v.PubKey, v.Power)
        if genDoc.PQCEnabled {
            vals[i].PQCPublicKey = v.PQCPublicKey
        }
        if genDoc.AIValidation {
            vals[i].AIScore = v.AIScore
        }
    }
    vset := NewValidatorSet(vals)
    return vset.Hash()
}

func (genDoc *GenesisDoc) ValidateAndComplete() error {
    if err := validateBasicFields(genDoc); err != nil {
        return err
    }

    if err := validateValidators(genDoc); err != nil {
        return err
    }

    if err := validateQuantumSafety(genDoc); err != nil {
        return err
    }

    setDefaultValues(genDoc)

    return nil
}

func validateBasicFields(genDoc *GenesisDoc) error {
    if genDoc.ChainID == "" {
        return errors.New("genesis doc must include non-empty chain_id")
    }
    if len(genDoc.ChainID) > MaxChainIDLen {
        return fmt.Errorf("chain_id in genesis doc is too long (max: %d)", MaxChainIDLen)
    }
    if genDoc.InitialHeight < 0 {
        return fmt.Errorf("initial_height cannot be negative (got %v)", genDoc.InitialHeight)
    }
    return nil
}

func validateValidators(genDoc *GenesisDoc) error {
    for i, v := range genDoc.Validators {
        if v.Power < MinPower {
            return fmt.Errorf("validator %v has insufficient voting power: %d (min: %d)", 
                v.Name, v.Power, MinPower)
        }

        if len(v.Address) > 0 && !bytes.Equal(v.PubKey.Address(), v.Address) {
            return fmt.Errorf("incorrect address for validator %v, expected: %v", 
                v.Name, v.PubKey.Address())
        }

        if len(v.Address) == 0 {
            genDoc.Validators[i].Address = v.PubKey.Address()
        }

        if genDoc.PQCEnabled && len(v.PQCPublicKey) == 0 {
            return fmt.Errorf("validator %v missing quantum-safe public key", v.Name)
        }
    }
    return nil
}

func validateQuantumSafety(genDoc *GenesisDoc) error {
    if !genDoc.PQCEnabled {
        return nil
    }

    for _, v := range genDoc.Validators {
        if !kyber.ValidatePublicKey(v.PQCPublicKey) {
            return fmt.Errorf("invalid quantum-safe public key for validator %v", v.Name)
        }
    }
    return nil
}

func setDefaultValues(genDoc *GenesisDoc) {
    if genDoc.InitialHeight == 0 {
        genDoc.InitialHeight = 1
    }

    if genDoc.ConsensusParams == nil {
        genDoc.ConsensusParams = DefaultConsensusParams()
    }

    if genDoc.GenesisTime.IsZero() {
        genDoc.GenesisTime = bctime.Now()
    }
}

func GenesisDocFromJSON(jsonBlob []byte) (*GenesisDoc, error) {
    genDoc := GenesisDoc{}
    if err := bcjson.Unmarshal(jsonBlob, &genDoc); err != nil {
        return nil, fmt.Errorf("failed to unmarshal genesis doc: %w", err)
    }

    if err := genDoc.ValidateAndComplete(); err != nil {
        return nil, err
    }

    return &genDoc, nil
}

func GenesisDocFromFile(genDocFile string) (*GenesisDoc, error) {
    jsonBlob, err := os.ReadFile(genDocFile)
    if err != nil {
        return nil, fmt.Errorf("failed to read genesis file: %w", err)
    }

    genDoc, err := GenesisDocFromJSON(jsonBlob)
    if err != nil {
        return nil, fmt.Errorf("invalid genesis file %s: %w", genDocFile, err)
    }

    return genDoc, nil
}
