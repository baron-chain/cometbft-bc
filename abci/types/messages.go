package types

import (
    "io"
    "sync"

    "github.com/baron-chain/cometbft-bc/libs/protoio"
    "github.com/cosmos/gogoproto/proto"
)

const maxMsgSize = 104857600 // 100MB

var msgPool = sync.Pool{
    New: func() interface{} {
        return new(proto.Message)
    },
}

func WriteMessage(msg proto.Message, w io.Writer) error {
    writer := protoio.NewDelimitedWriter(w)
    defer writer.Close()
    _, err := writer.WriteMsg(msg)
    return err
}

func ReadMessage(r io.Reader, msg proto.Message) error {
    reader := protoio.NewDelimitedReader(r, maxMsgSize)
    defer reader.Close()
    _, err := reader.ReadMsg(msg)
    return err
}

// Request Converters
type RequestConverter struct{}

func (r RequestConverter) ToRequest(value interface{}) *Request {
    switch v := value.(type) {
    case string:
        return &Request{Value: &Request_Echo{&RequestEcho{Message: v}}}
    case *RequestFlush:
        return &Request{Value: &Request_Flush{v}}
    case RequestInfo:
        return &Request{Value: &Request_Info{&v}}
    case RequestDeliverTx:
        return &Request{Value: &Request_DeliverTx{&v}}
    case RequestCheckTx:
        return &Request{Value: &Request_CheckTx{&v}}
    case *RequestCommit:
        return &Request{Value: &Request_Commit{v}}
    case RequestQuery:
        return &Request{Value: &Request_Query{&v}}
    case RequestInitChain:
        return &Request{Value: &Request_InitChain{&v}}
    case RequestBeginBlock:
        return &Request{Value: &Request_BeginBlock{&v}}
    case RequestEndBlock:
        return &Request{Value: &Request_EndBlock{&v}}
    case RequestListSnapshots:
        return &Request{Value: &Request_ListSnapshots{&v}}
    case RequestOfferSnapshot:
        return &Request{Value: &Request_OfferSnapshot{&v}}
    case RequestLoadSnapshotChunk:
        return &Request{Value: &Request_LoadSnapshotChunk{&v}}
    case RequestApplySnapshotChunk:
        return &Request{Value: &Request_ApplySnapshotChunk{&v}}
    case RequestPrepareProposal:
        return &Request{Value: &Request_PrepareProposal{&v}}
    case RequestProcessProposal:
        return &Request{Value: &Request_ProcessProposal{&v}}
    default:
        return nil
    }
}

// Response Converters
type ResponseConverter struct{}

func (r ResponseConverter) ToResponse(value interface{}) *Response {
    switch v := value.(type) {
    case error:
        return &Response{Value: &Response_Exception{&ResponseException{Error: v.Error()}}}
    case string:
        if v == "" {
            return &Response{Value: &Response_Flush{&ResponseFlush{}}}
        }
        return &Response{Value: &Response_Echo{&ResponseEcho{Message: v}}}
    case ResponseInfo:
        return &Response{Value: &Response_Info{&v}}
    case ResponseDeliverTx:
        return &Response{Value: &Response_DeliverTx{&v}}
    case ResponseCheckTx:
        return &Response{Value: &Response_CheckTx{&v}}
    case ResponseCommit:
        return &Response{Value: &Response_Commit{&v}}
    case ResponseQuery:
        return &Response{Value: &Response_Query{&v}}
    case ResponseInitChain:
        return &Response{Value: &Response_InitChain{&v}}
    case ResponseBeginBlock:
        return &Response{Value: &Response_BeginBlock{&v}}
    case ResponseEndBlock:
        return &Response{Value: &Response_EndBlock{&v}}
    case ResponseListSnapshots:
        return &Response{Value: &Response_ListSnapshots{&v}}
    case ResponseOfferSnapshot:
        return &Response{Value: &Response_OfferSnapshot{&v}}
    case ResponseLoadSnapshotChunk:
        return &Response{Value: &Response_LoadSnapshotChunk{&v}}
    case ResponseApplySnapshotChunk:
        return &Response{Value: &Response_ApplySnapshotChunk{&v}}
    case ResponsePrepareProposal:
        return &Response{Value: &Response_PrepareProposal{&v}}
    case ResponseProcessProposal:
        return &Response{Value: &Response_ProcessProposal{&v}}
    default:
        return nil
    }
}

// Convenience functions for common operations
func ToRequestEcho(message string) *Request {
    return new(RequestConverter).ToRequest(message)
}

func ToRequestFlush() *Request {
    return new(RequestConverter).ToRequest(&RequestFlush{})
}

func ToResponseException(err error) *Response {
    return new(ResponseConverter).ToResponse(err)
}

func ToResponseEcho(message string) *Response {
    return new(ResponseConverter).ToResponse(message)
}
