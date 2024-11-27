package types

import (
    "math"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/baron-chain/cometbft-bc/crypto"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    bcversion "github.com/baron-chain/cometbft-bc/proto/tendermint/version"
)

func TestLightBlockValidateBasic(t *testing.T) {
    header := makeRandHeader()
    commit := randCommit(time.Now())
    vals, _ := RandValidatorSet(5, 1)
    setupHeaderAndCommit(header, commit, vals)

    // Generate quantum keys for testing
    privKey, pubKey := kyber.GenerateKeypair()
    pqcSig, _ := kyber.Sign(privKey, header.Hash())

    sh := &SignedHeader{
        Header: &header,
        Commit: commit,
    }

    testCases := []struct {
        name      string
        lb        LightBlock
        expectErr bool
    }{
        {
            name: "valid light block with quantum signature",
            lb: LightBlock{
                SignedHeader:  sh,
                ValidatorSet:  vals,
                PQCSignature:  pqcSig,
                AITrustScore: 0.95,
            },
            expectErr: false,
        },
        {
            name: "invalid quantum signature",
            lb: LightBlock{
                SignedHeader:  sh,
                ValidatorSet:  vals,
                PQCSignature:  []byte("invalid"),
                AITrustScore: 0.95,
            },
            expectErr: true,
        },
        {
            name: "missing validator set",
            lb: LightBlock{
                SignedHeader: sh,
                PQCSignature: pqcSig,
            },
            expectErr: true,
        },
        {
            name: "invalid AI trust score",
            lb: LightBlock{
                SignedHeader:  sh,
                ValidatorSet:  vals,
                PQCSignature:  pqcSig,
                AITrustScore: 1.5, // Invalid score > 1.0
            },
            expectErr: true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.lb.ValidateBasic(header.ChainID)
            if tc.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestLightBlockProtobuf(t *testing.T) {
    header := makeRandHeader()
    commit := randCommit(time.Now())
    vals, _ := RandValidatorSet(5, 1)
    setupHeaderAndCommit(header, commit, vals)

    privKey, _ := kyber.GenerateKeypair()
    pqcSig, _ := kyber.Sign(privKey, header.Hash())

    sh := &SignedHeader{
        Header: &header,
        Commit: commit,
    }

    testCases := []struct {
        name       string
        lb         *LightBlock
        toProtoErr bool
        fromProtoErr bool
    }{
        {
            name: "valid light block with PQC",
            lb: &LightBlock{
                SignedHeader:  sh,
                ValidatorSet:  vals,
                PQCSignature:  pqcSig,
                AITrustScore: 0.95,
            },
            toProtoErr:   false,
            fromProtoErr: false,
        },
        {
            name: "empty light block",
            lb: &LightBlock{},
            toProtoErr:   false,
            fromProtoErr: true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test conversion to protobuf
            pb, err := tc.lb.ToProto()
            if tc.toProtoErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)

            // Test conversion from protobuf
            lb2, err := LightBlockFromProto(pb)
            if tc.fromProtoErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)

            // Verify all fields match
            assert.Equal(t, tc.lb.SignedHeader, lb2.SignedHeader)
            assert.Equal(t, tc.lb.ValidatorSet, lb2.ValidatorSet)
            assert.Equal(t, tc.lb.PQCSignature, lb2.PQCSignature)
            assert.Equal(t, tc.lb.AITrustScore, lb2.AITrustScore)
        })
    }
}

func setupHeaderAndCommit(header *Header, commit *Commit, vals *ValidatorSet) {
    header.Height = commit.Height
    header.LastBlockID = commit.BlockID
    header.ValidatorsHash = vals.Hash()
    header.Version.Block = bcversion.BlockProtocol
    commit.BlockID.Hash = header.Hash()
}

func TestSignedHeaderWithPQC(t *testing.T) {
    commit := randCommit(time.Now())
    chainID := "baron-test-chain"
    timestamp := time.Now()
    
    header := createTestHeader(chainID, commit, timestamp)
    privKey, pubKey := kyber.GenerateKeypair()
    pqcSig, _ := kyber.Sign(privKey, header.Hash())

    testCases := []struct {
        name      string
        sh        SignedHeader
        pqcSig    []byte
        expectErr bool
    }{
        {
            name: "valid signed header with quantum signature",
            sh: SignedHeader{
                Header: &header,
                Commit: commit,
            },
            pqcSig:    pqcSig,
            expectErr: false,
        },
        {
            name: "invalid quantum signature",
            sh: SignedHeader{
                Header: &header,
                Commit: commit,
            },
            pqcSig:    []byte("invalid"),
            expectErr: true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.sh.ValidateBasic(chainID)
            if tc.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }

            if len(tc.pqcSig) > 0 {
                valid := kyber.Verify(pubKey, tc.sh.Header.Hash(), tc.pqcSig)
                assert.Equal(t, !tc.expectErr, valid)
            }
        })
    }
}

func createTestHeader(chainID string, commit *Commit, timestamp time.Time) Header {
    return Header{
        Version:            bcversion.Consensus{Block: bcversion.BlockProtocol, App: 1},
        ChainID:            chainID,
        Height:             commit.Height,
        Time:               timestamp,
        LastBlockID:        commit.BlockID,
        LastCommitHash:     commit.Hash(),
        DataHash:           commit.Hash(),
        ValidatorsHash:     commit.Hash(),
        NextValidatorsHash: commit.Hash(),
        ConsensusHash:      commit.Hash(),
        AppHash:            commit.Hash(),
        LastResultsHash:    commit.Hash(),
        EvidenceHash:       commit.Hash(),
        ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
    }
}
