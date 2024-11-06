package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validatorTestCase struct {
	description string
	validator   *Validator
	expectPass  bool
	errMsg      string
}

func TestValidatorProtoBuf(t *testing.T) {
	validator, err := RandValidator(true, 100)
	require.NoError(t, err)

	tests := []struct {
		description     string
		validator      *Validator
		expectProto    bool
		expectFromProto bool
	}{
		{
			description:     "valid validator",
			validator:      validator,
			expectProto:    true,
			expectFromProto: true,
		},
		{
			description:     "empty validator",
			validator:      &Validator{},
			expectProto:    false,
			expectFromProto: false,
		},
		{
			description:     "nil validator",
			validator:      nil,
			expectProto:    false,
			expectFromProto: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			protoVal, err := tc.validator.ToProto()
			if tc.expectProto {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				return
			}

			val, err := ValidatorFromProto(protoVal)
			if tc.expectFromProto {
				require.NoError(t, err)
				require.Equal(t, tc.validator, val)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidatorValidateBasic(t *testing.T) {
	priv := NewMockPV()
	pubKey, err := priv.GetPubKey()
	require.NoError(t, err)

	tests := []validatorTestCase{
		{
			description: "valid validator",
			validator:   NewValidator(pubKey, 1),
			expectPass: true,
		},
		{
			description: "nil validator",
			validator:   nil,
			expectPass: false,
			errMsg:     "nil validator",
		},
		{
			description: "missing public key",
			validator:   &Validator{PubKey: nil},
			expectPass: false,
			errMsg:     "validator does not have a public key",
		},
		{
			description: "negative voting power",
			validator:   NewValidator(pubKey, -1),
			expectPass: false,
			errMsg:     "validator has negative voting power",
		},
		{
			description: "missing address",
			validator: &Validator{
				PubKey:  pubKey,
				Address: nil,
			},
			expectPass: false,
			errMsg:     "validator address is the wrong size: ",
		},
		{
			description: "invalid address size",
			validator: &Validator{
				PubKey:  pubKey,
				Address: []byte{'a'},
			},
			expectPass: false,
			errMsg:     "validator address is the wrong size: 61",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.validator.ValidateBasic()
			if tc.expectPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tc.errMsg, err.Error())
			}
		})
	}
}
