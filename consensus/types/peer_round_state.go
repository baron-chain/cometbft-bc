package types

import (
    "fmt"
    "time"
    
    "github.com/baron-chain/cometbft-bc/libs/bits"
    "github.com/baron-chain/cometbft-bc/types"
)

// PeerRoundState represents a peer's known state in the consensus protocol
type PeerRoundState struct {
    Height                     int64          `json:"height"`
    Round                      int32          `json:"round"`
    Step                       RoundStepType  `json:"step"`
    StartTime                  time.Time      `json:"start_time"`
    
    // Proposal state
    Proposal                   bool           `json:"proposal"`
    ProposalBlockPartSetHeader types.PartSetHeader `json:"proposal_block_part_set_header"`
    ProposalBlockParts        *bits.BitArray  `json:"proposal_block_parts"`
    ProposalPOLRound          int32          `json:"proposal_pol_round"`
    ProposalPOL              *bits.BitArray  `json:"proposal_pol"`
    
    // Voting state
    Prevotes                  *bits.BitArray  `json:"prevotes"`
    Precommits                *bits.BitArray  `json:"precommits"`
    
    // Commit state
    LastCommitRound           int32          `json:"last_commit_round"`
    LastCommit                *bits.BitArray  `json:"last_commit"`
    CatchupCommitRound        int32          `json:"catchup_commit_round"`
    CatchupCommit             *bits.BitArray  `json:"catchup_commit"`
}

const (
    defaultIndent = ""
    stringFormat = `PeerRoundState{
%s  Height/Round/Step: %d/%d/%v @%v
%s  Proposal: %v -> %v
%s  POL: %v (round %d)
%s  Votes: prevotes=%v precommits=%v
%s  Last Commit: %v (round %d)
%s  Catchup: %v (round %d)
%s}`
)

// String returns a string representation of PeerRoundState
func (prs PeerRoundState) String() string {
    return prs.StringIndented(defaultIndent)
}

// StringIndented returns an indented string representation of PeerRoundState
func (prs PeerRoundState) StringIndented(indent string) string {
    return fmt.Sprintf(stringFormat,
        indent, prs.Height, prs.Round, prs.Step, prs.StartTime.Format(time.RFC3339),
        indent, prs.ProposalBlockPartSetHeader, prs.ProposalBlockParts,
        indent, prs.ProposalPOL, prs.ProposalPOLRound,
        indent, prs.Prevotes, prs.Precommits,
        indent, prs.LastCommit, prs.LastCommitRound,
        indent, prs.CatchupCommit, prs.CatchupCommitRound,
        indent,
    )
}

// NewPeerRoundState creates a new PeerRoundState instance
func NewPeerRoundState(height int64, round int32) *PeerRoundState {
    return &PeerRoundState{
        Height:              height,
        Round:              round,
        Step:               RoundStepPropose,
        StartTime:          time.Now().UTC(),
        ProposalPOLRound:   -1,
        LastCommitRound:    -1,
        CatchupCommitRound: -1,
        Prevotes:          bits.NewBitArray(0),
        Precommits:        bits.NewBitArray(0),
        LastCommit:        bits.NewBitArray(0),
        CatchupCommit:     bits.NewBitArray(0),
    }
}

// IsValid performs basic validation of the peer round state
func (prs PeerRoundState) IsValid() bool {
    return prs.Height > 0 && 
           prs.Round >= -1 && 
           prs.ProposalPOLRound >= -1 &&
           prs.LastCommitRound >= -1 &&
           prs.CatchupCommitRound >= -1
}

// HasProposal checks if the peer has a complete proposal
func (prs PeerRoundState) HasProposal() bool {
    return prs.Proposal && 
           prs.ProposalBlockParts != nil && 
           prs.ProposalBlockParts.All()
}

// HasPrevoteQuorum checks if the peer has +2/3 prevotes
func (prs PeerRoundState) HasPrevoteQuorum() bool {
    return prs.Prevotes != nil && 
           prs.Prevotes.HasTwoThirdsMajority()
}

// HasPrecommitQuorum checks if the peer has +2/3 precommits
func (prs PeerRoundState) HasPrecommitQuorum() bool {
    return prs.Precommits != nil && 
           prs.Precommits.HasTwoThirdsMajority()
}
