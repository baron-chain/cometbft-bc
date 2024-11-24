package types

import (
    "errors"
    "fmt"
    "sync"
    
    bcjson "github.com/baron-chain/cometbft-bc/libs/json"
    bcmath "github.com/baron-chain/cometbft-bc/libs/math"
    "github.com/baron-chain/cometbft-bc/p2p"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
    "github.com/baron-chain/cometbft-bc/types"
)

var ErrUnwantedRoundVote = errors.New("peer sent vote from unwanted round")

type RoundVoteSet struct {
    Prevotes   *types.VoteSet
    Precommits *types.VoteSet
}

type HeightVoteSet struct {
    mu sync.RWMutex
    
    chainID           string
    height            int64
    round             int32
    valSet           *types.ValidatorSet
    roundVoteSets     map[int32]RoundVoteSet
    peerCatchupRounds map[p2p.ID][]int32
}

func NewHeightVoteSet(chainID string, height int64, valSet *types.ValidatorSet) *HeightVoteSet {
    hvs := &HeightVoteSet{
        chainID:           chainID,
        roundVoteSets:     make(map[int32]RoundVoteSet),
        peerCatchupRounds: make(map[p2p.ID][]int32),
    }
    hvs.Reset(height, valSet)
    return hvs
}

func (hvs *HeightVoteSet) Reset(height int64, valSet *types.ValidatorSet) {
    hvs.mu.Lock()
    defer hvs.mu.Unlock()

    hvs.height = height
    hvs.valSet = valSet
    hvs.roundVoteSets = make(map[int32]RoundVoteSet)
    hvs.peerCatchupRounds = make(map[p2p.ID][]int32)
    hvs.round = 0
    
    hvs.addRound(0)
}

func (hvs *HeightVoteSet) Height() int64 {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock()
    return hvs.height
}

func (hvs *HeightVoteSet) Round() int32 {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock()
    return hvs.round
}

func (hvs *HeightVoteSet) SetRound(round int32) {
    hvs.mu.Lock()
    defer hvs.mu.Unlock()

    if round < bcmath.SafeSubInt32(hvs.round, 1) && hvs.round != 0 {
        panic("SetRound must increment round")
    }

    for r := bcmath.SafeSubInt32(hvs.round, 1); r <= round; r++ {
        if _, exists := hvs.roundVoteSets[r]; !exists {
            hvs.addRound(r)
        }
    }
    hvs.round = round
}

func (hvs *HeightVoteSet) addRound(round int32) {
    if _, exists := hvs.roundVoteSets[round]; exists {
        panic("round already exists")
    }

    hvs.roundVoteSets[round] = RoundVoteSet{
        Prevotes:   types.NewVoteSet(hvs.chainID, hvs.height, round, bcproto.PrevoteType, hvs.valSet),
        Precommits: types.NewVoteSet(hvs.chainID, hvs.height, round, bcproto.PrecommitType, hvs.valSet),
    }
}

func (hvs *HeightVoteSet) AddVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
    hvs.mu.Lock()
    defer hvs.mu.Unlock()

    if !types.IsVoteTypeValid(vote.Type) {
        return false, nil
    }

    voteSet := hvs.getVoteSet(vote.Round, vote.Type)
    if voteSet == nil {
        rounds := hvs.peerCatchupRounds[peerID]
        if len(rounds) < 2 {
            hvs.addRound(vote.Round)
            voteSet = hvs.getVoteSet(vote.Round, vote.Type)
            hvs.peerCatchupRounds[peerID] = append(rounds, vote.Round)
        } else {
            return false, ErrUnwantedRoundVote
        }
    }
    
    return voteSet.AddVote(vote)
}

func (hvs *HeightVoteSet) Prevotes(round int32) *types.VoteSet {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock()
    return hvs.getVoteSet(round, bcproto.PrevoteType)
}

func (hvs *HeightVoteSet) Precommits(round int32) *types.VoteSet {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock() 
    return hvs.getVoteSet(round, bcproto.PrecommitType)
}

func (hvs *HeightVoteSet) POLInfo() (polRound int32, polBlockID types.BlockID) {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock()

    for r := hvs.round; r >= 0; r-- {
        if voteSet := hvs.getVoteSet(r, bcproto.PrevoteType); voteSet != nil {
            if blockID, ok := voteSet.TwoThirdsMajority(); ok {
                return r, blockID
            }
        }
    }
    return -1, types.BlockID{}
}

func (hvs *HeightVoteSet) getVoteSet(round int32, voteType bcproto.SignedMsgType) *types.VoteSet {
    rvs, ok := hvs.roundVoteSets[round]
    if !ok {
        return nil
    }

    switch voteType {
    case bcproto.PrevoteType:
        return rvs.Prevotes
    case bcproto.PrecommitType:
        return rvs.Precommits
    default:
        panic(fmt.Sprintf("invalid vote type %X", voteType))
    }
}

func (hvs *HeightVoteSet) SetPeerMaj23(round int32, voteType bcproto.SignedMsgType, peerID p2p.ID, blockID types.BlockID) error {
    hvs.mu.Lock()
    defer hvs.mu.Unlock()

    if !types.IsVoteTypeValid(voteType) {
        return fmt.Errorf("invalid vote type %X", voteType)
    }

    voteSet := hvs.getVoteSet(round, voteType)
    if voteSet == nil {
        return nil
    }

    return voteSet.SetPeerMaj23(types.P2PID(peerID), blockID)
}

func (hvs *HeightVoteSet) MarshalJSON() ([]byte, error) {
    hvs.mu.RLock()
    defer hvs.mu.RUnlock()
    return bcjson.Marshal(hvs.toAllRoundVotes())
}
