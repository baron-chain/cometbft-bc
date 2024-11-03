/*
Package server provides ABCI server implementations.

It supports multiple transport protocols:
  - gRPC: A high-performance RPC server using Protocol Buffers
  - Socket: A simple TCP socket-based server
*/
package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/service"
)

// Transport represents the server transport protocol
type Transport string

const (
	// TransportGRPC represents the gRPC transport protocol
	TransportGRPC Transport = "grpc"

	// TransportSocket represents the socket transport protocol
	TransportSocket Transport = "socket"
)

// Common errors that may be returned by the server factory
var (
	ErrInvalidTransport  = errors.New("invalid transport protocol")
	ErrEmptyAddress     = errors.New("server address cannot be empty")
	ErrNilApplication   = errors.New("application cannot be nil")
)

// ServerOption configures how we set up the server
type ServerOption func(*serverOptions)

// serverOptions represents configurable server options
type serverOptions struct {
	grpcOptions    []GRPCOption
	socketOptions  []SocketOption
}

// WithGRPCOptions sets gRPC-specific server options
func WithGRPCOptions(opts ...GRPCOption) ServerOption {
	return func(o *serverOptions) {
		o.grpcOptions = append(o.grpcOptions, opts...)
	}
}

// WithSocketOptions sets socket-specific server options
func WithSocketOptions(opts ...SocketOption) ServerOption {
	return func(o *serverOptions) {
		o.socketOptions = append(o.socketOptions, opts...)
	}
}

// NewServer creates a new ABCI server with the specified transport.
//
// Parameters:
//   - protoAddr: The protocol address to listen on (e.g., "tcp://0.0.0.0:26658")
//   - transport: The transport protocol to use ("grpc" or "socket")
//   - app: The ABCI application implementation
//   - opts: Optional server configuration options
//
// Returns:
//   - service.Service: The created server instance
//   - error: Any error that occurred during server creation
func NewServer(protoAddr string, transport string, app types.Application, opts ...ServerOption) (service.Service, error) {
	if err := validateInputs(protoAddr, transport, app); err != nil {
		return nil, err
	}

	options := &serverOptions{}
	for _, opt := range opts {
		opt(options)
	}

	t := Transport(strings.ToLower(transport))
	switch t {
	case TransportSocket:
		return NewSocketServer(protoAddr, app, options.socketOptions...), nil

	case TransportGRPC:
		grpcApp := types.NewGRPCApplication(app)
		server, err := NewGRPCServer(protoAddr, grpcApp, options.grpcOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC server: %w", err)
		}
		return server, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransport, transport)
	}
}

// validateInputs validates the server creation parameters
func validateInputs(addr string, transport string, app types.Application) error {
	if addr == "" {
		return ErrEmptyAddress
	}

	if app == nil {
		return ErrNilApplication
	}

	if transport == "" {
		return ErrInvalidTransport
	}

	return nil
}

// IsValidTransport checks if the given transport protocol is supported
func IsValidTransport(transport string) bool {
	t := Transport(strings.ToLower(transport))
	switch t {
	case TransportGRPC, TransportSocket:
		return true
	default:
		return false
	}
}

// GetAvailableTransports returns a list of supported transport protocols
func GetAvailableTransports() []string {
	return []string{
		string(TransportGRPC),
		string(TransportSocket),
	}
}
