package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/cometbft/cometbft/libs/log"
	rs "github.com/cometbft/cometbft/rpc/jsonrpc/server"
	types "github.com/cometbft/cometbft/rpc/jsonrpc/types"
)

const (
	testEndpoint = "http://localhost/"
	methodPost   = "POST"
)

// TestRPCFunc is a simple RPC function for testing
func TestRPCFunc(s string, i int) (string, int) {
	return "foo", 200
}

// RPCServer encapsulates the RPC server configuration and dependencies
type RPCServer struct {
	mux    *http.ServeMux
	logger log.Logger
}

// NewRPCServer creates and initializes a new RPCServer instance
func NewRPCServer() (*RPCServer, error) {
	// Initialize buffer for logging
	logBuffer := new(bytes.Buffer)
	logger := log.NewTMLogger(logBuffer)

	// Create and configure server
	server := &RPCServer{
		mux:    http.NewServeMux(),
		logger: logger,
	}

	// Register RPC functions
	rpcFuncs := map[string]*rs.RPCFunc{
		"c": rs.NewRPCFunc(TestRPCFunc, "s,i"),
	}

	if err := server.registerHandlers(rpcFuncs); err != nil {
		return nil, fmt.Errorf("failed to register RPC handlers: %w", err)
	}

	return server, nil
}

// registerHandlers registers the RPC functions with the server's multiplexer
func (s *RPCServer) registerHandlers(funcs map[string]*rs.RPCFunc) error {
	if s.mux == nil {
		return fmt.Errorf("server multiplexer not initialized")
	}
	rs.RegisterRPCFuncs(s.mux, funcs, s.logger)
	return nil
}

// handleRequest processes an HTTP request and returns the response
func (s *RPCServer) handleRequest(data []byte) (*types.RPCResponse, error) {
	// Create test request
	req, err := http.NewRequest(methodPost, testEndpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create response recorder
	rec := httptest.NewRecorder()

	// Process request
	s.mux.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	response := new(types.RPCResponse)
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

// Global server instance
var server *RPCServer

// init initializes the global server instance
func init() {
	var err error
	server, err = NewRPCServer()
	if err != nil {
		// In init(), we can't return an error, so we panic
		panic(fmt.Sprintf("failed to initialize RPC server: %v", err))
	}
}

// Fuzz implements the fuzzing entrypoint
func Fuzz(data []byte) int {
	response, err := server.handleRequest(data)
	if err != nil {
		// For fuzzing, we return 0 to indicate uninteresting input
		return 0
	}

	// Verify response is valid
	if response == nil {
		return 0
	}

	// Return 1 to indicate interesting input that should be added to the corpus
	return 1
}
