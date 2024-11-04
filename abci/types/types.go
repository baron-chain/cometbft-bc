package types

import (
	"bytes"
	"encoding/json"
	"github.com/cosmos/gogoproto/jsonpb"
)

const CodeTypeOK uint32 = 0

// JSON encoding configuration
var (
	jsonpbMarshaller = jsonpb.Marshaler{
		EnumsAsInts:  true,
		EmitDefaults: true,
	}
	jsonpbUnmarshaller = jsonpb.Unmarshaler{}
)

// Response status checks for different response types
type Response interface {
	IsOK() bool
	IsErr() bool
}

func (r ResponseCheckTx) IsOK() bool        { return r.Code == CodeTypeOK }
func (r ResponseCheckTx) IsErr() bool       { return r.Code != CodeTypeOK }
func (r ResponseDeliverTx) IsOK() bool      { return r.Code == CodeTypeOK }
func (r ResponseDeliverTx) IsErr() bool     { return r.Code != CodeTypeOK }
func (r ResponseQuery) IsOK() bool          { return r.Code == CodeTypeOK }
func (r ResponseQuery) IsErr() bool         { return r.Code != CodeTypeOK }

// ProcessProposal specific status checks
func (r ResponseProcessProposal) IsAccepted() bool      { return r.Status == ResponseProcessProposal_ACCEPT }
func (r ResponseProcessProposal) IsStatusUnknown() bool { return r.Status == ResponseProcessProposal_UNKNOWN }

// jsonRoundTripper ensures types implement both json.Marshaler and json.Unmarshaler
type jsonRoundTripper interface {
	json.Marshaler
	json.Unmarshaler
}

// marshalJSON is a helper function to implement common JSON marshaling logic
func marshalJSON(r interface{}) ([]byte, error) {
	s, err := jsonpbMarshaller.MarshalToString(r)
	return []byte(s), err
}

// unmarshalJSON is a helper function to implement common JSON unmarshaling logic
func unmarshalJSON(b []byte, r interface{}) error {
	return jsonpbUnmarshaller.Unmarshal(bytes.NewBuffer(b), r)
}

// MarshalJSON/UnmarshalJSON implementations
func (r *ResponseCheckTx) MarshalJSON() ([]byte, error)    { return marshalJSON(r) }
func (r *ResponseCheckTx) UnmarshalJSON(b []byte) error    { return unmarshalJSON(b, r) }
func (r *ResponseDeliverTx) MarshalJSON() ([]byte, error)  { return marshalJSON(r) }
func (r *ResponseDeliverTx) UnmarshalJSON(b []byte) error  { return unmarshalJSON(b, r) }
func (r *ResponseQuery) MarshalJSON() ([]byte, error)      { return marshalJSON(r) }
func (r *ResponseQuery) UnmarshalJSON(b []byte) error      { return unmarshalJSON(b, r) }
func (r *ResponseCommit) MarshalJSON() ([]byte, error)     { return marshalJSON(r) }
func (r *ResponseCommit) UnmarshalJSON(b []byte) error     { return unmarshalJSON(b, r) }
func (r *EventAttribute) MarshalJSON() ([]byte, error)     { return marshalJSON(r) }
func (r *EventAttribute) UnmarshalJSON(b []byte) error     { return unmarshalJSON(b, r) }

// Compile-time type assertions
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
