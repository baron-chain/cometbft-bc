package example

import (
    "context"
    "fmt"
    "math/rand"
    "net"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"

    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    "github.com/baron-chain/cometbft-bc/abci/example/code"
    "github.com/baron-chain/cometbft-bc/abci/example/kvstore"
    bcserver "github.com/baron-chain/cometbft-bc/abci/server"
    "github.com/baron-chain/cometbft-bc/abci/types"
)

const (
    socketFileFormat   = "baron-test-%08x.sock"
    socketPathFormat   = "unix://%v"
    grpcTimeout       = 5 * time.Second
    testTimeout       = 30 * time.Second
    streamTxCount     = 20000
    grpcTxCount      = 2000
    flushInterval     = 100
)

type testEnv struct {
    t       *testing.T
    app     types.Application
    server  service
    client  abcicli.Client
    socket  string
    cleanup func()
}

type service interface {
    Start() error
    Stop() error
}

func init() {
    rand.Seed(time.Now().UnixNano())
}

func setupTestEnv(t *testing.T, app types.Application, useGRPC bool) *testEnv {
    socketFile := fmt.Sprintf(socketFileFormat, rand.Int31n(1<<30))
    socket := fmt.Sprintf(socketPathFormat, socketFile)
    
    env := &testEnv{
        t:       t,
        app:     app,
        socket:  socket,
        cleanup: func() { os.Remove(socketFile) },
    }
    t.Cleanup(env.cleanup)

    if useGRPC {
        env.setupGRPC()
    } else {
        env.setupSocket()
    }

    return env
}

func (env *testEnv) setupSocket() {
    logger := log.TestingLogger().With("module", "baron-server")
    server := bcserver.NewSocketServer(env.socket, env.app)
    server.SetLogger(logger)
    require.NoError(env.t, server.Start())
    env.t.Cleanup(func() {
        require.NoError(env.t, server.Stop())
    })

    client := abcicli.NewSocketClient(env.socket, false)
    client.SetLogger(logger.With("component", "client"))
    require.NoError(env.t, client.Start())
    env.t.Cleanup(func() {
        require.NoError(env.t, client.Stop())
    })

    env.server = server
    env.client = client
}

func (env *testEnv) setupGRPC() {
    logger := log.TestingLogger().With("module", "baron-grpc")
    server := bcserver.NewGRPCServer(env.socket, env.app)
    server.SetLogger(logger)
    require.NoError(env.t, server.Start())
    env.t.Cleanup(func() {
        require.NoError(env.t, server.Stop())
    })
    env.server = server
}

func TestKVStoreIntegration(t *testing.T) {
    t.Parallel()
    testStreamDelivery(t, kvstore.NewApplication())
}

func TestBaseAppIntegration(t *testing.T) {
    t.Parallel()
    testStreamDelivery(t, types.NewBaseApplication())
}

func TestGRPCIntegration(t *testing.T) {
    t.Parallel()
    testGRPCDelivery(t, types.NewGRPCApplication(types.NewBaseApplication()))
}

func testStreamDelivery(t *testing.T, app types.Application) {
    env := setupTestEnv(t, app, false)
    done := make(chan struct{})
    txCount := 0

    env.client.SetResponseCallback(func(req *types.Request, res *types.Response) {
        switch r := res.Value.(type) {
        case *types.Response_DeliverTx:
            txCount++
            require.Equal(t, code.CodeTypeOK, r.DeliverTx.Code)
            require.LessOrEqual(t, txCount, streamTxCount)
            
            if txCount == streamTxCount {
                time.AfterFunc(time.Second, func() { close(done) })
            }
        case *types.Response_Flush:
            // Expected flush response
        default:
            t.Errorf("unexpected response type %T", res.Value)
        }
    })

    // Send transactions
    for i := 0; i < streamTxCount; i++ {
        reqRes := env.client.DeliverTxAsync(types.RequestDeliverTx{Tx: []byte("test")})
        require.NotNil(t, reqRes)

        if i%flushInterval == 0 {
            env.client.FlushAsync()
        }
    }
    env.client.FlushAsync()
    
    select {
    case <-done:
    case <-time.After(testTimeout):
        t.Fatal("test timeout")
    }
}

func testGRPCDelivery(t *testing.T, app types.ABCIApplicationServer) {
    env := setupTestEnv(t, app.(types.Application), true)

    ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
    defer cancel()

    conn, err := grpc.DialContext(ctx, env.socket,
        grpc.WithInsecure(),
        grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
            return bcnet.Connect(addr)
        }),
    )
    require.NoError(t, err)
    defer conn.Close()

    client := types.NewABCIApplicationClient(conn)

    for i := 0; i < grpcTxCount; i++ {
        resp, err := client.DeliverTx(ctx, &types.RequestDeliverTx{Tx: []byte("test")})
        require.NoError(t, err)
        require.Equal(t, code.CodeTypeOK, resp.Code)
    }
}
