package types

import (
    context "golang.org/x/net/context"
    "sync"
)

type Application interface {
    // Core Methods
    Info(RequestInfo) ResponseInfo
    Query(RequestQuery) ResponseQuery
    CheckTx(RequestCheckTx) ResponseCheckTx
    
    // Chain & Block Lifecycle
    InitChain(RequestInitChain) ResponseInitChain
    BeginBlock(RequestBeginBlock) ResponseBeginBlock 
    DeliverTx(RequestDeliverTx) ResponseDeliverTx
    EndBlock(RequestEndBlock) ResponseEndBlock
    Commit() ResponseCommit
    
    // Proposal Handling
    PrepareProposal(RequestPrepareProposal) ResponsePrepareProposal
    ProcessProposal(RequestProcessProposal) ResponseProcessProposal
    
    // State Sync
    ListSnapshots(RequestListSnapshots) ResponseListSnapshots
    OfferSnapshot(RequestOfferSnapshot) ResponseOfferSnapshot
    LoadSnapshotChunk(RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk
    ApplySnapshotChunk(RequestApplySnapshotChunk) ResponseApplySnapshotChunk
}

type BaseApplication struct {
    mu sync.RWMutex
}

func NewBaseApplication() *BaseApplication {
    return &BaseApplication{}
}

func (app *BaseApplication) Info(RequestInfo) ResponseInfo {
    return ResponseInfo{}
}

func (app *BaseApplication) DeliverTx(RequestDeliverTx) ResponseDeliverTx {
    return ResponseDeliverTx{Code: CodeTypeOK}
}

func (app *BaseApplication) CheckTx(RequestCheckTx) ResponseCheckTx {
    return ResponseCheckTx{Code: CodeTypeOK}
}

func (app *BaseApplication) Commit() ResponseCommit {
    return ResponseCommit{}
}

func (app *BaseApplication) Query(RequestQuery) ResponseQuery {
    return ResponseQuery{Code: CodeTypeOK}
}

func (app *BaseApplication) InitChain(RequestInitChain) ResponseInitChain {
    return ResponseInitChain{}
}

func (app *BaseApplication) BeginBlock(RequestBeginBlock) ResponseBeginBlock {
    return ResponseBeginBlock{}
}

func (app *BaseApplication) EndBlock(RequestEndBlock) ResponseEndBlock {
    return ResponseEndBlock{}
}

func (app *BaseApplication) ListSnapshots(RequestListSnapshots) ResponseListSnapshots {
    return ResponseListSnapshots{}
}

func (app *BaseApplication) OfferSnapshot(RequestOfferSnapshot) ResponseOfferSnapshot {
    return ResponseOfferSnapshot{}
}

func (app *BaseApplication) LoadSnapshotChunk(RequestLoadSnapshotChunk) ResponseLoadSnapshotChunk {
    return ResponseLoadSnapshotChunk{}
}

func (app *BaseApplication) ApplySnapshotChunk(RequestApplySnapshotChunk) ResponseApplySnapshotChunk {
    return ResponseApplySnapshotChunk{}
}

func (app *BaseApplication) PrepareProposal(req RequestPrepareProposal) ResponsePrepareProposal {
    app.mu.Lock()
    defer app.mu.Unlock()
    
    txs := make([][]byte, 0, len(req.Txs))
    var totalBytes int64
    for _, tx := range req.Txs {
        txBytes := int64(len(tx))
        if totalBytes+txBytes > req.MaxTxBytes {
            break
        }
        totalBytes += txBytes
        txs = append(txs, tx)
    }
    return ResponsePrepareProposal{Txs: txs}
}

func (app *BaseApplication) ProcessProposal(RequestProcessProposal) ResponseProcessProposal {
    return ResponseProcessProposal{Status: ResponseProcessProposal_ACCEPT}
}

type GRPCApplication struct {
    app Application
    mu  sync.RWMutex
}

func NewGRPCApplication(app Application) *GRPCApplication {
    return &GRPCApplication{app: app}
}

func (app *GRPCApplication) Echo(ctx context.Context, req *RequestEcho) (*ResponseEcho, error) {
    return &ResponseEcho{Message: req.Message}, nil
}

func (app *GRPCApplication) Flush(ctx context.Context, req *RequestFlush) (*ResponseFlush, error) {
    return &ResponseFlush{}, nil
}

func (app *GRPCApplication) Info(ctx context.Context, req *RequestInfo) (*ResponseInfo, error) {
    app.mu.RLock()
    defer app.mu.RUnlock()
    res := app.app.Info(*req)
    return &res, nil
}

func (app *GRPCApplication) DeliverTx(ctx context.Context, req *RequestDeliverTx) (*ResponseDeliverTx, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.DeliverTx(*req)
    return &res, nil
}

func (app *GRPCApplication) CheckTx(ctx context.Context, req *RequestCheckTx) (*ResponseCheckTx, error) {
    app.mu.RLock()
    defer app.mu.RUnlock()
    res := app.app.CheckTx(*req)
    return &res, nil
}

func (app *GRPCApplication) Query(ctx context.Context, req *RequestQuery) (*ResponseQuery, error) {
    app.mu.RLock()
    defer app.mu.RUnlock()
    res := app.app.Query(*req)
    return &res, nil
}

func (app *GRPCApplication) Commit(ctx context.Context, req *RequestCommit) (*ResponseCommit, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.Commit()
    return &res, nil
}

func (app *GRPCApplication) InitChain(ctx context.Context, req *RequestInitChain) (*ResponseInitChain, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.InitChain(*req)
    return &res, nil
}

func (app *GRPCApplication) BeginBlock(ctx context.Context, req *RequestBeginBlock) (*ResponseBeginBlock, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.BeginBlock(*req)
    return &res, nil
}

func (app *GRPCApplication) EndBlock(ctx context.Context, req *RequestEndBlock) (*ResponseEndBlock, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.EndBlock(*req)
    return &res, nil
}

func (app *GRPCApplication) ListSnapshots(ctx context.Context, req *RequestListSnapshots) (*ResponseListSnapshots, error) {
    app.mu.RLock()
    defer app.mu.RUnlock()
    res := app.app.ListSnapshots(*req)
    return &res, nil
}

func (app *GRPCApplication) OfferSnapshot(ctx context.Context, req *RequestOfferSnapshot) (*ResponseOfferSnapshot, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.OfferSnapshot(*req)
    return &res, nil
}

func (app *GRPCApplication) LoadSnapshotChunk(ctx context.Context, req *RequestLoadSnapshotChunk) (*ResponseLoadSnapshotChunk, error) {
    app.mu.RLock()
    defer app.mu.RUnlock()
    res := app.app.LoadSnapshotChunk(*req)
    return &res, nil
}

func (app *GRPCApplication) ApplySnapshotChunk(ctx context.Context, req *RequestApplySnapshotChunk) (*ResponseApplySnapshotChunk, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.ApplySnapshotChunk(*req)
    return &res, nil
}

func (app *GRPCApplication) PrepareProposal(ctx context.Context, req *RequestPrepareProposal) (*ResponsePrepareProposal, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.PrepareProposal(*req)
    return &res, nil
}

func (app *GRPCApplication) ProcessProposal(ctx context.Context, req *RequestProcessProposal) (*ResponseProcessProposal, error) {
    app.mu.Lock()
    defer app.mu.Unlock()
    res := app.app.ProcessProposal(*req)
    return &res, nil
}
