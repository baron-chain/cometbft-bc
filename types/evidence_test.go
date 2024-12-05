package types
//BaronChain
import (
    "math"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/baron-chain/cometbft-bc/crypto"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    "github.com/baron-chain/cometbft-bc/crypto/tmhash"
    bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
    bcversion "github.com/baron-chain/cometbft-bc/proto/tendermint/version"
)

const (
    defaultChainID = "baron-chain-test"
    testVersion    = 1
)

var defaultTestTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvidenceList(t *testing.T) {
    t.Run("basic evidence operations", func(t *testing.T) {
        ev := generateTestEvidence(t)
        evl := EvidenceList([]Evidence{ev})

        assert.NotNil(t, evl.Hash())
        assert.True(t, evl.Has(ev))
        assert.False(t, evl.Has(&DuplicateVoteEvidence{}))
    })

    t.Run("quantum-safe evidence validation", func(t *testing.T) {
        ev := generateQuantumSafeEvidence(t)
        evl := EvidenceList([]Evidence{ev})

        err := evl.ValidateBasic()
        assert.NoError(t, err, "quantum-safe evidence should validate")
        assert.True(t, verifyPQCSignature(ev))
    })
}

func generateTestEvidence(t *testing.T) *DuplicateVoteEvidence {
    val := NewMockPV()
    blockID1 := makeBlockID([]byte("block1"), 1000, []byte("parts1"))
    blockID2 := makeBlockID([]byte("block2"), 1000, []byte("parts2"))

    return &DuplicateVoteEvidence{
        VoteA:            generateTestVote(t, val, defaultChainID, 0, 10, 2, blockID1, defaultTestTime),
        VoteB:            generateTestVote(t, val, defaultChainID, 0, 10, 2, blockID2, defaultTestTime.Add(time.Minute)),
        TotalVotingPower: 30,
        ValidatorPower:   10,
        Timestamp:        defaultTestTime,
    }
}

func generateQuantumSafeEvidence(t *testing.T) *DuplicateVoteEvidence {
    ev := generateTestEvidence(t)
    
    // Add quantum signature
    privKey, pubKey := kyber.GenerateKeypair()
    data := append(ev.VoteA.Bytes(), ev.VoteB.Bytes()...)
    sig, err := kyber.Sign(privKey, data)
    require.NoError(t, err)
    
    ev.PQCSignature = sig
    ev.PQCPubKey = pubKey
    
    return ev
}

func TestDuplicateVoteEvidenceValidation(t *testing.T) {
    testCases := []struct {
        name          string
        malleate     func(*DuplicateVoteEvidence)
        expectErr    bool
    }{
        {
            name:      "valid evidence",
            malleate:  func(ev *DuplicateVoteEvidence) {},
            expectErr: false,
        },
        {
            name:      "missing vote A",
            malleate:  func(ev *DuplicateVoteEvidence) { ev.VoteA = nil },
            expectErr: true,
        },
        {
            name:      "missing vote B",
            malleate:  func(ev *DuplicateVoteEvidence) { ev.VoteB = nil },
            expectErr: true,
        },
        {
            name:      "invalid PQC signature",
            malleate:  func(ev *DuplicateVoteEvidence) { ev.PQCSignature = []byte("invalid") },
            expectErr: true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            ev := generateQuantumSafeEvidence(t)
            tc.malleate(ev)
            err := ev.ValidateBasic()
            if tc.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func generateTestVote(t *testing.T, val PrivValidator, chainID string, valIndex int32, 
    height int64, round int32, blockID BlockID, time time.Time) *Vote {
    
    pubKey, err := val.GetPubKey()
    require.NoError(t, err)

    vote := &Vote{
        ValidatorAddress: pubKey.Address(),
        ValidatorIndex:   valIndex,
        Height:          height,
        Round:           round,
        Type:            bcproto.PrecommitType,
        BlockID:         blockID,
        Timestamp:       time,
    }

    vpb := vote.ToProto()
    err = val.SignVote(chainID, vpb)
    require.NoError(t, err)
    vote.Signature = vpb.Signature

    return vote
}

func verifyPQCSignature(ev *DuplicateVoteEvidence) bool {
    data := append(ev.VoteA.Bytes(), ev.VoteB.Bytes()...)
    return kyber.Verify(ev.PQCPubKey, data, ev.PQCSignature)
}

// Helper functions
func makeRandomHeader() *Header {
    return &Header{
        Version: bcversion.Consensus{
            Block: testVersion,
            App:   testVersion,
        },
        ChainID:            defaultChainID,
        Height:             int64(bcrand.Uint16()) + 1,
        Time:              defaultTestTime,
        LastBlockID:        makeRandomBlockID(),
        LastCommitHash:     tmhash.Sum(bcrand.Bytes(32)),
        DataHash:           tmhash.Sum(bcrand.Bytes(32)),
        ValidatorsHash:     tmhash.Sum(bcrand.Bytes(32)),
        NextValidatorsHash: tmhash.Sum(bcrand.Bytes(32)),
        ConsensusHash:      tmhash.Sum(bcrand.Bytes(32)),
        AppHash:            tmhash.Sum(bcrand.Bytes(32)),
        LastResultsHash:    tmhash.Sum(bcrand.Bytes(32)),
        EvidenceHash:       tmhash.Sum(bcrand.Bytes(32)),
        ProposerAddress:    bcrand.Bytes(crypto.AddressSize),
    }
}
