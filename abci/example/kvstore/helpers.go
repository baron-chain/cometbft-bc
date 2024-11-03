package kvstore

import (
	"errors"

	"github.com/cometbft/cometbft/abci/types"
	cmtrand "github.com/cometbft/cometbft/libs/rand"
)

const (
	defaultPubKeyLength = 32
	minValidatorCount  = 1
	maxValidatorCount  = 100
)

var (
	ErrInvalidValidatorCount = errors.New("validator count must be between 1 and 100")
	ErrNilApplication       = errors.New("application cannot be nil")
)

// ValidatorGenerator handles creation of validator updates
type ValidatorGenerator struct {
	pubKeyLength int
}

// NewValidatorGenerator creates a new validator generator with custom settings
func NewValidatorGenerator(pubKeyLength int) *ValidatorGenerator {
	if pubKeyLength <= 0 {
		pubKeyLength = defaultPubKeyLength
	}
	
	return &ValidatorGenerator{
		pubKeyLength: pubKeyLength,
	}
}

// GenerateValidator creates a single random validator with deterministic public key
// derived from the input index and random power value
func (vg *ValidatorGenerator) GenerateValidator(index int) types.ValidatorUpdate {
	pubkey := cmtrand.Bytes(vg.pubKeyLength)
	power := int64(cmtrand.Uint16() + 1) // Ensure non-zero power
	
	return types.UpdateValidator(pubkey, power, "")
}

// GenerateValidators creates a specified number of random validators
func (vg *ValidatorGenerator) GenerateValidators(count int) ([]types.ValidatorUpdate, error) {
	if count < minValidatorCount || count > maxValidatorCount {
		return nil, ErrInvalidValidatorCount
	}

	validators := make([]types.ValidatorUpdate, count)
	for i := 0; i < count; i++ {
		validators[i] = vg.GenerateValidator(i)
	}
	
	return validators, nil
}

// InitializeApp initializes a KVStore application with default settings
func InitializeApp(app *PersistentKVStoreApplication) error {
	if app == nil {
		return ErrNilApplication
	}

	generator := NewValidatorGenerator(defaultPubKeyLength)
	validators, err := generator.GenerateValidators(minValidatorCount)
	if err != nil {
		return err
	}

	app.InitChain(types.RequestInitChain{
		Validators: validators,
	})

	return nil
}

// InitializeAppWithValidators initializes a KVStore application with a specific number of validators
func InitializeAppWithValidators(app *PersistentKVStoreApplication, validatorCount int) error {
	if app == nil {
		return ErrNilApplication
	}

	generator := NewValidatorGenerator(defaultPubKeyLength)
	validators, err := generator.GenerateValidators(validatorCount)
	if err != nil {
		return err
	}

	app.InitChain(types.RequestInitChain{
		Validators: validators,
	})

	return nil
}
