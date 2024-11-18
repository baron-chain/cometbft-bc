package mocks

import (
    types "github.com/baron-chain/cometbft-bc/abci/types"
    "sync"
)

type BaseMock struct {
    base        *types.BaseApplication
    Application *Application
    mu          sync.RWMutex
}

func NewBaseMock() *BaseMock {
    return &BaseMock{
        base:        types.NewBaseApplication(),
        Application: new(Application),
    }
}

func (m *BaseMock) withFallback[T any](appMethod func() T, baseMethod func() T) T {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    var ret T
    defer func() {
        if r := recover(); r != nil {
            ret = baseMethod()
        }
    }()
    return appMethod()
}

// Core ABCI Methods
func (m *BaseMock) Info(req types.RequestInfo) types.ResponseInfo {
    return m.withFallback(
        func() types.ResponseInfo { return m.Application.Info(req) },
        func() types.ResponseInfo { return m.base.Info(req) },
    )
}

func (m *BaseMock) Query(req types.RequestQuery) types.ResponseQuery {
    return m.withFallback(
        func() types.ResponseQuery { return m.Application.Query(req) },
        func() types.ResponseQuery { return m.base.Query(req) },
    )
}

func (m *BaseMock) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
    return m.withFallback(
        func() types.ResponseCheckTx { return m.Application.CheckTx(req) },
        func() types.ResponseCheckTx { return m.base.CheckTx(req) },
    )
}

// Consensus Methods
func (m *BaseMock) InitChain(req types.RequestInitChain) types.ResponseInitChain {
    return m.withFallback(
        func() types.ResponseInitChain { return m.Application.InitChain(req) },
        func() types.ResponseInitChain { return m.base.InitChain(req) },
    )
}

func (m *BaseMock) PrepareProposal(req types.RequestPrepareProposal) types.ResponsePrepareProposal {
    return m.withFallback(
        func() types.ResponsePrepareProposal { return m.Application.PrepareProposal(req) },
        func() types.ResponsePrepareProposal { return m.base.PrepareProposal(req) },
    )
}

func (m *BaseMock) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
    return m.withFallback(
        func() types.ResponseProcessProposal { return m.Application.ProcessProposal(req) },
        func() types.ResponseProcessProposal { return m.base.ProcessProposal(req) },
    )
}

func (m *BaseMock) Commit() types.ResponseCommit {
    return m.withFallback(
        func() types.ResponseCommit { return m.Application.Commit() },
        func() types.ResponseCommit { return m.base.Commit() },
    )
}

// State Sync Methods
func (m *BaseMock) ListSnapshots(req types.RequestListSnapshots) types.ResponseListSnapshots {
    return m.withFallback(
        func() types.ResponseListSnapshots { return m.Application.ListSnapshots(req) },
        func() types.ResponseListSnapshots { return m.base.ListSnapshots(req) },
    )
}

func (m *BaseMock) OfferSnapshot(req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
    return m.withFallback(
        func() types.ResponseOfferSnapshot { return m.Application.OfferSnapshot(req) },
        func() types.ResponseOfferSnapshot { return m.base.OfferSnapshot(req) },
    )
}

func (m *BaseMock) LoadSnapshotChunk(req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
    return m.withFallback(
        func() types.ResponseLoadSnapshotChunk { return m.Application.LoadSnapshotChunk(req) },
        func() types.ResponseLoadSnapshotChunk { return m.base.LoadSnapshotChunk(req) },
    )
}

func (m *BaseMock) ApplySnapshotChunk(req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
    return m.withFallback(
        func() types.ResponseApplySnapshotChunk { return m.Application.ApplySnapshotChunk(req) },
        func() types.ResponseApplySnapshotChunk { return m.base.ApplySnapshotChunk(req) },
    )
}

// Block Lifecycle Methods
func (m *BaseMock) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
    return m.withFallback(
        func() types.ResponseBeginBlock { return m.Application.BeginBlock(req) },
        func() types.ResponseBeginBlock { return m.base.BeginBlock(req) },
    )
}

func (m *BaseMock) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
    return m.withFallback(
        func() types.ResponseDeliverTx { return m.Application.DeliverTx(req) },
        func() types.ResponseDeliverTx { return m.base.DeliverTx(req) },
    )
}

func (m *BaseMock) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
    return m.withFallback(
        func() types.ResponseEndBlock { return m.Application.EndBlock(req) },
        func() types.ResponseEndBlock { return m.base.EndBlock(req) },
    )
}
