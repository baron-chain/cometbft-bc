package types

import (
    "bytes"
    "sort"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
)

var (
    valEd25519   = []string{PubKeyTypeEd25519}
    valSecp256k1 = []string{PubKeyTypeSecp256k1}
    valKyber     = []string{PubKeyTypeKyber}
    valHybrid    = []string{PubKeyTypeEd25519, PubKeyTypeKyber}
)

func TestConsensusParamsValidation(t *testing.T) {
    testCases := []struct {
        name   string
        params ConsensusParams
        valid  bool
    }{
        {
            name:   "valid baseline params",
            params: makeParams(1, 0, 2, 0, valEd25519, true, true),
            valid:  true,
        },
        {
            name:   "valid quantum-safe params",
            params: makeParamsWithQuantum(1, 0, 2, 0, valKyber, 256),
            valid:  true,
        },
        {
            name:   "invalid quantum key size",
            params: makeParamsWithQuantum(1, 0, 2, 0, valKyber, 128),
            valid:  false,
        },
        {
            name:   "valid hybrid keys",
            params: makeParamsWithQuantum(1, 0, 2, 0, valHybrid, 256),
            valid:  true,
        },
        {
            name:   "invalid AI trust score",
            params: makeParamsWithAI(1, 0, 2, 0, valEd25519, 1.5),
            valid:  false,
        },
        {
            name:   "zero block size",
            params: makeParams(0, 0, 2, 0, valEd25519, true, true),
            valid:  false,
        },
        {
            name:   "block too large",
            params: makeParams(101*1024*1024, 0, 2, 0, valEd25519, true, true),
            valid:  false,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.params.ValidateBasic()
            if tc.valid {
                assert.NoError(t, err)
            } else {
                assert.Error(t, err)
            }
        })
    }
}

func makeParams(
    blockBytes, blockGas int64,
    evidenceAge int64,
    maxEvidenceBytes int64,
    pubkeyTypes []string,
    enablePQC bool,
    enableAI bool,
) ConsensusParams {
    return ConsensusParams{
        Block: BlockParams{
            MaxBytes:        blockBytes,
            MaxGas:         blockGas,
            MaxTransactions: 10000,
        },
        Evidence: EvidenceParams{
            MaxAgeNumBlocks: evidenceAge,
            MaxAgeDuration:  time.Duration(evidenceAge),
            MaxBytes:        maxEvidenceBytes,
        },
        Validator: ValidatorParams{
            PubKeyTypes: pubkeyTypes,
        },
        QuantumSafe: QuantumParams{
            Enabled:          enablePQC,
            MinKeySize:       256,
            RequiredKeyTypes: []string{PubKeyTypeKyber},
        },
        AI: AIParams{
            Enabled:            enableAI,
            MinTrustScore:      0.7,
            ValidationInterval: 100,
        },
    }
}

func makeParamsWithQuantum(
    blockBytes, blockGas int64,
    evidenceAge int64,
    maxEvidenceBytes int64,
    pubkeyTypes []string,
    minKeySize int,
) ConsensusParams {
    params := makeParams(blockBytes, blockGas, evidenceAge, maxEvidenceBytes, 
        pubkeyTypes, true, true)
    params.QuantumSafe.MinKeySize = minKeySize
    return params
}

func makeParamsWithAI(
    blockBytes, blockGas int64,
    evidenceAge int64,
    maxEvidenceBytes int64,
    pubkeyTypes []string,
    minTrustScore float64,
) ConsensusParams {
    params := makeParams(blockBytes, blockGas, evidenceAge, maxEvidenceBytes, 
        pubkeyTypes, true, true)
    params.AI.MinTrustScore = minTrustScore
    return params
}

func TestConsensusParamsHash(t *testing.T) {
    params := []ConsensusParams{
        makeParams(4, 2, 3, 1, valEd25519, true, true),
        makeParams(1, 4, 3, 1, valKyber, true, true),
        makeParamsWithQuantum(1, 2, 4, 1, valHybrid, 256),
        makeParamsWithAI(2, 5, 7, 1, valEd25519, 0.8),
    }

    hashes := make([][]byte, len(params))
    for i := range params {
        hashes[i] = params[i].Hash()
    }

    sort.Slice(hashes, func(i, j int) bool {
        return bytes.Compare(hashes[i], hashes[j]) < 0
    })

    for i := 0; i < len(hashes)-1; i++ {
        assert.NotEqual(t, hashes[i], hashes[i+1], 
            "params should produce unique hashes")
    }
}

func TestConsensusParamsUpdate(t *testing.T) {
    testCases := []struct {
        name          string
        params        ConsensusParams
        updates       *bcproto.ConsensusParams
        updatedParams ConsensusParams
    }{
        {
            name:          "update quantum params",
            params:        makeParams(1, 2, 3, 0, valEd25519, true, true),
            updates: &bcproto.ConsensusParams{
                Block: &bcproto.BlockParams{
                    MaxBytes: 100,
                    MaxGas:   200,
                },
                QuantumSafe: &bcproto.QuantumParams{
                    Enabled:    true,
                    MinKeySize: 512,
                },
            },
            updatedParams: makeParamsWithQuantum(100, 200, 3, 0, valEd25519, 512),
        },
        {
            name:          "update AI params",
            params:        makeParams(1, 2, 3, 0, valEd25519, true, true),
            updates: &bcproto.ConsensusParams{
                AI: &bcproto.AIParams{
                    MinTrustScore: 0.9,
                },
            },
            updatedParams: makeParamsWithAI(1, 2, 3, 0, valEd25519, 0.9),
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            updated := tc.params.Update(tc.updates)
            assert.Equal(t, tc.updatedParams, updated)
        })
    }
}

func TestProtoConversion(t *testing.T) {
    params := []ConsensusParams{
        makeParams(4, 2, 3, 1, valEd25519, true, true),
        makeParamsWithQuantum(1, 4, 3, 1, valKyber, 256),
        makeParamsWithAI(1, 2, 4, 1, valEd25519, 0.8),
    }

    for _, param := range params {
        pbParams := param.ToProto()
        restored := ConsensusParamsFromProto(pbParams)
        assert.Equal(t, param, restored, "params should survive proto conversion")
    }
}
