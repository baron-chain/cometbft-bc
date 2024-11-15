package abcicli

import (
    "context"
    "fmt"
    "net"
    "sync"
    "time"

    "golang.org/x/net/context"
    "google.golang.org/grpc"

    "github.com/baron-chain/cometbft-bc/abci/types"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
    "github.com/baron-chain/cometbft-bc/libs/service"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
)

const (
    dialTimeout  = 3 * time.Second
    echoTimeout  = 1 * time.Second
    channelSize  = 64 // Buffer size for async responses
)

var _ Client = (*grpcClient)(nil)

type grpcClient struct {
    service.BaseService
    mustConnect bool

    client   types.ABCIApplicationClient
    conn     *grpc.ClientConn
    chReqRes chan *ReqRes // Ordered async responses channel

    mtx   bcsync.Mutex
    addr  string
    err   error
    resCb func(*types.Request, *types.Response)
}

func NewGRPCClient(addr string, mustConnect bool) Client {
    cli := &grpcClient{
        addr:        addr,
        mustConnect: mustConnect,
        chReqRes:    make(chan *ReqRes, channelSize),
    }
    cli.BaseService = *service.NewBaseService(nil, "baronchain-grpc", cli)
    return cli
}

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
    return bcnet.Connect(addr)
}

func (cli *grpcClient) OnStart() error {
    if err := cli.BaseService.OnStart(); err != nil {
        return err
    }

    go cli.processResponses()

    for {
        conn, err := grpc.Dial(cli.addr, 
            grpc.WithInsecure(), 
            grpc.WithContextDialer(dialerFunc),
            grpc.WithTimeout(dialTimeout))

        if err != nil {
            if cli.mustConnect {
                return fmt.Errorf("baron-chain: failed to connect to %s: %w", cli.addr, err)
            }
            cli.Logger.Error("Failed to connect, retrying...", "addr", cli.addr, "err", err)
            time.Sleep(dialTimeout)
            continue
        }

        cli.conn = conn
        client := types.NewABCIApplicationClient(conn)
        cli.client = client

        // Verify connection with echo
        ctx, cancel := context.WithTimeout(context.Background(), echoTimeout)
        defer cancel()
        
        _, err = client.Echo(ctx, &types.RequestEcho{Message: "baron-chain"}, grpc.WaitForReady(true))
        if err == nil {
            cli.Logger.Info("Connected to ABCI server", "addr", cli.addr)
            return nil
        }

        cli.Logger.Error("Echo failed", "err", err)
        time.Sleep(echoTimeout)
    }
}

func (cli *grpcClient) OnStop() {
    cli.BaseService.OnStop()
    if cli.conn != nil {
        cli.conn.Close()
    }
    close(cli.chReqRes)
}

func (cli *grpcClient) processResponses() {
    for reqres := range cli.chReqRes {
        if reqres == nil {
            cli.Logger.Error("Received nil reqres")
            continue
        }

        cli.mtx.Lock()
        reqres.Done() 

        if cli.resCb != nil {
            cli.resCb(reqres.Request, reqres.Response)
        }

        reqres.InvokeCallback()
        cli.mtx.Unlock()
    }
}

// Error handling
func (cli *grpcClient) StopForError(err error) {
    cli.mtx.Lock()
    if !cli.IsRunning() {
        cli.mtx.Unlock()
        return
    }

    if cli.err == nil {
        cli.err = fmt.Errorf("baron-chain: %w", err)
    }
    cli.mtx.Unlock()

    cli.Logger.Error("Stopping GRPC client", "err", err)
    cli.Stop()
}

func (cli *grpcClient) Error() error {
    cli.mtx.Lock()
    defer cli.mtx.Unlock()
    return cli.err
}

func (cli *grpcClient) SetResponseCallback(resCb Callback) {
    cli.mtx.Lock()
    cli.resCb = resCb
    cli.mtx.Unlock()
}

// Async call helpers
func (cli *grpcClient) finishAsyncCall(req *types.Request, res *types.Response) *ReqRes {
    reqres := NewReqRes(req)
    reqres.Response = res
    cli.chReqRes <- reqres
    return reqres
}

// Only showing a few examples of async methods - apply same pattern to all others
func (cli *grpcClient) EchoAsync(msg string) *ReqRes {
    req := types.ToRequestEcho(msg)
    res, err := cli.client.Echo(context.Background(), req.GetEcho(), grpc.WaitForReady(true))
    if err != nil {
        cli.StopForError(err)
    }
    return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_Echo{Echo: res}})
}

func (cli *grpcClient) CheckTxAsync(params types.RequestCheckTx) *ReqRes {
    req := types.ToRequestCheckTx(params) 
    res, err := cli.client.CheckTx(context.Background(), req.GetCheckTx(), grpc.WaitForReady(true))
    if err != nil {
        cli.StopForError(err)
    }
    return cli.finishAsyncCall(req, &types.Response{Value: &types.Response_CheckTx{CheckTx: res}})
}

// Sync call helper
func (cli *grpcClient) finishSyncCall(reqres *ReqRes) *types.Response {
    var once sync.Once
    ch := make(chan *types.Response, 1)
    reqres.SetCallback(func(res *types.Response) {
        once.Do(func() {
            ch <- res
        })
    })
    return <-ch
}

// Only showing a few examples of sync methods - apply same pattern to all others
func (cli *grpcClient) EchoSync(msg string) (*types.ResponseEcho, error) {
    reqres := cli.EchoAsync(msg)
    return cli.finishSyncCall(reqres).GetEcho(), cli.Error()
}

func (cli *grpcClient) CheckTxSync(params types.RequestCheckTx) (*types.ResponseCheckTx, error) {
    reqres := cli.CheckTxAsync(params)
    return cli.finishSyncCall(reqres).GetCheckTx(), cli.Error()
}
