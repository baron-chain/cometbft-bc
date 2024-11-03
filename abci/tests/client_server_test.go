package tests

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abciclient "github.com/cometbft/cometbft/abci/client"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	abciserver "github.com/cometbft/cometbft/abci/server"
	"github.com/cometbft/cometbft/abci/types"
)

const (
	testTimeout      = 10 * time.Second
	defaultPort      = 26658
	defaultHost      = "localhost"
	defaultTransport = "socket"
)

type testSetup struct {
	t         *testing.T
	app       types.Application
	server    abciserver.Service
	client    abciclient.Client
	transport string
	addr      string
	ctx       context.Context
	cancel    context.CancelFunc
}

func newTestSetup(t *testing.T, opts ...testOption) *testSetup {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

	ts := &testSetup{
		t:         t,
		app:       kvstore.NewApplication(),
		transport: defaultTransport,
		addr:      fmt.Sprintf("%s:%d", defaultHost, defaultPort),
		ctx:       ctx,
		cancel:    cancel,
	}

	for _, opt := range opts {
		opt(ts)
	}

	return ts
}

type testOption func(*testSetup)

func withTransport(transport string) testOption {
	return func(ts *testSetup) {
		ts.transport = transport
	}
}

func withAddress(addr string) testOption {
	return func(ts *testSetup) {
		ts.addr = addr
	}
}

func withApplication(app types.Application) testOption {
	return func(ts *testSetup) {
		ts.app = app
	}
}

func (ts *testSetup) start() {
	ts.t.Helper()

	// Start server
	server, err := abciserver.NewServer(ts.addr, ts.transport, ts.app)
	require.NoError(ts.t, err, "creating server")
	ts.server = server

	err = server.Start()
	require.NoError(ts.t, err, "starting server")

	// Wait for server to be ready
	require.Eventually(ts.t, func() bool {
		conn, err := net.Dial("tcp", ts.addr)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, testTimeout, 10*time.Millisecond, "waiting for server")

	// Start client
	client, err := abciclient.NewClient(ts.addr, ts.transport, true)
	require.NoError(ts.t, err, "creating client")
	ts.client = client

	err = client.Start()
	require.NoError(ts.t, err, "starting client")
}

func (ts *testSetup) stop() {
	ts.t.Helper()

	if ts.client != nil {
		err := ts.client.Stop()
		assert.NoError(ts.t, err, "stopping client")
	}

	if ts.server != nil {
		err := ts.server.Stop()
		assert.NoError(ts.t, err, "stopping server")
	}

	ts.cancel()
}

func TestClientServer(t *testing.T) {
	testCases := []struct {
		name      string
		transport string
		addr      string
	}{
		{
			name:      "Socket transport with no prefix",
			transport: "socket",
			addr:      "localhost:26658",
		},
		{
			name:      "Socket transport with tcp prefix",
			transport: "socket",
			addr:      "tcp://localhost:26659",
		},
		{
			name:      "GRPC transport with no prefix",
			transport: "grpc",
			addr:      "localhost:26660",
		},
		{
			name:      "GRPC transport with tcp prefix",
			transport: "grpc",
			addr:      "tcp://localhost:26661",
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestSetup(t,
				withTransport(tc.transport),
				withAddress(tc.addr),
			)
			defer ts.stop()

			ts.start()

			// Test basic connectivity
			testBasicConnectivity(t, ts)
		})
	}
}

func testBasicConnectivity(t *testing.T, ts *testSetup) {
	t.Helper()

	// Test Info call
	res, err := ts.client.InfoSync(types.RequestInfo{Version: "1.0.0"})
	require.NoError(t, err, "InfoSync call")
	assert.NotNil(t, res, "InfoSync response")

	// Test Echo call
	testMessage := "Hello ABCI"
	res2, err := ts.client.EchoSync(testMessage)
	require.NoError(t, err, "EchoSync call")
	assert.Equal(t, testMessage, res2.Message, "Echo response")

	// Test Flush
	err = ts.client.FlushSync()
	require.NoError(t, err, "FlushSync call")
}

func TestClientServerErrors(t *testing.T) {
	testCases := []struct {
		name        string
		transport   string
		addr        string
		expectedErr string
	}{
		{
			name:        "Invalid transport",
			transport:   "invalid",
			addr:        "localhost:26658",
			expectedErr: "unknown server type invalid",
		},
		{
			name:        "Invalid address",
			transport:   "socket",
			addr:        "invalid:address",
			expectedErr: "listen tcp: address invalid:address: missing port in address",
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := abciserver.NewServer(tc.addr, tc.transport, kvstore.NewApplication())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestClientServerConcurrency(t *testing.T) {
	ts := newTestSetup(t)
	defer ts.stop()

	ts.start()

	const numGoroutines = 10
	const numRequests = 100

	errCh := make(chan error, numGoroutines*numRequests)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numRequests; j++ {
				msg := fmt.Sprintf("test-%d", j)
				_, err := ts.client.EchoSync(msg)
				errCh <- err
			}
		}()
	}

	for i := 0; i < numGoroutines*numRequests; i++ {
		select {
		case err := <-errCh:
			assert.NoError(t, err, "concurrent request")
		case <-time.After(testTimeout):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}
}
