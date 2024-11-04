package types

import (
	"fmt"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptoenc "github.com/cometbft/cometbft/crypto/encoding"
	"github.com/cometbft/cometbft/crypto/secp256k1"
)

// ValidatorKeyType represents supported validator public key types
type ValidatorKeyType string

const (
	KeyTypeEd25519   ValidatorKeyType = ed25519.KeyType
	KeyTypeSecp256k1 ValidatorKeyType = secp256k1.KeyType
)

// createValidatorUpdate creates a ValidatorUpdate with the given public key and power
func createValidatorUpdate(pubKey interface{}, power int64) (ValidatorUpdate, error) {
	pkProto, err := cryptoenc.PubKeyToProto(pubKey)
	if err != nil {
		return ValidatorUpdate{}, fmt.Errorf("failed to convert public key to proto: %w", err)
	}

	return ValidatorUpdate{
		PubKey: pkProto,
		Power:  power,
	}, nil
}

// Ed25519ValidatorUpdate creates a ValidatorUpdate with an Ed25519 public key
func Ed25519ValidatorUpdate(pk []byte, power int64) (ValidatorUpdate, error) {
	pubKey := ed25519.PubKey(pk)
	return createValidatorUpdate(pubKey, power)
}

// UpdateValidator creates a ValidatorUpdate based on the provided key type
// If keyType is empty, Ed25519 is used as default
func UpdateValidator(pk []byte, power int64, keyType string) (ValidatorUpdate, error) {
	if keyType == "" {
		keyType = string(KeyTypeEd25519)
	}

	switch ValidatorKeyType(keyType) {
	case KeyTypeEd25519:
		return Ed25519ValidatorUpdate(pk, power)
	
	case KeyTypeSecp256k1:
		pubKey := secp256k1.PubKey(pk)
		return createValidatorUpdate(pubKey, power)
	
	default:
		return ValidatorUpdate{}, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// MustUpdateValidator is like UpdateValidator but panics on error
func MustUpdateValidator(pk []byte, power int64, keyType string) ValidatorUpdate {
	update, err := UpdateValidator(pk, power, keyType)
	if err != nil {
		panic(err)
	}
	return update
}
