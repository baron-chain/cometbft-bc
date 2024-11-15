package abcicli

import (
    types "github.com/baron-chain/cometbft-bc/abci/types"
    "github.com/baron-chain/cometbft-bc/libs/service"
    bcsync "github.com/baron-chain/cometbft-bc/libs/sync"
)

var _ Client = (*localClient)(nil)

type localClient struct {
    service.BaseService

    mtx   *bcsync.Mutex
    app   types.Application
    resCb Callback
}

func NewLocalClient(mtx *bcsync.Mutex, app types.Application) Client {
    if mtx == nil {
        mtx = new(bcsync.Mutex)
    }
    
    cli := &localClient{
        mtx: mtx,
        app: app,
    }
    cli.BaseService = *service.NewBaseService(nil, "baron-local", cli)
    return cli
}

func (lc *localClient) SetResponseCallback(cb Callback) {
    lc.mtx.Lock()
    lc.resCb = cb
    lc.mtx.Unlock()
}

func (lc *localClient) Error() error {
    return nil
}

// Helper functions for request/response handling
func (lc *localClient) handleAsync(req *types.Request, res *types.Response) *ReqRes {
    if lc.resCb != nil {
        lc.resCb(req, res)
    }
    
    reqRes := NewReqRes(req)
    reqRes.Response = res
    reqRes.callbackInvoked = true
    return reqRes
}

// Async request implementations
func (lc *localClient) FlushAsync() *ReqRes {
    return NewReqRes(types.ToRequestFlush())
}

func (lc *localClient) EchoAsync(msg string) *ReqRes {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    req := types.ToRequestEcho(msg)
    res := types.ToResponseEcho(msg)
    return lc.handleAsync(req, res)
}

func (lc *localClient) CheckTxAsync(req types.RequestCheckTx) *ReqRes {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.CheckTx(req)
    return lc.handleAsync(
        types.ToRequestCheckTx(req),
        types.ToResponseCheckTx(res),
    )
}

func (lc *localClient) DeliverTxAsync(req types.RequestDeliverTx) *ReqRes {
    lc.mtx.Lock() 
    defer lc.mtx.Unlock()

    res := lc.app.DeliverTx(req)
    return lc.handleAsync(
        types.ToRequestDeliverTx(req),
        types.ToResponseDeliverTx(res),
    )
}

func (lc *localClient) QueryAsync(req types.RequestQuery) *ReqRes {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.Query(req)
    return lc.handleAsync(
        types.ToRequestQuery(req),
        types.ToResponseQuery(res),
    )
}

// Sync implementations with error handling
func (lc *localClient) FlushSync() error {
    return nil
}

func (lc *localClient) EchoSync(msg string) (*types.ResponseEcho, error) {
    return &types.ResponseEcho{Message: msg}, nil
}

func (lc *localClient) CheckTxSync(req types.RequestCheckTx) (*types.ResponseCheckTx, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.CheckTx(req)
    return &res, nil
}

func (lc *localClient) DeliverTxSync(req types.RequestDeliverTx) (*types.ResponseDeliverTx, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.DeliverTx(req)
    return &res, nil
}

func (lc *localClient) QuerySync(req types.RequestQuery) (*types.ResponseQuery, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.Query(req)
    return &res, nil
}

// Block-related operations
func (lc *localClient) BeginBlockSync(req types.RequestBeginBlock) (*types.ResponseBeginBlock, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.BeginBlock(req)
    return &res, nil
}

func (lc *localClient) EndBlockSync(req types.RequestEndBlock) (*types.ResponseEndBlock, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.EndBlock(req)
    return &res, nil
}

func (lc *localClient) CommitSync() (*types.ResponseCommit, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.Commit()
    return &res, nil
}

// Proposal handling
func (lc *localClient) PrepareProposalSync(req types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.PrepareProposal(req)
    return &res, nil
}

func (lc *localClient) ProcessProposalSync(req types.RequestProcessProposal) (*types.ResponseProcessProposal, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.ProcessProposal(req)
    return &res, nil
}

// Snapshot operations
func (lc *localClient) ListSnapshotsSync(req types.RequestListSnapshots) (*types.ResponseListSnapshots, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.ListSnapshots(req)
    return &res, nil
}

func (lc *localClient) OfferSnapshotSync(req types.RequestOfferSnapshot) (*types.ResponseOfferSnapshot, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.OfferSnapshot(req)
    return &res, nil
}

func (lc *localClient) LoadSnapshotChunkSync(req types.RequestLoadSnapshotChunk) (*types.ResponseLoadSnapshotChunk, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.LoadSnapshotChunk(req)
    return &res, nil
}

func (lc *localClient) ApplySnapshotChunkSync(req types.RequestApplySnapshotChunk) (*types.ResponseApplySnapshotChunk, error) {
    lc.mtx.Lock()
    defer lc.mtx.Unlock()

    res := lc.app.ApplySnapshotChunk(req)
    return &res, nil
}
