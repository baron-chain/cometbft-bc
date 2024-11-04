package types

import (
	"fmt"
	"time"

	cmtproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
	cmttime "github.com/baron-chain/cometbft-bc/types/time"
)

// TimeFormat defines the canonical time format for use in signatures
const TimeFormat = time.RFC3339Nano

// Canonicalizer is an interface for types that can be converted to their canonical form
type Canonicalizer[T any] interface {
	Canonicalize() T
}

// CanonicalizeBlockID converts a BlockID to its canonical form
// Returns nil if the input represents a zero or nil BlockID
func CanonicalizeBlockID(bid cmtproto.BlockID) (*cmtproto.CanonicalBlockID, error) {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		return nil, fmt.Errorf("failed to convert BlockID from proto: %w", err)
	}

	if rbid == nil || rbid.IsZero() {
		return nil, nil
	}

	return &cmtproto.CanonicalBlockID{
		Hash:          bid.Hash,
		PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
	}, nil
}

// MustCanonicalizeBlockID is like CanonicalizeBlockID but panics on error
func MustCanonicalizeBlockID(bid cmtproto.BlockID) *cmtproto.CanonicalBlockID {
	canonical, err := CanonicalizeBlockID(bid)
	if err != nil {
		panic(err)
	}
	return canonical
}

// CanonicalizePartSetHeader converts a PartSetHeader to its canonical form
func CanonicalizePartSetHeader(psh cmtproto.PartSetHeader) cmtproto.CanonicalPartSetHeader {
	return cmtproto.CanonicalPartSetHeader(psh)
}

// CanonicalizeProposal converts a Proposal to its canonical form for signing
func CanonicalizeProposal(chainID string, proposal *cmtproto.Proposal) (cmtproto.CanonicalProposal, error) {
	blockID, err := CanonicalizeBlockID(proposal.BlockID)
	if err != nil {
		return cmtproto.CanonicalProposal{}, fmt.Errorf("failed to canonicalize BlockID: %w", err)
	}

	return cmtproto.CanonicalProposal{
		Type:      cmtproto.ProposalType,
		Height:    proposal.Height,
		Round:     int64(proposal.Round),
		POLRound:  int64(proposal.PolRound),
		BlockID:   blockID,
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}, nil
}

// MustCanonicalizeProposal is like CanonicalizeProposal but panics on error
func MustCanonicalizeProposal(chainID string, proposal *cmtproto.Proposal) cmtproto.CanonicalProposal {
	canonical, err := CanonicalizeProposal(chainID, proposal)
	if err != nil {
		panic(err)
	}
	return canonical
}

// CanonicalizeVote converts a Vote to its canonical form for signing
// The canonical form excludes ValidatorIndex and ValidatorAddress fields
func CanonicalizeVote(chainID string, vote *cmtproto.Vote) (cmtproto.CanonicalVote, error) {
	blockID, err := CanonicalizeBlockID(vote.BlockID)
	if err != nil {
		return cmtproto.CanonicalVote{}, fmt.Errorf("failed to canonicalize BlockID: %w", err)
	}

	return cmtproto.CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,
		Round:     int64(vote.Round),
		BlockID:   blockID,
		Timestamp: vote.Timestamp,
		ChainID:   chainID,
	}, nil
}

// MustCanonicalizeVote is like CanonicalizeVote but panics on error
func MustCanonicalizeVote(chainID string, vote *cmtproto.Vote) cmtproto.CanonicalVote {
	canonical, err := CanonicalizeVote(chainID, vote)
	if err != nil {
		panic(err)
	}
	return canonical
}

// CanonicalTime formats time in the canonical format, ensuring UTC timezone
func CanonicalTime(t time.Time) string {
	return cmttime.Canonical(t).Format(TimeFormat)
}
