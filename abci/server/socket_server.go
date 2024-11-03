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

	"github.com/cometbft/cometbft/abci/types"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	cmtnet "github.com/cometbft/cometbft/libs/net"
	"github.com/cometbft/cometbft/libs/service"
	cmtsync "github.com/cometbft/cometbft/libs/sync"
)

const (
	defaultReadBufferSize  = 1024 * 1024    // 1MB
	defaultWriteBufferSize = 1024 * 1024    // 1MB
	defaultResponseBuffer  = 1000
	defaultConnTimeout     = 5 * time.Second
)

var (
	ErrServerNotRunning = errors.New("server not running")
	ErrInvalidConn     = errors.New("invalid connection")
	ErrConnClosed      = errors.New("connection closed by client")
)

// SocketOption configures the socket server
type SocketOption func(*SocketServer)

// SocketServer represents a socket-based ABCI server
type SocketServer struct {
	service.BaseService

	proto      string
	addr       string
	listener   net.Listener
	bufferSize int
	timeout    time.Duration

	connsMtx   cmtsync.Mutex
	conns      map[int]*Connection
	nextConnID atomic.Int32

	appMtx cmtsync.Mutex
	app    types.Application
}

// Connection represents a client connection
type Connection struct {
	conn      net.Conn
	id        int
	responses chan *types.Response
	done      chan struct{}
	closeOnce cmtsync.Once
}

// WithBufferSize sets the read/write buffer size
func WithBufferSize(size int) SocketOption {
	return func(s *SocketServer) {
		s.bufferSize = size
	}
}

// WithTimeout sets the connection timeout
func WithTimeout(timeout time.Duration) SocketOption {
	return func(s *SocketServer) {
		s.timeout = timeout
	}
}

// NewSocketServer creates a new socket-based ABCI server
func NewSocketServer(protoAddr string, app types.Application, opts ...SocketOption) *SocketServer {
	proto, addr := cmtnet.ProtocolAndAddress(protoAddr)
	
	s := &SocketServer{
		proto:      proto,
		addr:       addr,
		app:        app,
		conns:      make(map[int]*Connection),
		bufferSize: defaultReadBufferSize,
		timeout:    defaultConnTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.BaseService = *service.NewBaseService(nil, "ABCIServer", s)
	return s
}

// OnStart implements service.Service
func (s *SocketServer) OnStart() error {
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s://%s: %w", s.proto, s.addr, err)
	}

	s.listener = ln
	go s.acceptConnections()

	s.Logger.Info("Started socket server", 
		"proto", s.proto,
		"addr", s.addr,
		"bufferSize", s.bufferSize,
	)
	return nil
}

// OnStop implements service.Service
func (s *SocketServer) OnStop() {
	if s.listener != nil {
		s.listener.Close()
	}

	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()

	for _, conn := range s.conns {
		conn.close()
	}
}

func (s *SocketServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.IsRunning() {
				return
			}
			s.Logger.Error("Failed to accept connection", "err", err)
			continue
		}

		connID := int(s.nextConnID.Add(1))
		connection := &Connection{
			conn:      conn,
			id:        connID,
			responses: make(chan *types.Response, defaultResponseBuffer),
			done:      make(chan struct{}),
		}

		s.connsMtx.Lock()
		s.conns[connID] = connection
		s.connsMtx.Unlock()

		go s.handleConnection(connection)
	}
}

func (s *SocketServer) handleConnection(conn *Connection) {
	defer func() {
		conn.close()
		s.removeConnection(conn.id)
	}()

	// Start response handler
	go s.handleResponses(conn)

	// Handle requests
	reader := bufio.NewReaderSize(conn.conn, s.bufferSize)
	for {
		select {
		case <-conn.done:
			return
		default:
			if err := s.handleRequest(reader, conn); err != nil {
				if !errors.Is(err, io.EOF) {
					s.Logger.Error("Error handling request", "err", err)
				}
				return
			}
		}
	}
}

