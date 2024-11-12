package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/baron-chain/cometbft-bc/config"
	"github.com/baron-chain/cometbft-bc/crypto/ed25519"
	"github.com/baron-chain/cometbft-bc/libs/log"
	"github.com/baron-chain/cometbft-bc/p2p"
	"github.com/baron-chain/cometbft-bc/privval"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/test/e2e/pkg/infra"
	"github.com/baron-chain/cometbft-bc/types"
)

const (
	AppAddressTCP  = "tcp://127.0.0.1:30000"
	AppAddressUNIX = "unix:///var/run/app.sock"

	PrivvalAddressTCP     = "tcp://0.0.0.0:27559"
	PrivvalAddressUNIX    = "unix:///var/run/privval.sock"
	PrivvalKeyFile        = "config/priv_validator_key.json"
	PrivvalStateFile      = "data/priv_validator_state.json"
	PrivvalDummyKeyFile   = "config/dummy_validator_key.json"
	PrivvalDummyStateFile = "data/dummy_validator_state.json"

	StateSyncDiscoveryTime = 5 * time.Second
)

func Setup(testnet *e2e.Testnet, infp infra.Provider) error {
	logger.Info("setup", "msg", log.NewLazySprintf("Generating testnet files in %q", testnet.Dir))

	if err := setupDirectories(testnet); err != nil {
		return err
	}

	if err := infp.Setup(); err != nil {
		return err
	}

	genesis, err := MakeGenesis(testnet)
	if err != nil {
		return err
	}

	return setupNodes(testnet, genesis)
}

func setupDirectories(testnet *e2e.Testnet) error {
	return os.MkdirAll(testnet.Dir, os.ModePerm)
}

func setupNodes(testnet *e2e.Testnet, genesis types.GenesisDoc) error {
	for _, node := range testnet.Nodes {
		if err := setupNode(node, genesis); err != nil {
			return fmt.Errorf("failed to setup node %s: %w", node.Name, err)
		}
	}
	return nil
}

func setupNode(node *e2e.Node, genesis types.GenesisDoc) error {
	nodeDir := filepath.Join(node.Testnet.Dir, node.Name)

	if err := createNodeDirectories(node, nodeDir); err != nil {
		return err
	}

	if err := createNodeConfigs(node, nodeDir); err != nil {
		return err
	}

	if node.Mode == e2e.ModeLight {
		return nil
	}

	return createNodeFiles(node, nodeDir, genesis)
}

