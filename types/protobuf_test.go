package types

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    abci "github.com/baron-chain/cometbft-bc/abci/types"
    "github.com/baron-chain/cometbft-bc/crypto"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    cryptoenc "github.com/baron-chain/cometbft-bc/crypto/encoding"
)

func TestABCIPubKey(t *testing.T) {
    pk := kyber.GenPrivKey().PubKey()
    abciPubKey, err := cryptoenc.PubKeyToProto(pk)
    require.NoError(t, err)
    
    pk2, err := cryptoenc.PubKeyFromProto(abciPubKey)
    require.NoError(t, err)
    require.Equal(t, pk, pk2)
}

func TestABCIValidators(t *testing.T) {
    pk := kyber.GenPrivKey().PubKey()
    expectedVal := NewValidator(pk, 10)
    
    t.Run("Basic Validator Update", func(t *testing.T) {
        val := NewValidator(pk, 10)
        abciVal := TM2PB.ValidatorUpdate(val)
        vals, err := PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
        
        assert.NoError(t, err)
        assert.Equal(t, expectedVal, vals[0])
        
        abciVals := TM2PB.ValidatorUpdates(NewValidatorSet(vals))
        assert.Equal(t, []abci.ValidatorUpdate{abciVal}, abciVals)
    })
    
    t.Run("Validator with Address", func(t *testing.T) {
        val := NewValidator(pk, 10)
        val.Address = pk.Address()
        abciVal := TM2PB.ValidatorUpdate(val)
        vals, err := PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
        
        assert.NoError(t, err)
        assert.Equal(t, expectedVal, vals[0])
    })
}

type mockPubKey struct{}

func (mockPubKey) Address() Address                            { return []byte{} }
func (mockPubKey) Bytes() []byte                              { return []byte{} }
func (mockPubKey) VerifySignature([]byte, []byte) bool        { return false }
func (mockPubKey) Equals(crypto.PubKey) bool                  { return false }
func (mockPubKey) String() string                             { return "" }
func (mockPubKey) Type() string                               { return "mockPubKey" }

func TestValidatorUpdate(t *testing.T) {
    pubkey := kyber.GenPrivKey().PubKey()
    
    t.Run("Valid Update", func(t *testing.T) {
        abciVal := TM2PB.NewValidatorUpdate(pubkey, 10)
        assert.Equal(t, int64(10), abciVal.Power)
    })
    
    t.Run("Invalid Updates", func(t *testing.T) {
        assert.Panics(t, func() { TM2PB.NewValidatorUpdate(nil, 10) })
        assert.Panics(t, func() { TM2PB.NewValidatorUpdate(mockPubKey{}, 10) })
    })
}

func TestValidatorWithoutPubKey(t *testing.T) {
    pk := kyber.GenPrivKey().PubKey()
    val := NewValidator(pk, 10)
    abciVal := TM2PB.Validator(val)
    
    expected := abci.Validator{
        Address: pk.Address(),
        Power:   10,
    }
    
    assert.Equal(t, expected, abciVal)
}
