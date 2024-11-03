package abcicli

import (
	"fmt"
	"sync"

	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/service"
	cmtsync "github.com/cometbft/cometbft/libs/sync"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

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

type Callback func(*types.Request, *types.Response)

func NewClient(addr, transport string, mustConnect bool) (Client, error) {
	switch transport {
	case "socket":
		return NewSocketClient(addr, mustConnect), nil
	case "grpc":
		return NewGRPCClient(addr, mustConnect), nil
	default:
		return nil, fmt.Errorf("unsupported transport: %s", transport)
	}
}

type ReqRes struct {
	*types.Request
	*sync.WaitGroup
	*types.Response

	mtx             cmtsync.Mutex
	cb              func(*types.Response)
	callbackInvoked bool
}

func NewReqRes(req *types.Request) *ReqRes {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	
	return &ReqRes{
		Request:   req,
		WaitGroup: wg,
	}
}

func (r *ReqRes) SetCallback(cb func(res *types.Response)) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.callbackInvoked {
		cb(r.Response)
		return
	}
	r.cb = cb
}

func (r *ReqRes) InvokeCallback() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.cb != nil {
		r.cb(r.Response)
	}
	r.callbackInvoked = true
}

func (r *ReqRes) GetCallback() func(*types.Response) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.cb
}