func (s *SocketServer) handleRequest(reader *bufio.Reader, conn *Connection) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			err = fmt.Errorf("panic in request handler: %v\n%s", r, stack)
		}
	}()

	req := &types.Request{}
	if err := types.ReadMessage(reader, req); err != nil {
		return fmt.Errorf("error reading message: %w", err)
	}

	s.appMtx.Lock()
	resp := s.processRequest(req)
	s.appMtx.Unlock()

	select {
	case conn.responses <- resp:
	case <-conn.done:
		return ErrConnClosed
	}

	return nil
}

func (s *SocketServer) handleResponses(conn *Connection) {
	writer := bufio.NewWriterSize(conn.conn, s.bufferSize)
	
	for {
		select {
		case <-conn.done:
			return
		case res := <-conn.responses:
			if err := s.writeResponse(writer, res); err != nil {
				s.Logger.Error("Error writing response", "err", err)
				return
			}
		}
	}
}

func (s *SocketServer) writeResponse(writer *bufio.Writer, res *types.Response) error {
	if err := types.WriteMessage(res, writer); err != nil {
		return fmt.Errorf("error writing message: %w", err)
	}

	if _, ok := res.Value.(*types.Response_Flush); ok {
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("error flushing writer: %w", err)
		}
	}

	return nil
}

func (s *SocketServer) processRequest(req *types.Request) *types.Response {
	switch r := req.Value.(type) {
	case *types.Request_Echo:
		return types.ToResponseEcho(r.Echo.Message)
	case *types.Request_Flush:
		return types.ToResponseFlush()
	case *types.Request_Info:
		return types.ToResponseInfo(s.app.Info(*r.Info))
	case *types.Request_DeliverTx:
		return types.ToResponseDeliverTx(s.app.DeliverTx(*r.DeliverTx))
	case *types.Request_CheckTx:
		return types.ToResponseCheckTx(s.app.CheckTx(*r.CheckTx))
	case *types.Request_Commit:
		return types.ToResponseCommit(s.app.Commit())
	case *types.Request_Query:
		return types.ToResponseQuery(s.app.Query(*r.Query))
	case *types.Request_InitChain:
		return types.ToResponseInitChain(s.app.InitChain(*r.InitChain))
	case *types.Request_BeginBlock:
		return types.ToResponseBeginBlock(s.app.BeginBlock(*r.BeginBlock))
	case *types.Request_EndBlock:
		return types.ToResponseEndBlock(s.app.EndBlock(*r.EndBlock))
	case *types.Request_ListSnapshots:
		return types.ToResponseListSnapshots(s.app.ListSnapshots(*r.ListSnapshots))
	case *types.Request_OfferSnapshot:
		return types.ToResponseOfferSnapshot(s.app.OfferSnapshot(*r.OfferSnapshot))
	case *types.Request_PrepareProposal:
		return types.ToResponsePrepareProposal(s.app.PrepareProposal(*r.PrepareProposal))
	case *types.Request_ProcessProposal:
		return types.ToResponseProcessProposal(s.app.ProcessProposal(*r.ProcessProposal))
	case *types.Request_LoadSnapshotChunk:
		return types.ToResponseLoadSnapshotChunk(s.app.LoadSnapshotChunk(*r.LoadSnapshotChunk))
	case *types.Request_ApplySnapshotChunk:
		return types.ToResponseApplySnapshotChunk(s.app.ApplySnapshotChunk(*r.ApplySnapshotChunk))
	default:
		return types.ToResponseException("unknown request")
	}
}

func (s *SocketServer) removeConnection(id int) {
	s.connsMtx.Lock()
	defer s.connsMtx.Unlock()
	delete(s.conns, id)
}

func (conn *Connection) close() {
	conn.closeOnce.Do(func() {
		conn.conn.Close()
		close(conn.done)
		close(conn.responses)
	})
}
