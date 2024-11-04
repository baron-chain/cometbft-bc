package types

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

func TestJSONMarshaling(t *testing.T) {
	t.Run("empty response includes all fields", func(t *testing.T) {
		resp := &ResponseDeliverTx{}
		bz, err := json.Marshal(resp)
		require.NoError(t, err)
		require.Contains(t, string(bz), "code")
	})

	t.Run("marshal/unmarshal preserves data", func(t *testing.T) {
		original := ResponseCheckTx{
			Code:      1,
			Data:      []byte("hello"),
			GasWanted: 43,
			Events: []Event{{
				Type: "testEvent",
				Attributes: []EventAttribute{{
					Key:   "pho",
					Value: "bo",
				}},
			}},
		}

		bz, err := json.Marshal(&original)
		require.NoError(t, err)

		var decoded ResponseCheckTx
		err = json.Unmarshal(bz, &decoded)
		require.NoError(t, err)
		assert.Equal(t, original, decoded)
	})
}

func TestProtoMessageSerialization(t *testing.T) {
	testCases := []struct {
		name    string
		message proto.Message
		target  proto.Message
	}{
		{
			name: "RequestEcho",
			message: &RequestEcho{
				Message: "Hello",
			},
			target: new(RequestEcho),
		},
		{
			name: "Header",
			message: &cmtproto.Header{
				Height:  4,
				ChainID: "test",
			},
			target: new(cmtproto.Header),
		},
		{
			name: "ResponseCheckTx",
			message: &ResponseCheckTx{
				Data:      []byte("hello-world"),
				Log:       "hello-world",
				GasWanted: 10,
				Events: []Event{{
					Type: "testEvent",
					Attributes: []EventAttribute{{
						Key:   "abc",
						Value: "def",
					}},
				}},
			},
			target: new(ResponseCheckTx),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testProtoRoundTrip(t, tc.message, tc.target)
		})
	}
}

// testProtoRoundTrip tests that a protobuf message can be written and read back correctly
func testProtoRoundTrip(t *testing.T, message proto.Message, target proto.Message) {
	buf := new(bytes.Buffer)
	
	err := WriteMessage(message, buf)
	require.NoError(t, err, "WriteMessage failed")
	
	err = ReadMessage(buf, target)
	require.NoError(t, err, "ReadMessage failed")
	
	assert.True(t, proto.Equal(message, target), 
		"messages do not match after round trip: got %v, want %v", 
		target, message)
}

// Test cases to add when implementing TODO items:
/*
func TestCompleteProtoMessageSet(t *testing.T) {
	testCases := []struct {
		name    string
		message proto.Message
		target  proto.Message
	}{
		// Add other message types here as they are implemented
		{
			name: "ResponseBeginBlock",
			message: &ResponseBeginBlock{
				Events: []Event{...},
			},
			target: new(ResponseBeginBlock),
		},
		{
			name: "ResponseEndBlock",
			message: &ResponseEndBlock{
				ValidatorUpdates: ...,
				ConsensusParamUpdates: ...,
				Events: ...,
			},
			target: new(ResponseEndBlock),
		},
		// ... additional test cases ...
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testProtoRoundTrip(t, tc.message, tc.target)
		})
	}
}
*/
