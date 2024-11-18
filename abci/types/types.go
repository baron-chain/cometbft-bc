package types

import (
    "bytes"
    "encoding/json"
    "sync"
    
    "github.com/baron-chain/gogoproto-bc/jsonpb"
)

const (
    CodeTypeOK uint32 = 0
)

var (
    jsonPool = sync.Pool{
        New: func() interface{} {
            return &bytes.Buffer{}
        },
    }

    marshallerPool = sync.Pool{
        New: func() interface{} {
            return &jsonpb.Marshaler{
                EnumsAsInts:  true,
                EmitDefaults: true,
            }
        },
    }

    unmarshallerPool = sync.Pool{
        New: func() interface{} {
            return &jsonpb.Unmarshaler{}
        },
    }
)

type Response interface {
    IsOK() bool
    IsErr() bool
    GetCode() uint32
}

type jsonRoundTripper interface {
    json.Marshaler
    json.Unmarshaler
}

// ResponseBase implements common response functionality
type ResponseBase struct {
    Code uint32
}

func (r ResponseBase) IsOK() bool  { return r.Code == CodeTypeOK }
func (r ResponseBase) IsErr() bool { return r.Code != CodeTypeOK }
func (r ResponseBase) GetCode() uint32 { return r.Code }

// Response implementations
func (r ResponseCheckTx) IsOK() bool    { return r.Code == CodeTypeOK }
func (r ResponseCheckTx) IsErr() bool   { return r.Code != CodeTypeOK }
func (r ResponseCheckTx) GetCode() uint32 { return r.Code }

func (r ResponseDeliverTx) IsOK() bool  { return r.Code == CodeTypeOK }
func (r ResponseDeliverTx) IsErr() bool { return r.Code != CodeTypeOK }
func (r ResponseDeliverTx) GetCode() uint32 { return r.Code }

func (r ResponseQuery) IsOK() bool      { return r.Code == CodeTypeOK }
func (r ResponseQuery) IsErr() bool     { return r.Code != CodeTypeOK }
func (r ResponseQuery) GetCode() uint32 { return r.Code }

// ProcessProposal specific status
type ProposalStatus uint32

const (
    ProposalStatusUnknown ProposalStatus = iota
    ProposalStatusAccept
    ProposalStatusReject
)

func (r ResponseProcessProposal) GetStatus() ProposalStatus {
    switch r.Status {
    case ResponseProcessProposal_ACCEPT:
        return ProposalStatusAccept
    case ResponseProcessProposal_UNKNOWN:
        return ProposalStatusUnknown
    default:
        return ProposalStatusReject
    }
}

func (r ResponseProcessProposal) IsAccepted() bool {
    return r.Status == ResponseProcessProposal_ACCEPT
}

func (r ResponseProcessProposal) IsStatusUnknown() bool {
    return r.Status == ResponseProcessProposal_UNKNOWN
}

// JSON marshaling optimization
type JSONMarshaler struct {
    mu sync.Mutex
}

func NewJSONMarshaler() *JSONMarshaler {
    return &JSONMarshaler{}
}

func (j *JSONMarshaler) marshalJSON(r interface{}) ([]byte, error) {
    j.mu.Lock()
    defer j.mu.Unlock()

    buf := jsonPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        jsonPool.Put(buf)
    }()

    marshaller := marshallerPool.Get().(*jsonpb.Marshaler)
    defer marshallerPool.Put(marshaller)

    if err := marshaller.Marshal(buf, r); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

func (j *JSONMarshaler) unmarshalJSON(data []byte, r interface{}) error {
    j.mu.Lock()
    defer j.mu.Unlock()

    unmarshaller := unmarshallerPool.Get().(*jsonpb.Unmarshaler)
    defer unmarshallerPool.Put(unmarshaller)

    return unmarshaller.Unmarshal(bytes.NewReader(data), r)
}

// Optimized JSON implementations
var defaultJSONMarshaler = NewJSONMarshaler()

func (r *ResponseCheckTx) MarshalJSON() ([]byte, error) {
    return defaultJSONMarshaler.marshalJSON(r)
}

func (r *ResponseCheckTx) UnmarshalJSON(b []byte) error {
    return defaultJSONMarshaler.unmarshalJSON(b, r)
}

func (r *ResponseDeliverTx) MarshalJSON() ([]byte, error) {
    return defaultJSONMarshaler.marshalJSON(r)
}

func (r *ResponseDeliverTx) UnmarshalJSON(b []byte) error {
    return defaultJSONMarshaler.unmarshalJSON(b, r)
}

func (r *ResponseQuery) MarshalJSON() ([]byte, error) {
    return defaultJSONMarshaler.marshalJSON(r)
}

func (r *ResponseQuery) UnmarshalJSON(b []byte) error {
    return defaultJSONMarshaler.unmarshalJSON(b, r)
}

func (r *ResponseCommit) MarshalJSON() ([]byte, error) {
    return defaultJSONMarshaler.marshalJSON(r)
}

func (r *ResponseCommit) UnmarshalJSON(b []byte) error {
    return defaultJSONMarshaler.unmarshalJSON(b, r)
}

func (r *EventAttribute) MarshalJSON() ([]byte, error) {
    return defaultJSONMarshaler.marshalJSON(r)
}

func (r *EventAttribute) UnmarshalJSON(b []byte) error {
    return defaultJSONMarshaler.unmarshalJSON(b, r)
}

// Type assertions
var (
    _ Response = (*ResponseCheckTx)(nil)
    _ Response = (*ResponseDeliverTx)(nil)
    _ Response = (*ResponseQuery)(nil)
    
    _ jsonRoundTripper = (*ResponseCommit)(nil)
    _ jsonRoundTripper = (*ResponseQuery)(nil)
    _ jsonRoundTripper = (*ResponseDeliverTx)(nil)
    _ jsonRoundTripper = (*ResponseCheckTx)(nil)
    _ jsonRoundTripper = (*EventAttribute)(nil)
)
