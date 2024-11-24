package types

import (
    "fmt"
    "os"
    "testing"

    cfg "github.com/baron-chain/cometbft-bc/config"
    "github.com/baron-chain/cometbft-bc/crypto/tmhash"
    bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
    "github.com/baron-chain/cometbft-bc/types"
    bctime "github.com/baron-chain/cometbft-bc/types/time"
)

var (
    testConfig *cfg.Config
    testChainID string = "baron_test_chain"
)

func TestMain(m *testing.M) {
    testConfig = cfg.ResetTestRoot("baron_consensus_vote_test")
    exitCode := m.Run()
    cleanup()
    os.Exit(exitCode)
}

func cleanup() {
    if testConfig.RootDir != "" {
        os.RemoveAll(testConfig.RootDir)
    }
}

func TestPeerCatchupRounds(t *testing.T) {
    const (
        numValidators = 10
        votingPower   = 1
        height        = int64(1)
        peerID1       = "peer1"
        peerID2       = "peer2"
    )

    testCases := []struct {
        name        string
        round       int32
        peerID      string
        expectAdd   bool
        expectError error
    }{
        {
            name:        "first catchup round",
            round:       999,
            peerID:      peerID1,
            expectAdd:   true,
            expectError: nil,
        },
        {
            name:        "second catchup round",
            round:       1000,
            peerID:      peerID1,
            expectAdd:   true,
            expectError: nil,
        },
        {
            name:        "exceed catchup limit",
            round:       1001,
            peerID:      peerID1,
            expectAdd:   false,
            expectError: ErrUnwantedRoundVote,
        },
        {
            name:        "different peer valid round",
            round:       1001,
            peerID:      peerID2,
            expectAdd:   true,
            expectError: nil,
        },
    }

    valSet, privVals := types.RandValidatorSet(numValidators, votingPower)
    hvs := NewHeightVoteSet(testChainID, height, valSet)

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            vote := generateTestVote(t, height, 0, tc.round, privVals)
            added, err := hvs.AddVote(vote, tc.peerID)

            if added != tc.expectAdd {
                t.Errorf("expected AddVote to return %v, got %v", tc.expectAdd, added)
            }
            
            if err != tc.expectError {
                t.Errorf("expected error %v, got %v", tc.expectError, err)
            }
        })
    }
}

func generateTestVote(t *testing.T, height int64, valIndex, round int32, privVals []types.PrivValidator) *types.Vote {
    t.Helper()
    
    privVal := privVals[valIndex]
    pubKey, err := privVal.GetPubKey()
    if err != nil {
        t.Fatalf("failed to get pubkey: %v", err)
    }

    vote := &types.Vote{
        ValidatorAddress: pubKey.Address(),
        ValidatorIndex:   valIndex,
        Height:          height,
        Round:           round,
        Timestamp:       bctime.Now(),
        Type:            bcproto.PrecommitType,
        BlockID: types.BlockID{
            Hash:           bcrand.Bytes(tmhash.Size),
            PartSetHeader: types.PartSetHeader{},
        },
    }

    v := vote.ToProto()
    if err := privVal.SignVote(testChainID, v); err != nil {
        t.Fatalf("failed to sign vote: %v", err)
    }
    
    vote.Signature = v.Signature
    return vote
}
