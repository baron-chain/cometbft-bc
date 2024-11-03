package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/cometbft/cometbft/abci/types"
	cmtnet "github.com/cometbft/cometbft/libs/net"
	"github.com/cometbft/cometbft/libs/service"
)

const (
	// Server configuration defaults
	defaultReadBufferSize     = 1024 * 1024      // 1MB
	defaultWriteBufferSize    = 1024 * 1024      // 1MB
	defaultMaxRecvMsgSize    = 10 * 1024 * 1024 // 10MB
	defaultMaxSendMsgSize    = 10 * 1024 * 1024 // 10MB
	defaultShutdownTimeout   = 5 * time.Second
	defaultKeepAliveTime     = 5 * time.Minute
	defaultKeepAliveTimeout  = 20 * time.Second
)

var (
	ErrServerNotInitialized = errors.New("server not initialized")
	ErrServerAlreadyStarted = errors.New("server already started")
	ErrInvalidAddress      = errors.New("invalid server address")
	ErrNilApplication      = errors.New("application cannot be nil")
)

// ServerOption allows customization of the gRPC server.
type ServerOption func(*GRPCServer)

// GRPCServer represents a gRPC ABCI server instance.
type GRPCServer struct {
	service.BaseService

	proto    string
	addr     string
	app      types.ABCIApplicationServer
	server   *grpc.Server
	listener net.Listener

	maxRecvMsgSize    int
	maxSendMsgSize    int
	shutdownTimeout   time.Duration
	keepAliveTime     time.Duration
	keepAliveTimeout  time.Duration

	mu     sync.RWMutex
	done   chan struct{}
}

// WithMaxRecvMsgSize sets the maximum message size the server can receive.
func WithMaxRecvMsgSize(size int) ServerOption {
	return func(s *GRPCServer) {
		s.maxRecvMsgSize = size
	}
}

// WithMaxSendMsgSize sets the maximum message size the server can send.
func WithMaxSendMsgSize(size int) ServerOption {
	return func(s *GRPCServer) {
		s.maxSendMsgSize = size
	}
}

// WithShutdownTimeout sets the timeout for graceful server shutdown.
func WithShutdownTimeout(timeout time.Duration) ServerOption {
	return func(s *GRPCServer) {
		s.shutdownTimeout = timeout
	}
}

// NewGRPCServer creates a new gRPC ABCI server with the given options.
func NewGRPCServer(protoAddr string, app types.ABCIApplicationServer, opts ...ServerOption) (service.Service, error) {
	if app == nil {
		return nil, ErrNilApplication
	}

	proto, addr := cmtnet.ProtocolAndAddress(protoAddr)
	if addr == "" {
		return nil, ErrInvalidAddress
	}

	s := &GRPCServer{
		proto:            proto,
		addr:             addr,
		app:              app,
		maxRecvMsgSize:   defaultMaxRecvMsgSize,
		maxSendMsgSize:   defaultMaxSendMsgSize,
		shutdownTimeout:  defaultShutdownTimeout,
		keepAliveTime:    defaultKeepAliveTime,
		keepAliveTimeout: defaultKeepAliveTimeout,
		done:             make(chan struct{}),
	}

	// Apply custom options
	for _, opt := range opts {
		opt(s)
	}

	s.BaseService = *service.NewBaseService(nil, "ABCIServer", s)
	return s, nil
}

// OnStart implements service.Service.
func (s *GRPCServer) OnStart() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return ErrServerAlreadyStarted
	}

	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	serverOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.maxRecvMsgSize),
		grpc.MaxSendMsgSize(s.maxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    s.keepAliveTime,
			Timeout: s.keepAliveTimeout,
		}),
		grpc.ReadBufferSize(defaultReadBufferSize),
		grpc.WriteBufferSize(defaultWriteBufferSize),
	}

	s.server = grpc.NewServer(serverOpts...)
	types.RegisterABCIApplicationServer(s.server, s.app)
	s.listener = ln

	s.Logger.Info("Starting gRPC server", 
		"proto", s.proto,
		"addr", s.addr,
		"maxRecvMsgSize", s.maxRecvMsgSize,
		"maxSendMsgSize", s.maxSendMsgSize,
	)

	go s.serve()

	return nil
}

// OnStop implements service.Service.
func (s *GRPCServer) OnStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.Logger.Info("Server stopped gracefully")
	case <-time.After(s.shutdownTimeout):
		s.Logger.Info("Server force stopped due to shutdown timeout")
		s.server.Stop()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	close(s.done)
}

// serve runs the gRPC server and handles errors
func (s *GRPCServer) serve() {
	if err := s.server.Serve(s.listener); err != nil {
		select {
		case <-s.done:
			// Normal shutdown, ignore error
		default:
			s.Logger.Error("Error serving gRPC server", "err", err)
		}
	}
}

// Wait blocks until the server is stopped
func (s *GRPCServer) Wait() {
	<-s.done
}

// GetAddr returns the server's address
func (s *GRPCServer) GetAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}
