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
	"github.com/cometbft/cometbft/libs/log"
	cmtnet "github.com/cometbft/cometbft/libs/net"

	abcicli "github.com/cometbft/cometbft/abci/client"
	"github.com/cometbft/cometbft/abci/example/code"
	"github.com/cometbft/cometbft/abci/example/kvstore"
	abciserver "github.com/cometbft/cometbft/abci/server"
	"github.com/cometbft/cometbft/abci/types"
)

const (
	socketFileFormat    = "test-%08x.sock"
	socketPathFormat    = "unix://%v"
	defaultGRPCTimeout  = 5 * time.Second
	defaultTestTimeout  = 30 * time.Second
	streamDeliverCount = 20000
	grpcDeliverCount   = 2000
	flushInterval      = 123
)

type testSetup struct {
	t        *testing.T
	app      types.Application
	server   service
	client   abcicli.Client
	socket   string
	cleanup  func()
}

type service interface {
	Start() error
	Stop() error
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func setupTest(t *testing.T, app types.Application, isGRPC bool) *testSetup {
	socketFile := fmt.Sprintf(socketFileFormat, rand.Int31n(1<<30))
	socket := fmt.Sprintf(socketPathFormat, socketFile)
	
	ts := &testSetup{
		t:      t,
		app:    app,
		socket: socket,
	}

	ts.cleanup = func() {
		os.Remove(socketFile)
	}
	t.Cleanup(ts.cleanup)

	if isGRPC {
		ts.setupGRPC()
	} else {
		ts.setupSocket()
	}

	return ts
}

func (ts *testSetup) setupSocket() {
	server := abciserver.NewSocketServer(ts.socket, ts.app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	require.NoError(ts.t, server.Start(), "starting socket server")
	ts.t.Cleanup(func() {
		require.NoError(ts.t, server.Stop(), "stopping socket server")
	})

	client := abcicli.NewSocketClient(ts.socket, false)
	client.SetLogger(log.TestingLogger().With("module", "abci-client"))
	require.NoError(ts.t, client.Start(), "starting socket client")
	ts.t.Cleanup(func() {
		require.NoError(ts.t, client.Stop(), "stopping socket client")
	})

	ts.server = server
	ts.client = client
}

func (ts *testSetup) setupGRPC() {
	server := abciserver.NewGRPCServer(ts.socket, ts.app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	require.NoError(ts.t, server.Start(), "starting GRPC server")
	ts.t.Cleanup(func() {
		require.NoError(ts.t, server.Stop(), "stopping GRPC server")
	})

	ts.server = server
}

func TestKVStore(t *testing.T) {
	t.Parallel()
	testStream(t, kvstore.NewApplication())
}

func TestBaseApp(t *testing.T) {
	t.Parallel()
	testStream(t, types.NewBaseApplication())
}

func TestGRPC(t *testing.T) {
	t.Parallel()
	testGRPCSync(t, types.NewGRPCApplication(types.NewBaseApplication()))
}

func testStream(t *testing.T, app types.Application) {
	ts := setupTest(t, app, false)
	
	done := make(chan struct{})
	counter := 0

	ts.client.SetResponseCallback(func(req *types.Request, res *types.Response) {
		switch r := res.Value.(type) {
		case *types.Response_DeliverTx:
			counter++
			require.Equal(t, code.CodeTypeOK, r.DeliverTx.Code, "DeliverTx failed")
			require.LessOrEqual(t, counter, streamDeliverCount, "Too many DeliverTx responses")
			
			if counter == streamDeliverCount {
				time.AfterFunc(time.Second, func() {
					close(done)
				})
			}
		case *types.Response_Flush:
			// ignore flush responses
		default:
			t.Errorf("Unexpected response type %T", res.Value)
		}
	})

	// Send DeliverTx requests
	for i := 0; i < streamDeliverCount; i++ {
		reqRes := ts.client.DeliverTxAsync(types.RequestDeliverTx{Tx: []byte("test")})
		require.NotNil(t, reqRes)

		if i%flushInterval == 0 {
			ts.client.FlushAsync()
		}
	}

	ts.client.FlushAsync()
	
	select {
	case <-done:
		// Success
	case <-time.After(defaultTestTimeout):
		t.Fatal("Test timeout")
	}
}

func testGRPCSync(t *testing.T, app types.ABCIApplicationServer) {
	ts := setupTest(t, app.(types.Application), true)

	ctx, cancel := context.WithTimeout(context.Background(), defaultGRPCTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, ts.socket,
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return cmtnet.Connect(addr)
		}),
	)
	require.NoError(t, err, "connecting to GRPC server")
	defer conn.Close()

	client := types.NewABCIApplicationClient(conn)

	for i := 0; i < grpcDeliverCount; i++ {
		resp, err := client.DeliverTx(ctx, &types.RequestDeliverTx{Tx: []byte("test")})
		require.NoError(t, err, "DeliverTx request")
		require.Equal(t, code.CodeTypeOK, resp.Code, "DeliverTx failed")
		require.Less(t, i, grpcDeliverCount, "Too many DeliverTx responses")
	}

	time.Sleep(time.Second) // Wait for cleanup
}
