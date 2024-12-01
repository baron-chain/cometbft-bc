package types

import (
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtmath "github.com/cometbft/cometbft/libs/math"
)

var (
	MaxSignatureSize = cmtmath.MaxInt(ed25519.SignatureSize, 64)
)

type Signable interface {
	SignBytes(chainID string) []byte
}
