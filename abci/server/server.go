/*
Package server provides Baron Chain's ABCI server implementations.
Supported transport protocols:
  - gRPC: High-performance Protocol Buffers RPC server with quantum-safe cryptography
  - Socket: TCP socket-based server with AI-optimized routing
*/
package server

import (
    "errors"
    "fmt"
    "strings"

    "github.com/baron-chain/cometbft-bc/abci/types"
    "github.com/baron-chain/cometbft-bc/libs/service"
)

// BCTransport represents Baron Chain server transport protocol
type BCTransport string

const (
    // Transport protocols
    TransportGRPC   BCTransport = "grpc"
    TransportSocket BCTransport = "socket"
)

var (
    ErrInvalidTransport = errors.New("invalid baron chain transport protocol")
    ErrEmptyAddress    = errors.New("baron chain server address required")
    ErrNilApplication  = errors.New("baron chain application required")
)

// BCServerOption configures Baron Chain server setup
type BCServerOption func(*serverConfig)

// serverConfig represents Baron Chain server configuration
type serverConfig struct {
    grpcOpts   []GRPCOption
    socketOpts []SocketOption
}

// WithGRPCConfig sets gRPC-specific server configuration
func WithGRPCConfig(opts ...GRPCOption) BCServerOption {
    return func(cfg *serverConfig) {
        cfg.grpcOpts = append(cfg.grpcOpts, opts...)
    }
}

// WithSocketConfig sets socket-specific server configuration
func WithSocketConfig(opts ...SocketOption) BCServerOption {
    return func(cfg *serverConfig) {
        cfg.socketOpts = append(cfg.socketOpts, opts...)
    }
}

// NewBCServer creates a new Baron Chain ABCI server
//
// Parameters:
//   - address: Server address (e.g., "tcp://0.0.0.0:26658")
//   - transport: Transport protocol ("grpc" or "socket")
//   - app: ABCI application implementation
//   - opts: Optional server configurations
//
// Returns:
//   - service.Service: Server instance
//   - error: Any initialization error
func NewBCServer(address string, transport string, app types.Application, opts ...BCServerOption) (service.Service, error) {
    if err := validateServerParams(address, transport, app); err != nil {
        return nil, err
    }

    cfg := &serverConfig{}
    for _, opt := range opts {
        opt(cfg)
    }

    t := BCTransport(strings.ToLower(transport))
    switch t {
    case TransportSocket:
        return NewSocketServer(address, app, cfg.socketOpts...), nil
    case TransportGRPC:
        grpcApp := types.NewGRPCApplication(app)
        server, err := NewGRPCServer(address, grpcApp, cfg.grpcOpts...)
        if err != nil {
            return nil, fmt.Errorf("baron chain grpc server creation failed: %w", err)
        }
        return server, nil
    default:
        return nil, fmt.Errorf("%w: %s", ErrInvalidTransport, transport)
    }
}

// validateServerParams validates server initialization parameters
func validateServerParams(addr string, transport string, app types.Application) error {
    switch {
    case addr == "":
        return ErrEmptyAddress
    case app == nil:
        return ErrNilApplication
    case transport == "":
        return ErrInvalidTransport
    }
    return nil
}

// IsValidBCTransport checks if transport protocol is supported
func IsValidBCTransport(transport string) bool {
    t := BCTransport(strings.ToLower(transport))
    return t == TransportGRPC || t == TransportSocket
}

// GetSupportedTransports returns available transport protocols
func GetSupportedTransports() []string {
    return []string{
        string(TransportGRPC),
        string(TransportSocket),
    }
}
