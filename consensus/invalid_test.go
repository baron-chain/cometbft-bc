package consensus

import (
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    "github.com/baron-chain/cometbft-bc/libs/bytes"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcrand "github.com/baron-chain/cometbft-bc/libs/rand"
    "github.com/baron-chain/cometbft-bc/p2p"
    bccons "github.com/baron-chain/cometbft-bc/proto/consensus"
    bcproto "github.com/baron-chain/cometbft-bc/proto/types"
    "github.com/baron-chain/cometbft-bc/types"
)

const (
    invalidPrecommitTestValidators = 4
    maxTestBlocks = 10
    byzantineValidatorIndex = 0
)

// TestReactorInvalidPrecommit tests that a Byzantine validator sending invalid
// precommits does not prevent consensus from making progress
func TestReactorInvalidPrecommit(t *testing.T) {
    // Create a network of validators
    css, cleanup := randConsensusNet(
        invalidPrecommitTestValidators, 
        "consensus_invalid_precommit_test",
        newMockTickerFunc(true),
        newKVStore,
    )
    defer cleanup()

    // Initialize timeout tickers
    for i := range css {
        ticker := NewTimeoutTicker()
        ticker.SetLogger(css[i].Logger)
        css[i].SetTimeoutTicker(ticker)
    }

    // Start consensus network
    reactors, blocksSubs, eventBuses := startConsensusNet(t, css, invalidPrecommitTestValidators)
    defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

    // Configure Byzantine validator
    byzVal := css[byzantineValidatorIndex]
    byzReactor := reactors[byzantineValidatorIndex]

    byzVal.mtx.Lock()
    privVal := byzVal.privValidator
    byzVal.doPrevote = func(height int64, round int32) {
        sendInvalidPrecommit(t, height, round, byzVal, byzReactor.Switch, privVal)
    }
    byzVal.mtx.Unlock()

    // Wait for blocks to be produced
    waitForBlocks(t, blocksSubs, css, maxTestBlocks)
}

// sendInvalidPrecommit generates and broadcasts an invalid precommit vote
func sendInvalidPrecommit(
    t *testing.T,
    height int64,
    round int32,
    cs *State,
    sw *p2p.Switch,
    pv types.PrivValidator,
) {
    go func() {
        cs.mtx.Lock()
        defer cs.mtx.Unlock()

        // Get validator info
        cs.privValidator = pv
        pubKey, err := cs.privValidator.GetPubKey()
        require.NoError(t, err)

        addr := pubKey.Address()
        valIndex, _ := cs.Validators.GetByAddress(addr)

        // Create random invalid precommit
        invalidPrecommit := generateInvalidPrecommit(cs, addr, valIndex, height, round)

        // Sign the precommit
        protoVote := invalidPrecommit.ToProto()
        err = cs.privValidator.SignVote(cs.state.ChainID, protoVote)
        require.NoError(t, err)

        invalidPrecommit.Signature = protoVote.Signature
        
        // Disable validator to prevent normal voting
        cs.privValidator = nil

        // Broadcast invalid precommit to peers
        broadcastInvalidPrecommit(cs, sw, invalidPrecommit)
    }()
}

func generateInvalidPrecommit(
    cs *State,
    addr bytes.HexBytes,
    valIndex int32,
    height int64,
    round int32,
) *types.Vote {
    return &types.Vote{
        ValidatorAddress: addr,
        ValidatorIndex:   valIndex,
        Height:          height,
        Round:           round,
        Timestamp:       cs.voteTime(),
        Type:            bcproto.PrecommitType,
        BlockID: types.BlockID{
            Hash:          bcrand.Bytes(32),
            PartSetHeader: types.PartSetHeader{
                Total: 1,
                Hash:  bcrand.Bytes(32),
            },
        },
    }
}

func broadcastInvalidPrecommit(cs *State, sw *p2p.Switch, precommit *types.Vote) {
    peers := sw.Peers().List()
    for _, peer := range peers {
        cs.Logger.Info("Broadcasting invalid precommit",
            "block_hash", precommit.BlockID.Hash,
            "peer", peer)
            
        peer.SendEnvelope(p2p.Envelope{
            Message: &bccons.Vote{
                Vote: precommit.ToProto(),
            },
            ChannelID: VoteChannel,
        })
    }
}

func waitForBlocks(t *testing.T, subs []types.Subscription, css []*State, numBlocks int) {
    for i := 0; i < numBlocks; i++ {
        timeoutWaitGroup(t, len(css), func(j int) {
            select {
            case <-subs[j].Out():
            case <-time.After(30 * time.Second):
                t.Fatal("Timed out waiting for block")
            }
        }, css)
    }
}
