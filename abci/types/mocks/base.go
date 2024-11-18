package mocks

import (
	types "github.com/cometbft/cometbft/abci/types"
)

// BaseMock provides a wrapper around the generated Application mock and a BaseApplication.
// BaseMock first tries to use the mock's implementation of the method.
// If no functionality was provided for the mock by the user, BaseMock dispatches
// to the BaseApplication and uses its functionality.
// BaseMock allows users to provide mocked functionality for only the methods that matter
// for their test while avoiding a panic if the code calls Application methods that are
// not relevant to the test.
type BaseMock struct {
	base *types.BaseApplication
	*Application
}

func NewBaseMock() BaseMock {
	return BaseMock{
		base:        types.NewBaseApplication(),
		Application: new(Application),
	}
}

// withFallback is a helper function that executes an Application method and falls back
// to the base implementation if the Application method panics.
func (m BaseMock) withFallback[T any](appMethod func() T, baseMethod func() T) T {
	var ret T
	defer func() {
		if r := recover(); r != nil {
			ret = baseMethod()
		}
	}()
	ret = appMethod()
	return ret
}

// Info/Query Connection
func (m BaseMock) Info(input types.RequestInfo) types.ResponseInfo {
	return m.withFallback(
		func() types.ResponseInfo { return m.Application.Info(input) },
		func() types.ResponseInfo { return m.base.Info(input) },
	)
}

func (m BaseMock) Query(input types.RequestQuery) types.ResponseQuery {
	return m.withFallback(
		func() types.ResponseQuery { return m.Application.Query(input) },
		func() types.ResponseQuery { return m.base.Query(input) },
	)
}

// Mempool Connection
func (m BaseMock) CheckTx(input types.RequestCheckTx) types.ResponseCheckTx {
	return m.withFallback(
		func() types.ResponseCheckTx { return m.Application.CheckTx(input) },
		func() types.ResponseCheckTx { return m.base.CheckTx(input) },
	)
}

// Consensus Connection
func (m BaseMock) InitChain(input types.RequestInitChain) types.ResponseInitChain {
	return m.withFallback(
		func() types.ResponseInitChain { return m.Application.InitChain(input) },
		func() types.ResponseInitChain { return m.base.InitChain(input) },
	)
}

func (m BaseMock) PrepareProposal(input types.RequestPrepareProposal) types.ResponsePrepareProposal {
	return m.withFallback(
		func() types.ResponsePrepareProposal { return m.Application.PrepareProposal(input) },
		func() types.ResponsePrepareProposal { return m.base.PrepareProposal(input) },
	)
}

func (m BaseMock) ProcessProposal(input types.RequestProcessProposal) types.ResponseProcessProposal {
	return m.withFallback(
		func() types.ResponseProcessProposal { return m.Application.ProcessProposal(input) },
		func() types.ResponseProcessProposal { return m.base.ProcessProposal(input) },
	)
}

func (m BaseMock) Commit() types.ResponseCommit {
	return m.withFallback(
		func() types.ResponseCommit { return m.Application.Commit() },
		func() types.ResponseCommit { return m.base.Commit() },
	)
}

// State Sync Connection
func (m BaseMock) ListSnapshots(input types.RequestListSnapshots) types.ResponseListSnapshots {
	return m.withFallback(
		func() types.ResponseListSnapshots { return m.Application.ListSnapshots(input) },
		func() types.ResponseListSnapshots { return m.base.ListSnapshots(input) },
	)
}

func (m BaseMock) OfferSnapshot(input types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return m.withFallback(
		func() types.ResponseOfferSnapshot { return m.Application.OfferSnapshot(input) },
		func() types.ResponseOfferSnapshot { return m.base.OfferSnapshot(input) },
	)
}

func (m BaseMock) LoadSnapshotChunk(input types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return m.withFallback(
		func() types.ResponseLoadSnapshotChunk { return m.Application.LoadSnapshotChunk(input) },
		func() types.ResponseLoadSnapshotChunk { return m.base.LoadSnapshotChunk(input) },
	)
}

func (m BaseMock) ApplySnapshotChunk(input types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return m.withFallback(
		func() types.ResponseApplySnapshotChunk { return m.Application.ApplySnapshotChunk(input) },
		func() types.ResponseApplySnapshotChunk { return m.base.ApplySnapshotChunk(input) },
	)
}

func (m BaseMock) BeginBlock(input types.RequestBeginBlock) types.ResponseBeginBlock {
	return m.withFallback(
		func() types.ResponseBeginBlock { return m.Application.BeginBlock(input) },
		func() types.ResponseBeginBlock { return m.base.BeginBlock(input) },
	)
}

func (m BaseMock) DeliverTx(input types.RequestDeliverTx) types.ResponseDeliverTx {
	return m.withFallback(
		func() types.ResponseDeliverTx { return m.Application.DeliverTx(input) },
		func() types.ResponseDeliverTx { return m.base.DeliverTx(input) },
	)
}

func (m BaseMock) EndBlock(input types.RequestEndBlock) types.ResponseEndBlock {
	return m.withFallback(
		func() types.ResponseEndBlock { return m.Application.EndBlock(input) },
		func() types.ResponseEndBlock { return m.base.EndBlock(input) },
	)
}
