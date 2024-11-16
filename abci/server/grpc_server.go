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

    "github.com/baron-chain/cometbft-bc/abci/types"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
    "github.com/baron-chain/cometbft-bc/libs/service"
)

const (
    // Server configuration defaults
    defaultBufferSize     = 1024 * 1024      // 1MB for read/write buffers
    defaultMaxMsgSize     = 10 * 1024 * 1024 // 10MB for recv/send
    defaultGracePeriod    = 5 * time.Second
    defaultKeepAliveTime  = 5 * time.Minute
    defaultKeepAliveWait  = 20 * time.Second
)

var (
    ErrServerNotInit   = errors.New("baron chain server not initialized")
    ErrServerRunning   = errors.New("baron chain server already running")
    ErrInvalidAddr     = errors.New("invalid baron chain server address")
    ErrNilApp         = errors.New("baron chain application cannot be nil")
)

type ServerConfig func(*BCGRPCServer)

// BCGRPCServer represents Baron Chain's gRPC ABCI server instance
type BCGRPCServer struct {
    service.BaseService

    proto    string
    addr     string
    app      types.ABCIApplicationServer
    server   *grpc.Server
    listener net.Listener

    maxRecvSize      int
    maxSendSize      int
    gracePeriod      time.Duration
    keepAliveTime    time.Duration
    keepAliveTimeout time.Duration

    mu   sync.RWMutex
    done chan struct{}
}

// Configure server message size limits
func WithMaxMessageSize(recv, send int) ServerConfig {
    return func(s *BCGRPCServer) {
        s.maxRecvSize = recv
        s.maxSendSize = send
    }
}

// Configure server shutdown grace period
func WithGracePeriod(period time.Duration) ServerConfig {
    return func(s *BCGRPCServer) {
        s.gracePeriod = period
    }
}

// NewBCGRPCServer creates a new Baron Chain gRPC server
func NewBCGRPCServer(protoAddr string, app types.ABCIApplicationServer, configs ...ServerConfig) (service.Service, error) {
    if app == nil {
        return nil, ErrNilApp
    }

    proto, addr := bcnet.ProtocolAndAddress(protoAddr)
    if addr == "" {
        return nil, ErrInvalidAddr
    }

    srv := &BCGRPCServer{
        proto:           proto,
        addr:            addr,
        app:             app,
        maxRecvSize:     defaultMaxMsgSize,
        maxSendSize:     defaultMaxMsgSize,
        gracePeriod:     defaultGracePeriod,
        keepAliveTime:   defaultKeepAliveTime,
        keepAliveTimeout: defaultKeepAliveWait,
        done:            make(chan struct{}),
    }

    for _, config := range configs {
        config(srv)
    }

    srv.BaseService = *service.NewBaseService(nil, "BaronChainServer", srv)
    return srv, nil
}

// OnStart implements service.Service
func (s *BCGRPCServer) OnStart() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.listener != nil {
        return ErrServerRunning
    }

    ln, err := net.Listen(s.proto, s.addr)
    if err != nil {
        return fmt.Errorf("baron chain server listen failed: %w", err)
    }

    opts := []grpc.ServerOption{
        grpc.MaxRecvMsgSize(s.maxRecvSize),
        grpc.MaxSendMsgSize(s.maxSendSize),
        grpc.KeepaliveParams(keepalive.ServerParameters{
            Time:    s.keepAliveTime,
            Timeout: s.keepAliveTimeout,
        }),
        grpc.ReadBufferSize(defaultBufferSize),
        grpc.WriteBufferSize(defaultBufferSize),
    }

    s.server = grpc.NewServer(opts...)
    types.RegisterABCIApplicationServer(s.server, s.app)
    s.listener = ln

    s.Logger.Info("Starting Baron Chain gRPC server",
        "proto", s.proto,
        "addr", s.addr,
        "maxRecvSize", s.maxRecvSize,
        "maxSendSize", s.maxSendSize,
    )

    go s.serveRequests()

    return nil
}

// OnStop implements service.Service
func (s *BCGRPCServer) OnStop() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.server == nil {
        return
    }

    graceful := make(chan struct{})
    go func() {
        s.server.GracefulStop()
        close(graceful)
    }()

    select {
    case <-graceful:
        s.Logger.Info("Baron Chain server stopped gracefully")
    case <-time.After(s.gracePeriod):
        s.Logger.Info("Baron Chain server force stopped after timeout")
        s.server.Stop()
    }

    if s.listener != nil {
        s.listener.Close()
    }

    close(s.done)
}

func (s *BCGRPCServer) serveRequests() {
    if err := s.server.Serve(s.listener); err != nil {
        select {
        case <-s.done:
            // Normal shutdown
        default:
            s.Logger.Error("Baron Chain gRPC server error", "err", err)
        }
    }
}

// GetServerAddr returns the server's current address
func (s *BCGRPCServer) GetServerAddr() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    if s.listener != nil {
        return s.listener.Addr().String()
    }
    return s.addr
}

// WaitForShutdown blocks until server stops
func (s *BCGRPCServer) WaitForShutdown() {
    <-s.done
}
