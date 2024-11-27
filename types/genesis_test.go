package types

import (
    "encoding/json"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/baron-chain/cometbft-bc/crypto/ed25519"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    bcjson "github.com/baron-chain/cometbft-bc/libs/json"
    bctime "github.com/baron-chain/cometbft-bc/types/time"
)

func TestGenesisValidation(t *testing.T) {
    testCases := []struct {
        name    string
        genDoc  []byte
        wantErr bool
    }{
        {
            name:    "empty genesis",
            genDoc:  []byte{},
            wantErr: true,
        },
        {
            name:    "invalid json",
            genDoc:  []byte(`{"chain_id":`),
            wantErr: true,
        },
        {
            name: "missing quantum keys when PQC enabled",
            genDoc: []byte(`{
                "chain_id": "test-chain",
                "pqc_enabled": true,
                "validators": [
                    {
                        "pub_key": {"type":"tendermint/PubKeyEd25519","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},
                        "power": "10",
                        "name": "validator1"
                    }
                ]
            }`),
            wantErr: true,
        },
        {
            name: "valid genesis with quantum keys",
            genDoc: []byte(`{
                "chain_id": "test-chain",
                "pqc_enabled": true,
                "validators": [
                    {
                        "pub_key": {"type":"tendermint/PubKeyEd25519","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="},
                        "pqc_pub_key": "dummy_quantum_key",
                        "power": "10",
                        "name": "validator1",
                        "ai_score": 0.95
                    }
                ]
            }`),
            wantErr: false,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := GenesisDocFromJSON(tc.genDoc)
            if tc.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestQuantumSafeGenesis(t *testing.T) {
    privKey, pubKey := kyber.GenerateKeypair()
    validatorKey := ed25519.GenPrivKey().PubKey()

    genDoc := &GenesisDoc{
        GenesisTime:   bctime.Now(),
        ChainID:       "quantum-test-chain",
        InitialHeight: 1,
        PQCEnabled:    true,
        AIValidation:  true,
        Validators: []GenesisValidator{
            {
                Address:      validatorKey.Address(),
                PubKey:      validatorKey,
                Power:       10,
                Name:        "quantum-validator",
                PQCPublicKey: pubKey,
                AIScore:     0.95,
            },
        },
        ConsensusParams: DefaultConsensusParams(),
    }

    t.Run("validates quantum keys", func(t *testing.T) {
        err := genDoc.ValidateAndComplete()
        assert.NoError(t, err)

        // Verify quantum signature
        msg := []byte("test message")
        sig, err := kyber.Sign(privKey, msg)
        require.NoError(t, err)
        
        valid := kyber.Verify(pubKey, msg, sig)
        assert.True(t, valid)
    })

    t.Run("validates AI scores", func(t *testing.T) {
        assert.Equal(t, 0.95, genDoc.Validators[0].AIScore)
    })
}

func TestGenesisSaveAndLoad(t *testing.T) {
    tmpfile, err := os.CreateTemp("", "genesis")
    require.NoError(t, err)
    defer os.Remove(tmpfile.Name())

    genDoc := generateTestGenesisDoc(t)

    err = genDoc.SaveAs(tmpfile.Name())
    require.NoError(t, err)

    loaded, err := GenesisDocFromFile(tmpfile.Name())
    require.NoError(t, err)

    assert.Equal(t, genDoc.ChainID, loaded.ChainID)
    assert.Equal(t, genDoc.PQCEnabled, loaded.PQCEnabled)
    assert.Equal(t, genDoc.AIValidation, loaded.AIValidation)
    assert.Equal(t, len(genDoc.Validators), len(loaded.Validators))

    for i, val := range genDoc.Validators {
        assert.Equal(t, val.Power, loaded.Validators[i].Power)
        assert.Equal(t, val.PQCPublicKey, loaded.Validators[i].PQCPublicKey)
        assert.Equal(t, val.AIScore, loaded.Validators[i].AIScore)
    }
}

func generateTestGenesisDoc(t *testing.T) *GenesisDoc {
    _, pubKey := kyber.GenerateKeypair()
    validatorKey := ed25519.GenPrivKey().PubKey()

    return &GenesisDoc{
        GenesisTime:   bctime.Now(),
        ChainID:       "test-chain",
        InitialHeight: 1000,
        PQCEnabled:    true,
        AIValidation:  true,
        Validators: []GenesisValidator{
            {
                Address:      validatorKey.Address(),
                PubKey:      validatorKey,
                Power:       10,
                Name:        "test-validator",
                PQCPublicKey: pubKey,
                AIScore:     0.95,
            },
        },
        ConsensusParams: DefaultConsensusParams(),
        SidechainConfig: json.RawMessage(`{"enabled":true}`),
    }
}

func TestSidechainConfig(t *testing.T) {
    genDoc := generateTestGenesisDoc(t)
    
    var config struct {
        Enabled bool `json:"enabled"`
    }
    
    err := json.Unmarshal(genDoc.SidechainConfig, &config)
    require.NoError(t, err)
    assert.True(t, config.Enabled)
}
