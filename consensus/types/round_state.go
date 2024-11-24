package types

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/baron-chain/cometbft-bc/libs/bytes"
    "github.com/baron-chain/cometbft-bc/types"
)

type RoundStepType uint8

const (
    RoundStepNewHeight     = RoundStepType(1)
    RoundStepNewRound      = RoundStepType(2)
    RoundStepPropose       = RoundStepType(3)
    RoundStepPrevote       = RoundStepType(4)
    RoundStepPrevoteWait   = RoundStepType(5)
    RoundStepPrecommit     = RoundStepType(6)
    RoundStepPrecommitWait = RoundStepType(7)
    RoundStepCommit        = RoundStepType(8)

    maxRoundStep           = RoundStepCommit
)

func (rs RoundStepType) IsValid() bool {
    return rs > 0 && rs <= maxRoundStep
}

var roundStepStrings = map[RoundStepType]string{
    RoundStepNewHeight:     "RoundStepNewHeight",
    RoundStepNewRound:      "RoundStepNewRound", 
    RoundStepPropose:       "RoundStepPropose",
    RoundStepPrevote:       "RoundStepPrevote",
    RoundStepPrevoteWait:   "RoundStepPrevoteWait",
    RoundStepPrecommit:     "RoundStepPrecommit",
    RoundStepPrecommitWait: "RoundStepPrecommitWait",
    RoundStepCommit:        "RoundStepCommit",
}

func (rs RoundStepType) String() string {
    if s, ok := roundStepStrings[rs]; ok {
        return s
    }
    return "RoundStepUnknown"
}

type RoundState struct {
    Height    int64         `json:"height"`
    Round     int32         `json:"round"`
    Step      RoundStepType `json:"step"`
    StartTime time.Time     `json:"start_time"`

    CommitTime              time.Time           `json:"commit_time"`
    Validators             *types.ValidatorSet  `json:"validators"`
    Proposal               *types.Proposal      `json:"proposal"`
    ProposalBlock         *types.Block         `json:"proposal_block"`
    ProposalBlockParts    *types.PartSet       `json:"proposal_block_parts"`
    
    LockedRound            int32               `json:"locked_round"`
    LockedBlock           *types.Block         `json:"locked_block"`
    LockedBlockParts      *types.PartSet       `json:"locked_block_parts"`

    ValidRound             int32               `json:"valid_round"`
    ValidBlock            *types.Block         `json:"valid_block"`
    ValidBlockParts       *types.PartSet       `json:"valid_block_parts"`
    
    Votes                 *HeightVoteSet       `json:"votes"`
    CommitRound            int32               `json:"commit_round"`
    LastCommit            *types.VoteSet       `json:"last_commit"`
    LastValidators        *types.ValidatorSet  `json:"last_validators"`
    
    TriggeredTimeoutPrecommit bool             `json:"triggered_timeout_precommit"`
}

type RoundStateSimple struct {
    HeightRoundStep   string              `json:"height/round/step"`
    StartTime         time.Time           `json:"start_time"`
    ProposalBlockHash bytes.HexBytes      `json:"proposal_block_hash"`
    LockedBlockHash   bytes.HexBytes      `json:"locked_block_hash"`
    ValidBlockHash    bytes.HexBytes      `json:"valid_block_hash"`
    Votes             json.RawMessage     `json:"height_vote_set"`
    Proposer          types.ValidatorInfo `json:"proposer"`
}

func (rs *RoundState) ToSimple() (RoundStateSimple, error) {
    votesJSON, err := rs.Votes.MarshalJSON()
    if err != nil {
        return RoundStateSimple{}, fmt.Errorf("failed to marshal votes: %w", err)
    }

    proposer := rs.Validators.GetProposer()
    idx, _ := rs.Validators.GetByAddress(proposer.Address)

    return RoundStateSimple{
        HeightRoundStep:   fmt.Sprintf("%d/%d/%d", rs.Height, rs.Round, rs.Step),
        StartTime:         rs.StartTime,
        ProposalBlockHash: rs.ProposalBlock.Hash(),
        LockedBlockHash:   rs.LockedBlock.Hash(),
        ValidBlockHash:    rs.ValidBlock.Hash(),
        Votes:             votesJSON,
        Proposer: types.ValidatorInfo{
            Address: proposer.Address,
            Index:   idx,
        },
    }, nil
}

func (rs *RoundState) NewRoundEvent() types.EventDataNewRound {
    proposer := rs.Validators.GetProposer()
    idx, _ := rs.Validators.GetByAddress(proposer.Address)

    return types.EventDataNewRound{
        Height: rs.Height,
        Round:  rs.Round,
        Step:   rs.Step.String(),
        Proposer: types.ValidatorInfo{
            Address: proposer.Address,
            Index:   idx,
        },
    }
}

func (rs *RoundState) CompleteProposalEvent() types.EventDataCompleteProposal {
    blockID := types.BlockID{
        Hash:          rs.ProposalBlock.Hash(),
        PartSetHeader: rs.ProposalBlockParts.Header(),
    }

    return types.EventDataCompleteProposal{
        Height:  rs.Height,
        Round:   rs.Round,
        Step:    rs.Step.String(),
        BlockID: blockID,
    }
}

func (rs *RoundState) RoundStateEvent() types.EventDataRoundState {
    return types.EventDataRoundState{
        Height: rs.Height,
        Round:  rs.Round,
        Step:   rs.Step.String(),
    }
}

func NewRoundState(height int64, validators *types.ValidatorSet) *RoundState {
    return &RoundState{
        Height:      height,
        Round:       0,
        Step:        RoundStepNewHeight,
        StartTime:   time.Now().UTC(),
        Validators:  validators,
        Votes:       NewHeightVoteSet("baron-chain", height, validators),
    }
}

func (rs *RoundState) IsValid() bool {
    return rs.Height > 0 && rs.Step.IsValid() && rs.Validators != nil
}

func (rs *RoundState) String() string {
    return rs.StringIndented("")
}

func (rs *RoundState) StringShort() string {
    return fmt.Sprintf("RoundState{H:%v R:%v S:%v ST:%v}",
        rs.Height, rs.Round, rs.Step, rs.StartTime.Format(time.RFC3339))
}
