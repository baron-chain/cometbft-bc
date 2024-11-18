package types

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/baron-chain/gogoproto-bc/proto"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    cmtproto "github.com/baron-chain/cometbft-bc/proto/tendermint/types"
)

func TestJSONMarshaling(t *testing.T) {
    testCases := []struct {
        name     string
        message  proto.Message
        validate func(*testing.T, []byte)
    }{
        {
            name:    "empty response includes all fields",
            message: &ResponseDeliverTx{},
            validate: func(t *testing.T, bz []byte) {
                require.Contains(t, string(bz), "code")
            },
        },
        {
            name: "marshal/unmarshal preserves data",
            message: &ResponseCheckTx{
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
            },
            validate: func(t *testing.T, bz []byte) {
                var decoded ResponseCheckTx
                require.NoError(t, json.Unmarshal(bz, &decoded))
                assert.Equal(t, decoded, message)
            },
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            bz, err := json.Marshal(tc.message)
            require.NoError(t, err)
            tc.validate(t, bz)
        })
    }
}

func TestProtoMessageSerialization(t *testing.T) {
    testCases := []struct {
        name     string
        message  proto.Message
        validate func(*testing.T, proto.Message)
    }{
        {
            name: "RequestEcho",
            message: &RequestEcho{
                Message: "Hello",
            },
            validate: func(t *testing.T, decoded proto.Message) {
                msg, ok := decoded.(*RequestEcho)
                require.True(t, ok)
                assert.Equal(t, "Hello", msg.Message)
            },
        },
        {
            name: "Header",
            message: &cmtproto.Header{
                Height:  4,
                ChainID: "baron-chain-test",
            },
            validate: func(t *testing.T, decoded proto.Message) {
                header, ok := decoded.(*cmtproto.Header)
                require.True(t, ok)
                assert.Equal(t, int64(4), header.Height)
                assert.Equal(t, "baron-chain-test", header.ChainID)
            },
        },
        {
            name: "ResponseCheckTx",
            message: &ResponseCheckTx{
                Data:      []byte("baron-chain-tx"),
                Log:       "tx-log",
                GasWanted: 10,
                Events: []Event{{
                    Type: "baron-event",
                    Attributes: []EventAttribute{{
                        Key:   "chain",
                        Value: "baron",
                    }},
                }},
            },
            validate: func(t *testing.T, decoded proto.Message) {
                resp, ok := decoded.(*ResponseCheckTx)
                require.True(t, ok)
                assert.Equal(t, []byte("baron-chain-tx"), resp.Data)
                assert.Equal(t, "tx-log", resp.Log)
                assert.Equal(t, int64(10), resp.GasWanted)
                require.Len(t, resp.Events, 1)
                assert.Equal(t, "baron-event", resp.Events[0].Type)
            },
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test message serialization roundtrip
            target := proto.Clone(tc.message)
            proto.Reset(target)
            
            err := testProtoRoundTrip(t, tc.message, target)
            require.NoError(t, err)
            
            // Run custom validation
            tc.validate(t, target)
        })
    }
}

func testProtoRoundTrip(t *testing.T, message, target proto.Message) error {
    buf := new(bytes.Buffer)
    
    // Write message
    if err := WriteMessage(message, buf); err != nil {
        return err
    }
    
    // Read message
    if err := ReadMessage(buf, target); err != nil {
        return err 
    }

    // Verify equality
    if !proto.Equal(message, target) {
        t.Errorf("messages do not match after round trip: got %v, want %v", target, message)
    }

    return nil
}

func TestExtendedProtoMessages(t *testing.T) {
    testCases := []struct {
        name     string
        message  proto.Message
        validate func(*testing.T, proto.Message)
    }{
        {
            name: "ResponseBeginBlock",
            message: &ResponseBeginBlock{
                Events: []Event{{
                    Type: "begin_block",
                    Attributes: []EventAttribute{{
                        Key:   "height",
                        Value: "1",
                    }},
                }},
            },
            validate: func(t *testing.T, decoded proto.Message) {
                resp, ok := decoded.(*ResponseBeginBlock)
                require.True(t, ok)
                require.Len(t, resp.Events, 1)
                assert.Equal(t, "begin_block", resp.Events[0].Type)
            },
        },
        {
            name: "ResponseEndBlock",
            message: &ResponseEndBlock{
                ValidatorUpdates: []ValidatorUpdate{{
                    PubKey: PubKey{Data: []byte("test_key")},
                    Power:  10,
                }},
                Events: []Event{{
                    Type: "end_block",
                    Attributes: []EventAttribute{{
                        Key:   "height",
                        Value: "1", 
                    }},
                }},
            },
            validate: func(t *testing.T, decoded proto.Message) {
                resp, ok := decoded.(*ResponseEndBlock)
                require.True(t, ok)
                require.Len(t, resp.ValidatorUpdates, 1)
                require.Len(t, resp.Events, 1)
                assert.Equal(t, "end_block", resp.Events[0].Type)
            },
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            target := proto.Clone(tc.message)
            proto.Reset(target)
            
            err := testProtoRoundTrip(t, tc.message, target)
            require.NoError(t, err)
            
            tc.validate(t, target)
        })
    }
}
