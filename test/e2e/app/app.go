package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/baron-chain/cometbft-bc/abci/types"
	"github.com/baron-chain/cometbft-bc/crypto/rand"
	"github.com/baron-chain/cometbft-bc/libs/log"
	"github.com/baron-chain/cometbft-bc/version"
)

const (
	AppVersion        = 1
	DefaultKeyType    = "ed25519"
	DefaultBlocksKeep = 100
)

// BaronApp represents the Baron Chain ABCI application
type BaronApp struct {
	types.BaseApplication

	logger     log.Logger
	state      *State
	snapshots  *SnapshotStore
	config     *Config
	restoreSnp *types.Snapshot
	chunks     [][]byte
}

// Config defines Baron Chain application configuration
type Config struct {
	DataDir            string        `toml:"data_dir"`
	SnapshotInterval   uint64        `toml:"snapshot_interval"`
	RetainBlocks       uint64        `toml:"retain_blocks"`
	KeyType            string        `toml:"key_type"`
	PersistInterval    uint64        `toml:"persist_interval"`
	ValidatorUpdates   ValidatorMap  `toml:"validator_updates"`
	ProposalDelay      time.Duration `toml:"proposal_delay"`
	ProcessingDelay    time.Duration `toml:"processing_delay"`
	TransactionDelay   time.Duration `toml:"transaction_delay"`
}

type ValidatorMap map[string]map[string]uint8

// NewBaronApp creates a new Baron Chain application instance
func NewBaronApp(cfg *Config) (*BaronApp, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	state, err := NewState(cfg.DataDir, cfg.PersistInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	snapshotDir := filepath.Join(cfg.DataDir, "snapshots")
	snapshots, err := NewSnapshotStore(snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize snapshots: %w", err)
	}

	return &BaronApp{
		logger:    log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		state:     state,
		snapshots: snapshots,
		config:    cfg,
	}, nil
}

func validateConfig(cfg *Config) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("data directory must be specified")
	}
	if cfg.RetainBlocks > 0 && cfg.RetainBlocks < cfg.SnapshotInterval {
		return fmt.Errorf("retain_blocks must be greater than snapshot_interval")
	}
	if cfg.KeyType != "" && cfg.KeyType != "ed25519" && cfg.KeyType != "secp256k1" {
		return fmt.Errorf("invalid key_type: %s", cfg.KeyType)
	}
	return nil
}

// Info implements ABCI interface
func (app *BaronApp) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       AppVersion,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}
}

// InitChain initializes the blockchain with validators and initial app state
func (app *BaronApp) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	if err := app.initializeState(req); err != nil {
		app.logger.Error("failed to initialize chain", "error", err)
		panic(err)
	}

	validators, err := app.getValidatorUpdates(0)
	if err != nil {
		app.logger.Error("failed to get validator updates", "error", err)
		panic(err)
	}

	return types.ResponseInitChain{
		AppHash:    app.state.Hash,
		Validators: validators,
	}
}

func (app *BaronApp) initializeState(req types.RequestInitChain) error {
	app.state.initialHeight = uint64(req.InitialHeight)
	if len(req.AppStateBytes) > 0 {
		if err := app.state.Import(0, req.AppStateBytes); err != nil {
			return fmt.Errorf("failed to import app state: %w", err)
		}
	}
	return nil
}

// CheckTx validates a transaction before adding it to the mempool
func (app *BaronApp) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	if _, _, err := parseTx(req.Tx); err != nil {
		return types.ResponseCheckTx{
			Code: 1,
			Log:  fmt.Sprintf("invalid transaction: %v", err),
		}
	}

	if app.config.TransactionDelay > 0 {
		time.Sleep(app.config.TransactionDelay)
	}

	return types.ResponseCheckTx{Code: 0, GasWanted: 1}
}

// DeliverTx executes a transaction in the blockchain
func (app *BaronApp) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	key, value, err := parseTx(req.Tx)
	if err != nil {
		app.logger.Error("failed to process transaction", "error", err)
		return types.ResponseDeliverTx{Code: 1, Log: err.Error()}
	}

	if err := app.state.Set(key, value); err != nil {
		return types.ResponseDeliverTx{Code: 2, Log: err.Error()}
	}

	return types.ResponseDeliverTx{Code: 0}
}

// PrepareProposal handles block proposal preparation
func (app *BaronApp) PrepareProposal(req types.RequestPrepareProposal) types.ResponsePrepareProposal {
	txs := app.filterTransactions(req.Txs, req.MaxTxBytes)

	if app.config.ProposalDelay > 0 {
		time.Sleep(app.config.ProposalDelay)
	}

	return types.ResponsePrepareProposal{Txs: txs}
}

func (app *BaronApp) filterTransactions(txs [][]byte, maxBytes int64) [][]byte {
	filtered := make([][]byte, 0, len(txs))
	var totalBytes int64

	for _, tx := range txs {
		txSize := int64(len(tx))
		if totalBytes+txSize > maxBytes {
			break
		}
		
		if _, _, err := parseTx(tx); err == nil {
			filtered = append(filtered, tx)
			totalBytes += txSize
		}
	}

	return filtered
}

// NewApplication creates the application.
func NewApplication(cfg *Config) (*Application, error) {
	state, err := NewState(cfg.Dir, cfg.PersistInterval)
	if err != nil {
		return nil, err
	}
	snapshots, err := NewSnapshotStore(filepath.Join(cfg.Dir, "snapshots"))
	if err != nil {
		return nil, err
	}
	return &Application{
		logger:    log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		state:     state,
		snapshots: snapshots,
		cfg:       cfg,
	}, nil
}

// Info implements ABCI.
func (app *Application) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       appVersion,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}
}

