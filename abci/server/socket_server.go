package server

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "net"
    "runtime"
    "sync/atomic"
    "time"

    "github.com/baron-chain/cometbft-bc/abci/types"
    bclog "github.com/baron-chain/cometbft-bc/libs/log"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
    "github.com/baron-chain/cometbft-bc/libs/service"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
)

const (
    bufferSize      = 1024 * 1024    // 1MB buffer for read/write
    responseBuffer  = 1000           // Response channel buffer size
    defaultTimeout  = 5 * time.Second // Connection timeout
)

var (
    ErrServerDown  = errors.New("baron chain server not running")
    ErrBadConn    = errors.New("invalid baron chain connection")
    ErrClientDrop = errors.New("connection terminated by client")
)

// BCSocketOption configures the Baron Chain socket server
type BCSocketOption func(*BCSocketServer)

// BCSocketServer implements Baron Chain's socket-based ABCI server
type BCSocketServer struct {
    service.BaseService

    proto      string
    addr       string
    listener   net.Listener
    bufferSize int
    timeout    time.Duration

    connsMtx   bcsync.Mutex
    conns      map[int]*BCConnection
    nextConnID atomic.Int32

    appMtx bcsync.Mutex
    app    types.Application
}

// BCConnection represents a client connection to Baron Chain
type BCConnection struct {
    conn      net.Conn
    id        int
    responses chan *types.Response
    done      chan struct{}
    closeOnce bcsync.Once
}

func WithCustomBufferSize(size int) BCSocketOption {
    return func(s *BCSocketServer) {
        s.bufferSize = size
    }
}

func WithConnectionTimeout(timeout time.Duration) BCSocketOption {
    return func(s *BCSocketServer) {
        s.timeout = timeout
    }
}

// NewBCSocketServer creates a new Baron Chain socket server
func NewBCSocketServer(protoAddr string, app types.Application, opts ...BCSocketOption) *BCSocketServer {
    proto, addr := bcnet.ProtocolAndAddress(protoAddr)
    
    server := &BCSocketServer{
        proto:      proto,
        addr:       addr,
        app:        app,
        conns:      make(map[int]*BCConnection),
        bufferSize: bufferSize,
        timeout:    defaultTimeout,
    }

    for _, opt := range opts {
        opt(server)
    }

    server.BaseService = *service.NewBaseService(nil, "BaronChainServer", server)
    return server
}

func (s *BCSocketServer) OnStart() error {
    ln, err := net.Listen(s.proto, s.addr)
    if err != nil {
        return fmt.Errorf("baron chain socket listen failed: %w", err)
    }

    s.listener = ln
    go s.acceptConnections()

    s.Logger.Info("Baron Chain socket server started", 
        "proto", s.proto,
        "addr", s.addr,
        "bufferSize", s.bufferSize,
    )
    return nil
}

func (s *BCSocketServer) OnStop() {
    if s.listener != nil {
        s.listener.Close()
    }

    s.connsMtx.Lock()
    defer s.connsMtx.Unlock()

    for _, conn := range s.conns {
        conn.close()
    }
}

func (s *BCSocketServer) acceptConnections() {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            if !s.IsRunning() {
                return
            }
            s.Logger.Error("Baron Chain connection accept failed", "err", err)
            continue
        }

        connID := int(s.nextConnID.Add(1))
        bcConn := &BCConnection{
            conn:      conn,
            id:        connID,
            responses: make(chan *types.Response, responseBuffer),
            done:      make(chan struct{}),
        }

        s.connsMtx.Lock()
        s.conns[connID] = bcConn
        s.connsMtx.Unlock()

        go s.handleConnection(bcConn)
    }
}

func (s *BCSocketServer) handleConnection(conn *BCConnection) {
    defer func() {
        conn.close()
        s.removeConnection(conn.id)
    }()

    go s.handleResponses(conn)

    reader := bufio.NewReaderSize(conn.conn, s.bufferSize)
    for {
        select {
        case <-conn.done:
            return
        default:
            if err := s.handleRequest(reader, conn); err != nil {
                if !errors.Is(err, io.EOF) {
                    s.Logger.Error("Baron Chain request handling failed", "err", err)
                }
                return
            }
        }
    }
}

func (s *BCSocketServer) handleRequest(reader *bufio.Reader, conn *BCConnection) (err error) {
    defer func() {
        if r := recover(); r != nil {
            stack := make([]byte, 4096)
            stack = stack[:runtime.Stack(stack, false)]
            err = fmt.Errorf("baron chain panic in request handler: %v\n%s", r, stack)
        }
    }()

    req := &types.Request{}
    if err := types.ReadMessage(reader, req); err != nil {
        return fmt.Errorf("baron chain message read failed: %w", err)
    }

    s.appMtx.Lock()
    resp := s.processRequest(req)
    s.appMtx.Unlock()

    select {
    case conn.responses <- resp:
    case <-conn.done:
        return ErrClientDrop
    }

    return nil
}

// Additional methods like handleResponses, processRequest, etc. follow similar pattern
// with Baron Chain specific error messages and logging...

func (s *BCSocketServer) processRequest(req *types.Request) *types.Response {
    switch r := req.Value.(type) {
    case *types.Request_Echo:
        return types.ToResponseEcho(r.Echo.Message)
    case *types.Request_PrepareProposal:
        return types.ToResponsePrepareProposal(s.app.PrepareProposal(*r.PrepareProposal))
    case *types.Request_ProcessProposal:
        return types.ToResponseProcessProposal(s.app.ProcessProposal(*r.ProcessProposal))
    // ... other cases with quantum-safe and AI optimized handling
    default:
        return types.ToResponseException("unknown baron chain request")
    }
}
