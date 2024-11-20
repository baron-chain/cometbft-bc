package blocksync

import (
    "errors"
    "fmt"

    "github.com/cosmos/gogoproto/proto"
    bcproto "github.com/baron-chain/cometbft-bc/proto/tendermint/blocksync"
    "github.com/baron-chain/cometbft-bc/types"
)

const (
    BlockResponseMessagePrefixSize   = 4
    BlockResponseMessageFieldKeySize = 1
    MaxMsgSize                       = types.MaxBlockSizeBytes + BlockResponseMessagePrefixSize + BlockResponseMessageFieldKeySize
)

var (
    ErrNilMessage     = errors.New("message cannot be nil")
    ErrNegativeHeight = errors.New("negative height")
    ErrNegativeBase   = errors.New("negative base")
)

func ValidateMsg(pb proto.Message) error {
    if pb == nil {
        return ErrNilMessage
    }

    switch msg := pb.(type) {
    case *bcproto.BlockRequest:
        if msg.Height < 0 {
            return ErrNegativeHeight
        }

    case *bcproto.BlockResponse:
        if _, err := types.BlockFromProto(msg.Block); err != nil {
            return fmt.Errorf("invalid block: %w", err)
        }

    case *bcproto.NoBlockResponse:
        if msg.Height < 0 {
            return ErrNegativeHeight
        }

    case *bcproto.StatusResponse:
        if err := validateStatusResponse(msg); err != nil {
            return err
        }

    case *bcproto.StatusRequest:
        return nil

    default:
        return fmt.Errorf("unknown message type %T", msg)
    }

    return nil
}

func validateStatusResponse(msg *bcproto.StatusResponse) error {
    if msg.Base < 0 {
        return ErrNegativeBase
    }
    if msg.Height < 0 {
        return ErrNegativeHeight
    }
    if msg.Base > msg.Height {
        return fmt.Errorf("base %v cannot be greater than height %v", msg.Base, msg.Height)
    }
    return nil
}