// Info implements ABCI.
func (app *Application) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	var err error
	app.state.initialHeight = uint64(req.InitialHeight)
	if len(req.AppStateBytes) > 0 {
		err = app.state.Import(0, req.AppStateBytes)
		if err != nil {
			panic(err)
		}
	}
	resp := abci.ResponseInitChain{
		AppHash: app.state.Hash,
	}
	if resp.Validators, err = app.validatorUpdates(0); err != nil {
		panic(err)
	}
	return resp
}

// CheckTx implements ABCI.
func (app *Application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Log:  err.Error(),
		}
	}

	if app.cfg.CheckTxDelay != 0 {
		time.Sleep(app.cfg.CheckTxDelay)
	}

	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

// DeliverTx implements ABCI.
func (app *Application) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	key, value, err := parseTx(req.Tx)
	if err != nil {
		panic(err) // shouldn't happen since we verified it in CheckTx
	}
	app.state.Set(key, value)
	return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

// EndBlock implements ABCI.
func (app *Application) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	valUpdates, err := app.validatorUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}

	return abci.ResponseEndBlock{
		ValidatorUpdates: valUpdates,
		Events: []abci.Event{
			{
				Type: "val_updates",
				Attributes: []abci.EventAttribute{
					{
						Key:   "size",
						Value: strconv.Itoa(valUpdates.Len()),
					},
					{
						Key:   "height",
						Value: strconv.Itoa(int(req.Height)),
					},
				},
			},
		},
	}
}

// Commit implements ABCI.
func (app *Application) Commit() abci.ResponseCommit {
	height, hash, err := app.state.Commit()
	if err != nil {
		panic(err)
	}
	if app.cfg.SnapshotInterval > 0 && height%app.cfg.SnapshotInterval == 0 {
		snapshot, err := app.snapshots.Create(app.state)
		if err != nil {
			panic(err)
		}
		app.logger.Info("Created state sync snapshot", "height", snapshot.Height)
	}
	retainHeight := int64(0)
	if app.cfg.RetainBlocks > 0 {
		retainHeight = int64(height - app.cfg.RetainBlocks + 1)
	}
	return abci.ResponseCommit{
		Data:         hash,
		RetainHeight: retainHeight,
	}
}

// Query implements ABCI.
func (app *Application) Query(req abci.RequestQuery) abci.ResponseQuery {
	return abci.ResponseQuery{
		Height: int64(app.state.Height),
		Key:    req.Data,
		Value:  []byte(app.state.Get(string(req.Data))),
	}
}

// ListSnapshots implements ABCI.
func (app *Application) ListSnapshots(req abci.RequestListSnapshots) abci.ResponseListSnapshots {
	snapshots, err := app.snapshots.List()
	if err != nil {
		panic(err)
	}
	return abci.ResponseListSnapshots{Snapshots: snapshots}
}

// LoadSnapshotChunk implements ABCI.
func (app *Application) LoadSnapshotChunk(req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {
	chunk, err := app.snapshots.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		panic(err)
	}
	return abci.ResponseLoadSnapshotChunk{Chunk: chunk}
}

// OfferSnapshot implements ABCI.
func (app *Application) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	if app.restoreSnapshot != nil {
		panic("A snapshot is already being restored")
	}
	app.restoreSnapshot = req.Snapshot
	app.restoreChunks = [][]byte{}
	return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}
}

// ApplySnapshotChunk implements ABCI.
func (app *Application) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	if app.restoreSnapshot == nil {
		panic("No restore in progress")
	}
	app.restoreChunks = append(app.restoreChunks, req.Chunk)
	if len(app.restoreChunks) == int(app.restoreSnapshot.Chunks) {
		bz := []byte{}
		for _, chunk := range app.restoreChunks {
			bz = append(bz, chunk...)
		}
		err := app.state.Import(app.restoreSnapshot.Height, bz)
		if err != nil {
			panic(err)
		}
		app.restoreSnapshot = nil
		app.restoreChunks = nil
	}
	return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}
}

func (app *Application) PrepareProposal(
	req abci.RequestPrepareProposal) abci.ResponsePrepareProposal {
	txs := make([][]byte, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		txs = append(txs, tx)
	}

	if app.cfg.PrepareProposalDelay != 0 {
		time.Sleep(app.cfg.PrepareProposalDelay)
	}

	return abci.ResponsePrepareProposal{Txs: txs}
}

// ProcessProposal implements part of the Application interface.
// It accepts any proposal that does not contain a malformed transaction.
func (app *Application) ProcessProposal(req abci.RequestProcessProposal) abci.ResponseProcessProposal {
	for _, tx := range req.Txs {
		_, _, err := parseTx(tx)
		if err != nil {
			return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
		}
	}

	if app.cfg.ProcessProposalDelay != 0 {
		time.Sleep(app.cfg.ProcessProposalDelay)
	}

	return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}
}

func (app *Application) Rollback() error {
	return app.state.Rollback()
}

// validatorUpdates generates a validator set update.
func (app *Application) validatorUpdates(height uint64) (abci.ValidatorUpdates, error) {
	updates := app.cfg.ValidatorUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return nil, nil
	}

	valUpdates := abci.ValidatorUpdates{}
	for keyString, power := range updates {

		keyBytes, err := base64.StdEncoding.DecodeString(keyString)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", keyString, err)
		}
		valUpdates = append(valUpdates, abci.UpdateValidator(keyBytes, int64(power), app.cfg.KeyType))
	}
	return valUpdates, nil
}

// parseTx parses a tx in 'key=value' format into a key and value.
func parseTx(tx []byte) (string, string, error) {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tx format: %q", string(tx))
	}
	if len(parts[0]) == 0 {
		return "", "", errors.New("key cannot be empty")
	}
	return string(parts[0]), string(parts[1]), nil
}
