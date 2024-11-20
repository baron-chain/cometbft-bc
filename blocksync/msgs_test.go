package blocksync_test

import (
    "encoding/hex"
    "math"
    "testing"

    "github.com/baron-chain/gogoproto-bc/proto"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/baron-chain/cometbft-bc/blocksync"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/blocksync"
    "github.com/baron-chain/cometbft-bc/types"
)

func TestBlockRequestValidation(t *testing.T) {
    tests := []struct {
        name    string
        height  int64
        wantErr bool
    }{
        {"valid zero height", 0, false},
        {"valid positive height", 1, false},
        {"invalid negative height", -1, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            request := bcproto.BlockRequest{Height: tt.height}
            err := blocksync.ValidateMsg(&request)
            assert.Equal(t, tt.wantErr, err != nil)
        })
    }
}

func TestNoBlockResponseValidation(t *testing.T) {
    tests := []struct {
        name    string
        height  int64
        wantErr bool
    }{
        {"valid zero height", 0, false},
        {"valid positive height", 1, false}, 
        {"invalid negative height", -1, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            response := bcproto.NoBlockResponse{Height: tt.height}
            err := blocksync.ValidateMsg(&response)
            assert.Equal(t, tt.wantErr, err != nil)
        })
    }
}

func TestStatusRequestValidation(t *testing.T) {
    err := blocksync.ValidateMsg(&bcproto.StatusRequest{})
    assert.NoError(t, err)
}

func TestStatusResponseValidation(t *testing.T) {
    tests := []struct {
        name    string
        height  int64
        wantErr bool 
    }{
        {"valid zero height", 0, false},
        {"valid positive height", 1, false},
        {"invalid negative height", -1, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            response := bcproto.StatusResponse{Height: tt.height}
            err := blocksync.ValidateMsg(&response)
            assert.Equal(t, tt.wantErr, err != nil)
        })
    }
}

func TestMessageVectors(t *testing.T) {
    block := types.MakeBlock(3, []types.Tx{types.Tx("Hello World")}, nil, nil)
    block.Version.Block = 11

    bpb, err := block.ToProto()
    require.NoError(t, err)

    tests := []struct {
        name     string
        msg      proto.Message
        wantHex  string
    }{
        {
            "block request",
            &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
                BlockRequest: &bcproto.BlockRequest{Height: 1}}},
            "0a020801",
        },
        {
            "max height block request", 
            &bcproto.Message{Sum: &bcproto.Message_BlockRequest{
                BlockRequest: &bcproto.BlockRequest{Height: math.MaxInt64}}},
            "0a0a08ffffffffffffffff7f",
        },
        {
            "block response",
            &bcproto.Message{Sum: &bcproto.Message_BlockResponse{
                BlockResponse: &bcproto.BlockResponse{Block: bpb}}},
            "1a700a6e0a5b0a02080b1803220b088092b8c398feffffff012a0212003a20c4da88e876062aa1543400d50d0eaa0dac88096057949cfb7bca7f3a48c04bf96a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855120d0a0b48656c6c6f20576f726c641a00",
        },
        {
            "no block response",
            &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
                NoBlockResponse: &bcproto.NoBlockResponse{Height: 1}}},
            "12020801",
        },
        {
            "max height no block response",
            &bcproto.Message{Sum: &bcproto.Message_NoBlockResponse{
                NoBlockResponse: &bcproto.NoBlockResponse{Height: math.MaxInt64}}},
            "120a08ffffffffffffffff7f",
        },
        {
            "status request",
            &bcproto.Message{Sum: &bcproto.Message_StatusRequest{
                StatusRequest: &bcproto.StatusRequest{}}},
            "2200",
        },
        {
            "status response",
            &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
                StatusResponse: &bcproto.StatusResponse{Height: 1, Base: 2}}},
            "2a0408011002",
        },
        {
            "max height status response",
            &bcproto.Message{Sum: &bcproto.Message_StatusResponse{
                StatusResponse: &bcproto.StatusResponse{Height: math.MaxInt64, Base: math.MaxInt64}}},
            "2a1408ffffffffffffffff7f10ffffffffffffffff7f",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            bz, err := proto.Marshal(tt.msg)
            require.NoError(t, err)
            assert.Equal(t, tt.wantHex, hex.EncodeToString(bz))
        })
    }
}