func createNodeDirectories(node *e2e.Node, nodeDir string) error {
	dirs := []string{
		filepath.Join(nodeDir, "config"),
		filepath.Join(nodeDir, "data"),
		filepath.Join(nodeDir, "data", "app"),
	}

	for _, dir := range dirs {
		if node.Mode == e2e.ModeLight && strings.Contains(dir, "app") {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func createNodeConfigs(node *e2e.Node, nodeDir string) error {
	cfg, err := MakeConfig(node)
	if err != nil {
		return err
	}
	config.WriteConfigFile(filepath.Join(nodeDir, "config", "config.toml"), cfg)

	appCfg, err := MakeAppConfig(node)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(nodeDir, "config", "app.toml"), appCfg, 0o644)
}

func createNodeFiles(node *e2e.Node, nodeDir string, genesis types.GenesisDoc) error {
	if err := genesis.SaveAs(filepath.Join(nodeDir, "config", "genesis.json")); err != nil {
		return err
	}

	if err := saveNodeKey(node, nodeDir); err != nil {
		return err
	}

	return setupValidatorFiles(node, nodeDir)
}

func saveNodeKey(node *e2e.Node, nodeDir string) error {
	return (&p2p.NodeKey{PrivKey: node.NodeKey}).SaveAs(filepath.Join(nodeDir, "config", "node_key.json"))
}

func setupValidatorFiles(node *e2e.Node, nodeDir string) error {
	privVal := privval.NewFilePV(node.PrivvalKey,
		filepath.Join(nodeDir, PrivvalKeyFile),
		filepath.Join(nodeDir, PrivvalStateFile))
	privVal.Save()

	dummyPrivVal := privval.NewFilePV(ed25519.GenPrivKey(),
		filepath.Join(nodeDir, PrivvalDummyKeyFile),
		filepath.Join(nodeDir, PrivvalDummyStateFile))
	dummyPrivVal.Save()

	return nil
}

func MakeGenesis(testnet *e2e.Testnet) (types.GenesisDoc, error) {
	genesis := types.GenesisDoc{
		GenesisTime:     time.Now(),
		ChainID:         testnet.Name,
		ConsensusParams: types.DefaultConsensusParams(),
		InitialHeight:   testnet.InitialHeight,
	}
	genesis.ConsensusParams.Version.App = 1

	if err := addValidatorsToGenesis(&genesis, testnet); err != nil {
		return genesis, err
	}

	if err := addInitialStateToGenesis(&genesis, testnet); err != nil {
		return genesis, err
	}

	return genesis, genesis.ValidateAndComplete()
}

func addValidatorsToGenesis(genesis *types.GenesisDoc, testnet *e2e.Testnet) error {
	for validator, power := range testnet.Validators {
		genesis.Validators = append(genesis.Validators, types.GenesisValidator{
			Name:    validator.Name,
			Address: validator.PrivvalKey.PubKey().Address(),
			PubKey:  validator.PrivvalKey.PubKey(),
			Power:   power,
		})
	}

	sort.Slice(genesis.Validators, func(i, j int) bool {
		return strings.Compare(genesis.Validators[i].Name, genesis.Validators[j].Name) == -1
	})
	return nil
}

func addInitialStateToGenesis(genesis *types.GenesisDoc, testnet *e2e.Testnet) error {
	if len(testnet.InitialState) == 0 {
		return nil
	}

	appState, err := json.Marshal(testnet.InitialState)
	if err != nil {
		return err
	}
	genesis.AppState = appState
	return nil
}

func MakeConfig(node *e2e.Node) (*config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.Moniker = node.Name
	cfg.ProxyApp = AppAddressTCP
	cfg.RPC.ListenAddress = "tcp://0.0.0.0:26657"
	cfg.RPC.PprofListenAddress = ":6060"
	cfg.P2P.ExternalAddress = fmt.Sprintf("tcp://%v", node.AddressP2P(false))
	cfg.P2P.AddrBookStrict = false
	cfg.DBBackend = node.Database
	cfg.StateSync.DiscoveryTime = StateSyncDiscoveryTime

	if err := configureABCI(cfg, node); err != nil {
		return nil, err
	}

	if err := configurePrivVal(cfg, node); err != nil {
		return nil, err
	}

	if err := configureMode(cfg, node); err != nil {
		return nil, err
	}

	configureMempoolAndSync(cfg, node)
	configureStateSync(cfg, node)
	configurePeers(cfg, node)

	if node.Prometheus {
		cfg.Instrumentation.Prometheus = true
	}

	return cfg, nil
}

func configureABCI(cfg *config.Config, node *e2e.Node) error {
	switch node.ABCIProtocol {
	case e2e.ProtocolUNIX:
		cfg.ProxyApp = AppAddressUNIX
	case e2e.ProtocolTCP:
		cfg.ProxyApp = AppAddressTCP
	case e2e.ProtocolGRPC:
		cfg.ProxyApp = AppAddressTCP
		cfg.ABCI = "grpc"
	case e2e.ProtocolBuiltin:
		cfg.ProxyApp = ""
		cfg.ABCI = ""
	default:
		return fmt.Errorf("unexpected ABCI protocol setting %q", node.ABCIProtocol)
	}
	return nil
}

func configurePrivVal(cfg *config.Config, node *e2e.Node) error {
	cfg.PrivValidatorListenAddr = ""
	cfg.PrivValidatorKey = PrivvalDummyKeyFile
	cfg.PrivValidatorState = PrivvalDummyStateFile

	if node.Mode == e2e.ModeValidator {
		switch node.PrivvalProtocol {
		case e2e.ProtocolFile:
			cfg.PrivValidatorKey = PrivvalKeyFile
			cfg.PrivValidatorState = PrivvalStateFile
		case e2e.ProtocolUNIX:
			cfg.PrivValidatorListenAddr = PrivvalAddressUNIX
		case e2e.ProtocolTCP:
			cfg.PrivValidatorListenAddr = PrivvalAddressTCP
		default:
			return fmt.Errorf("invalid privval protocol setting %q", node.PrivvalProtocol)
		}
	}
	return nil
}

func configureMode(cfg *config.Config, node *e2e.Node) error {
	switch node.Mode {
	case e2e.ModeSeed:
		cfg.P2P.SeedMode = true
		cfg.P2P.PexReactor = true
	case e2e.ModeValidator, e2e.ModeFull, e2e.ModeLight:
		// Default settings are fine
	default:
		return fmt.Errorf("unexpected mode %q", node.Mode)
	}
	return nil
}

func configureMempoolAndSync(cfg *config.Config, node *e2e.Node) {
	if node.Mempool != "" {
		cfg.Mempool.Version = node.Mempool
	}

	if node.BlockSync == "" {
		cfg.BlockSyncMode = false
	} else {
		cfg.BlockSync.Version = node.BlockSync
	}
}

func configureStateSync(cfg *config.Config, node *e2e.Node) {
	if !node.StateSync {
		return
	}

	cfg.StateSync.Enable = true
	cfg.StateSync.RPCServers = []string{}
	
	for _, peer := range node.Testnet.ArchiveNodes() {
		if peer.Name == node.Name {
			continue
		}
		cfg.StateSync.RPCServers = append(cfg.StateSync.RPCServers, peer.AddressRPC())
	}
}

func configurePeers(cfg *config.Config, node *e2e.Node) {
	var seeds, persistentPeers []string

	for _, seed := range node.Seeds {
		seeds = append(seeds, seed.AddressP2P(true))
	}
	cfg.P2P.Seeds = strings.Join(seeds, ",")

	for _, peer := range node.PersistentPeers {
		persistentPeers = append(persistentPeers, peer.AddressP2P(true))
	}
	cfg.P2P.PersistentPeers = strings.Join(persistentPeers, ",")
}

func MakeAppConfig(node *e2e.Node) ([]byte, error) {
	cfg := createBaseAppConfig(node)

	if err := configureAppABCI(cfg, node); err != nil {
		return nil, err
	}

	if err := configureAppPrivVal(cfg, node); err != nil {
		return nil, err
	}

	if err := configureValidatorUpdates(cfg, node); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return nil, fmt.Errorf("failed to generate app config: %w", err)
	}

	return buf.Bytes(), nil
}

func createBaseAppConfig(node *e2e.Node) map[string]interface{} {
	return map[string]interface{}{
		"chain_id":               node.Testnet.Name,
		"dir":                    "data/app",
		"listen":                 AppAddressUNIX,
		"mode":                   node.Mode,
		"proxy_port":             node.ProxyPort,
		"protocol":               "socket",
		"persist_interval":       node.PersistInterval,
		"snapshot_interval":      node.SnapshotInterval,
		"retain_blocks":          node.RetainBlocks,
		"key_type":               node.PrivvalKey.Type(),
		"prepare_proposal_delay": node.Testnet.PrepareProposalDelay,
		"process_proposal_delay": node.Testnet.ProcessProposalDelay,
		"check_tx_delay":         node.Testnet.CheckTxDelay,
	}
}

// UpdateConfigStateSync updates the state sync config for a node.
func UpdateConfigStateSync(node *e2e.Node, height int64, hash []byte) error {
	cfgPath := filepath.Join(node.Testnet.Dir, node.Name, "config", "config.toml")

	// FIXME Apparently there's no function to simply load a config file without
	// involving the entire Viper apparatus, so we'll just resort to regexps.
	bz, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	bz = regexp.MustCompile(`(?m)^trust_height =.*`).ReplaceAll(bz, []byte(fmt.Sprintf(`trust_height = %v`, height)))
	bz = regexp.MustCompile(`(?m)^trust_hash =.*`).ReplaceAll(bz, []byte(fmt.Sprintf(`trust_hash = "%X"`, hash)))
	return os.WriteFile(cfgPath, bz, 0o644) //nolint:gosec
}
