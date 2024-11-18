package types

import (
	"fmt"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptoenc "github.com/cometbft/cometbft/crypto/encoding"
	"github.com/cometbft/cometbft/crypto/secp256k1"
)

// ValidatorKeyType represents supported validator public key types
type ValidatorKeyType string
