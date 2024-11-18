package types

import (
    "fmt"
    "sync"

    "github.com/baron-chain/cometbft-bc/crypto/ed25519"
    cryptoenc "github.com/baron-chain/cometbft-bc/crypto/encoding"
    "github.com/baron-chain/cometbft-bc/crypto/secp256k1"
)

type ValidatorKeyType string

const (
    KeyTypeEd25519   ValidatorKeyType = ed25519.KeyType
    KeyTypeSecp256k1 ValidatorKeyType = secp256k1.KeyType
)

var (
    validatorPool sync.Pool
    pubKeyPool    sync.Pool
)

func init() {
    validatorPool = sync.Pool{
        New: func() interface{} {
            return new(ValidatorUpdate)
        },
    }
    pubKeyPool = sync.Pool{
        New: func() interface{} {
            return new(PubKey)
        },
    }
}

type ValidatorManager struct {
    mu sync.RWMutex
}

func NewValidatorManager() *ValidatorManager {
    return &ValidatorManager{}
}

func (vm *ValidatorManager) createValidatorUpdate(pubKey interface{}, power int64) (*ValidatorUpdate, error) {
    vm.mu.Lock()
    defer vm.mu.Unlock()

    pkProto, err := cryptoenc.PubKeyToProto(pubKey)
    if err != nil {
        return nil, fmt.Errorf("failed to convert public key to proto: %w", err)
    }

    validator := validatorPool.Get().(*ValidatorUpdate)
    validator.PubKey = pkProto
    validator.Power = power

    return validator, nil
}

func (vm *ValidatorManager) Ed25519ValidatorUpdate(pk []byte, power int64) (*ValidatorUpdate, error) {
    if len(pk) != ed25519.PubKeySize {
        return nil, fmt.Errorf("invalid Ed25519 public key size: expected %d, got %d", 
            ed25519.PubKeySize, len(pk))
    }

    pubKey := ed25519.PubKey(pk)
    return vm.createValidatorUpdate(pubKey, power)
}

func (vm *ValidatorManager) Secp256k1ValidatorUpdate(pk []byte, power int64) (*ValidatorUpdate, error) {
    if len(pk) != secp256k1.PubKeySize {
        return nil, fmt.Errorf("invalid Secp256k1 public key size: expected %d, got %d",
            secp256k1.PubKeySize, len(pk))
    }

    pubKey := secp256k1.PubKey(pk)
    return vm.createValidatorUpdate(pubKey, power)
}

func (vm *ValidatorManager) UpdateValidator(pk []byte, power int64, keyType ValidatorKeyType) (*ValidatorUpdate, error) {
    vm.mu.RLock()
    defer vm.mu.RUnlock()

    if keyType == "" {
        keyType = KeyTypeEd25519
    }

    switch keyType {
    case KeyTypeEd25519:
        return vm.Ed25519ValidatorUpdate(pk, power)
    case KeyTypeSecp256k1:
        return vm.Secp256k1ValidatorUpdate(pk, power)
    default:
        return nil, fmt.Errorf("unsupported key type: %s", keyType)
    }
}

func (vm *ValidatorManager) MustUpdateValidator(pk []byte, power int64, keyType ValidatorKeyType) *ValidatorUpdate {
    update, err := vm.UpdateValidator(pk, power, keyType)
    if err != nil {
        panic(fmt.Sprintf("failed to update validator: %v", err))
    }
    return update
}

func (vm *ValidatorManager) ReleaseValidator(v *ValidatorUpdate) {
    if v == nil {
        return
    }
    vm.mu.Lock()
    defer vm.mu.Unlock()
    
    // Clear validator data before returning to pool
    v.PubKey.Reset()
    v.Power = 0
    validatorPool.Put(v)
}

// Helper functions for backward compatibility
func Ed25519ValidatorUpdate(pk []byte, power int64) (*ValidatorUpdate, error) {
    return NewValidatorManager().Ed25519ValidatorUpdate(pk, power)
}

func UpdateValidator(pk []byte, power int64, keyType string) (*ValidatorUpdate, error) {
    return NewValidatorManager().UpdateValidator(pk, power, ValidatorKeyType(keyType))
}

func MustUpdateValidator(pk []byte, power int64, keyType string) *ValidatorUpdate {
    return NewValidatorManager().MustUpdateValidator(pk, power, ValidatorKeyType(keyType))
}
