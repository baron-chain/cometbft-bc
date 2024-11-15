package abcicli

import (
    "fmt"
    "sync"
    "time"

    "github.com/baron-chain/cometbft-bc/abci/types"
    "github.com/baron-chain/cometbft-bc/libs/service"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
)

const (
    dialRetryInterval  = 3 * time.Second
    echoRetryInterval = 1 * time.Second
)

// Client defines the interface for ABCI client implementations
type Client interface {
    service.Service
    SetResponseCallback(Callback)
    Error() error
    
    // Async methods
    FlushAsync() *ReqRes
    EchoAsync(msg string) *ReqRes
    InfoAsync(types.RequestInfo) *ReqRes
    DeliverTxAsync(types.RequestDeliverTx) *ReqRes
    CheckTxAsync(types.RequestCheckTx) *ReqRes
    QueryAsync(types.RequestQuery) *ReqRes
    CommitAsync() *ReqRes
    InitChainAsync(types.RequestInitChain) *ReqRes
    PrepareProposalAsync(types.RequestPrepareProposal) *ReqRes
    BeginBlockAsync(types.RequestBeginBlock) *ReqRes
    EndBlockAsync(types.RequestEndBlock) *ReqRes
    ListSnapshotsAsync(types.RequestListSnapshots) *ReqRes
    OfferSnapshotAsync(types.RequestOfferSnapshot) *ReqRes
    LoadSnapshotChunkAsync(types.RequestLoadSnapshotChunk) *ReqRes
    ApplySnapshotChunkAsync(types.RequestApplySnapshotChunk) *ReqRes
    ProcessProposalAsync(types.RequestProcessProposal) *ReqRes
    
    // Sync methods
    FlushSync() error
    EchoSync(msg string) (*types.ResponseEcho, error)
    InfoSync(types.RequestInfo) (*types.ResponseInfo, error)
    DeliverTxSync(types.RequestDeliverTx) (*types.ResponseDeliverTx, error)
    CheckTxSync(types.RequestCheckTx) (*types.ResponseCheckTx, error)
    QuerySync(types.RequestQuery) (*types.ResponseQuery, error)
    CommitSync() (*types.ResponseCommit, error)
    InitChainSync(types.RequestInitChain) (*types.ResponseInitChain, error)
    PrepareProposalSync(types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error)
    BeginBlockSync(types.RequestBeginBlock) (*types.ResponseBeginBlock, error)
    EndBlockSync(types.RequestEndBlock) (*types.ResponseEndBlock, error)
    ListSnapshotsSync(types.RequestListSnapshots) (*types.ResponseListSnapshots, error)
    OfferSnapshotSync(types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error)
    LoadSnapshotChunkSync(types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error)
    ApplySnapshotChunkSync(types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error)
    ProcessProposalSync(types.RequestProcessProposal) (*types.ResponseProcessProposal, error)
}

// Callback is the signature for response callbacks
type Callback func(*types.Request, *types.Response)

// NewClient creates a new ABCI client
func NewClient(addr, transport string, mustConnect bool) (Client, error) {
    switch transport {
    case "socket":
        return NewSocketClient(addr, mustConnect), nil
    case "grpc":
        return NewGRPCClient(addr, mustConnect), nil
    default:
        return nil, fmt.Errorf("baron-chain: unsupported transport %q", transport)
    }
}

// ReqRes represents a request-response pair
type ReqRes struct {
    *types.Request
    *sync.WaitGroup
    *types.Response
    
    mtx             bcsync.Mutex
    cb              func(*types.Response)
    callbackInvoked bool
}

// NewReqRes creates a new request-response pair
func NewReqRes(req *types.Request) *ReqRes {
    return &ReqRes{
        Request:   req,
        WaitGroup: &sync.WaitGroup{},
    }
}

// SetCallback sets the response callback
func (r *ReqRes) SetCallback(cb func(res *types.Response)) {
    r.mtx.Lock()
    defer r.mtx.Unlock()
    
    if r.callbackInvoked && r.Response != nil {
        cb(r.Response)
        return
    }
    r.cb = cb
}

// InvokeCallback invokes the response callback if set
func (r *ReqRes) InvokeCallback() {
    r.mtx.Lock()
    defer r.mtx.Unlock()
    
    if r.cb != nil && r.Response != nil {
        r.cb(r.Response)
        r.callbackInvoked = true
    }
}

// GetCallback returns the current callback function
func (r *ReqRes) GetCallback() func(*types.Response) {
    r.mtx.Lock()
    defer r.mtx.Unlock()
    return r.cb
}
