package types

import cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

func IsVoteTypeValid(t cmtproto.SignedMsgType) bool {
	switch t {
	case cmtproto.PrevoteType, cmtproto.PrecommitType:
		return true
	default:
		return false
	}
}
