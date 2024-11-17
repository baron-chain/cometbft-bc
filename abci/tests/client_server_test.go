package tests

import (
    "context"
    "fmt"
    "net"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    bclient "github.com/baron-chain/cometbft-bc/abci/client"
    "github.com/baron-chain/cometbft-bc/abci/example/kvstore"
    bserver "github.com/baron-chain/cometbft-bc/abci/server"
    "github.com/baron-chain/cometbft-bc/abci/types"
)

const (
    testTimeout  = 10 * time.Second
    defaultPort  = 26658
    defaultHost  = "localhost"
    bcTransport  = "socket"
    dialTimeout  = 10 * time.Millisecond
)

// BCTestEnv represents Baron Chain test environment
type BCTestEnv struct {
    t         *testing.T
    app       types.Application
    server    bserver.Service
    client    bclient.Client
    transport string
    addr      string
    ctx       context.Context
    cancel    context.CancelFunc
}

// BCTestOption configures test environment
type BCTestOption func(*BCTestEnv)

// Create new Baron Chain test environment
func newBCTestEnv(t *testing.T, opts ...BCTestOption) *BCTestEnv {
    t.Helper()

    ctx, cancel := context.WithTimeout(context.Background(), testTimeout)

    env := &BCTestEnv{
        t:         t,
        app:       kvstore.NewApplication(),
        transport: bcTransport,
        addr:      fmt.Sprintf("%s:%d", defaultHost, defaultPort),
        ctx:       ctx,
        cancel:    cancel,
    }

    for _, opt := range opts {
        opt(env)
    }

    return env
}

// Test environment configuration options
func withBCTransport(transport string) BCTestOption {
    return func(env *BCTestEnv) {
        env.transport = transport
    }
}

func withBCAddress(addr string) BCTestOption {
    return func(env *BCTestEnv) {
        env.addr = addr
    }
}

func withBCApplication(app types.Application) BCTestOption {
    return func(env *BCTestEnv) {
        env.app = app
    }
}

// Start Baron Chain test environment
func (env *BCTestEnv) start() {
    env.t.Helper()

    server, err := bserver.NewServer(env.addr, env.transport, env.app)
    require.NoError(env.t, err, "failed to create Baron Chain server")
    env.server = server

    require.NoError(env.t, server.Start(), "failed to start Baron Chain server")

    // Wait for server readiness
    require.Eventually(env.t, 
        func() bool {
            conn, err := net.DialTimeout("tcp", env.addr, dialTimeout)
            if err != nil {
                return false
            }
            conn.Close()
            return true
        },
        testTimeout,
        dialTimeout,
        "Baron Chain server not ready",
    )

    // Initialize client
    client, err := bclient.NewClient(env.addr, env.transport, true)
    require.NoError(env.t, err, "failed to create Baron Chain client")
    env.client = client

    require.NoError(env.t, client.Start(), "failed to start Baron Chain client")
}

// Stop Baron Chain test environment
func (env *BCTestEnv) stop() {
    env.t.Helper()

    if env.client != nil {
        assert.NoError(env.t, env.client.Stop(), "failed to stop Baron Chain client")
    }

    if env.server != nil {
        assert.NoError(env.t, env.server.Stop(), "failed to stop Baron Chain server")
    }

    env.cancel()
}

func TestBaronChainClientServer(t *testing.T) {
    testCases := []struct {
        name      string
        transport string
        addr      string
    }{
        {
            name:      "Socket Direct",
            transport: "socket",
            addr:      "localhost:26658",
        },
        {
            name:      "Socket TCP",
            transport: "socket", 
            addr:      "tcp://localhost:26659",
        },
        {
            name:      "GRPC Direct",
            transport: "grpc",
            addr:      "localhost:26660",
        },
        {
            name:      "GRPC TCP",
            transport: "grpc",
            addr:      "tcp://localhost:26661",
        },
    }

    for _, tc := range testCases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            env := newBCTestEnv(t,
                withBCTransport(tc.transport),
                withBCAddress(tc.addr),
            )
            defer env.stop()

            env.start()
            testBCConnectivity(t, env)
        })
    }
}

func testBCConnectivity(t *testing.T, env *BCTestEnv) {
    t.Helper()

    // Test Info
    res, err := env.client.InfoSync(types.RequestInfo{Version: "1.0.0"})
    require.NoError(t, err, "Baron Chain Info request failed")
    assert.NotNil(t, res, "Baron Chain Info response empty")

    // Test Echo
    msg := "Baron Chain Test"
    res2, err := env.client.EchoSync(msg)
    require.NoError(t, err, "Baron Chain Echo request failed")
    assert.Equal(t, msg, res2.Message, "Baron Chain Echo response mismatch")

    // Test Flush
    require.NoError(t, env.client.FlushSync(), "Baron Chain Flush failed")
}

func TestBaronChainErrors(t *testing.T) {
    testCases := []struct {
        name        string
        transport   string
        addr        string
        expectError string
    }{
        {
            name:        "Invalid Transport",
            transport:   "invalid",
            addr:       "localhost:26658",
            expectError: "unknown server type invalid",
        },
        {
            name:        "Invalid Address",
            transport:   "socket",
            addr:        "invalid:address",
            expectError: "listen tcp: address invalid:address: missing port in address",
        },
    }

    for _, tc := range testCases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            _, err := bserver.NewServer(tc.addr, tc.transport, kvstore.NewApplication())
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tc.expectError)
        })
    }
}

func TestBaronChainConcurrency(t *testing.T) {
    env := newBCTestEnv(t)
    defer env.stop()

    env.start()

    const numRoutines = 10
    const numRequests = 100

    errors := make(chan error, numRoutines*numRequests)

    for i := 0; i < numRoutines; i++ {
        go func() {
            for j := 0; j < numRequests; j++ {
                msg := fmt.Sprintf("baron-test-%d", j)
                _, err := env.client.EchoSync(msg)
                errors <- err
            }
        }()
    }

    for i := 0; i < numRoutines*numRequests; i++ {
        select {
        case err := <-errors:
            assert.NoError(t, err, "Baron Chain concurrent request failed")
        case <-time.After(testTimeout):
            t.Fatal("Baron Chain concurrent test timeout")
        }
    }
}
