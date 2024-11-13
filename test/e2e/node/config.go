package config

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/BurntSushi/toml"
    "github.com/baron-chain/cometbft-bc/test/e2e/app"
)

const (
    FormatTOML = "toml"
    FormatJSON = "json"
    
    DefaultListenAddr = "tcp://0.0.0.0:26658"
    DefaultProtocol = "socket"
    DefaultPersistInterval = 1
    DefaultMode = "validator"
    DefaultKeyType = "kyber"
    DefaultSnapshotInterval = 100
    DefaultRetainBlocks = 10000
    DefaultConsensusTimeout = 5000
    DefaultBlockTime = 1000
    
    DefaultFilePerms = 0644
    DefaultDirPerms = 0755
)

var (
    ErrEmptyChainID = errors.New("chain_id parameter is required")
    ErrInvalidListenAddr = errors.New("invalid listen address")
    ErrInvalidProtocol = fmt.Errorf("protocol must be one of: %v", SupportedProtocols)
    ErrInvalidMode = fmt.Errorf("mode must be one of: %v", SupportedModes)
    ErrInvalidKeyType = fmt.Errorf("key_type must be one of: %v", SupportedKeyTypes)
    ErrInvalidBlockTime = errors.New("block_time must be greater than 0")
    ErrInvalidConsensusTimeout = errors.New("consensus_timeout must be greater than block_time")
)

var (
    SupportedProtocols = []string{"socket", "grpc", "builtin"}
    SupportedModes = []string{"validator", "full", "seed", "light"}
    SupportedKeyTypes = []string{"kyber", "dilithium", "falcon", "ed25519"}
)

type Config struct {
    // Core settings
    ChainID string `toml:"chain_id" json:"chain_id"`
    Version string `toml:"version" json:"version"`
    Listen string `toml:"listen" json:"listen"`
    Protocol string `toml:"protocol" json:"protocol"`
    Mode string `toml:"mode" json:"mode"`
    
    // Directory settings
    Dir string `toml:"dir" json:"dir"`
    DataDir string `toml:"data_dir" json:"data_dir"`
    LogDir string `toml:"log_dir" json:"log_dir"`
    
    // State management
    PersistInterval uint64 `toml:"persist_interval" json:"persist_interval"`
    SnapshotInterval uint64 `toml:"snapshot_interval" json:"snapshot_interval"`
    RetainBlocks uint64 `toml:"retain_blocks" json:"retain_blocks"`
    StateSync bool `toml:"state_sync" json:"state_sync"`
    
    // Consensus settings
    BlockTime uint64 `toml:"block_time" json:"block_time"`
    ConsensusTimeout uint64 `toml:"consensus_timeout" json:"consensus_timeout"`
    
    // Validator settings
    ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update" json:"validator_update"`
    ValidatorKey string `toml:"validator_key" json:"validator_key"`
    MinValidatorStake uint64 `toml:"min_validator_stake" json:"min_validator_stake"`
    
    // Crypto settings
    KeyType string `toml:"key_type" json:"key_type"`
    PrivValServer string `toml:"privval_server" json:"privval_server"`
    PrivValKey string `toml:"privval_key" json:"privval_key"`
    PrivValState string `toml:"privval_state" json:"privval_state"`
    
    // Performance settings
    MaxTxBytes uint64 `toml:"max_tx_bytes" json:"max_tx_bytes"`
    MaxBlockBytes uint64 `toml:"max_block_bytes" json:"max_block_bytes"`
    MaxGas uint64 `toml:"max_gas" json:"max_gas"`
    
    // Network settings
    P2PPort uint16 `toml:"p2p_port" json:"p2p_port"`
    RPCPort uint16 `toml:"rpc_port" json:"rpc_port"`
    MetricsPort uint16 `toml:"metrics_port" json:"metrics_port"`
    MaxPeers uint16 `toml:"max_peers" json:"max_peers"`
}

func NewDefaultConfig() *Config {
    return &Config{
        Listen: DefaultListenAddr,
        Protocol: DefaultProtocol,
        Mode: DefaultMode,
        PersistInterval: DefaultPersistInterval,
        KeyType: DefaultKeyType,
        SnapshotInterval: DefaultSnapshotInterval,
        RetainBlocks: DefaultRetainBlocks,
        BlockTime: DefaultBlockTime,
        ConsensusTimeout: DefaultConsensusTimeout,
        P2PPort: 26656,
        RPCPort: 26657,
        MetricsPort: 26660,
        MaxPeers: 50,
        StateSync: true,
        ValidatorUpdates: make(map[string]map[string]uint8),
    }
}

func LoadConfig(file string) (*Config, error) {
    cfg := NewDefaultConfig()

    format := getConfigFormat(file)
    data, err := os.ReadFile(file)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file %q: %w", file, err)
    }

    if err := parseConfig(format, data, cfg); err != nil {
        return nil, err
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return cfg, nil
}

func (cfg *Config) Save(file string) error {
    if err := os.MkdirAll(filepath.Dir(file), DefaultDirPerms); err != nil {
        return fmt.Errorf("failed to create config directory: %w", err)
    }

    data, err := marshalConfig(getConfigFormat(file), cfg)
    if err != nil {
        return err
    }

    return os.WriteFile(file, data, DefaultFilePerms)
}

func (cfg *Config) App() *app.Config {
    return &app.Config{
        Dir: cfg.Dir,
        DataDir: cfg.DataDir,
        LogDir: cfg.LogDir,
        SnapshotInterval: cfg.SnapshotInterval,
        RetainBlocks: cfg.RetainBlocks,
        KeyType: cfg.KeyType,
        ValidatorUpdates: cfg.ValidatorUpdates,
        PersistInterval: cfg.PersistInterval,
        StateSync: cfg.StateSync,
        BlockTime: cfg.BlockTime,
        ConsensusTimeout: cfg.ConsensusTimeout,
    }
}

func (cfg *Config) Validate() error {
    if cfg.ChainID == "" {
        return ErrEmptyChainID
    }

    if !contains(SupportedProtocols, cfg.Protocol) {
        return ErrInvalidProtocol
    }

    if cfg.Protocol != "builtin" && cfg.Listen == "" {
        return ErrInvalidListenAddr
    }

    if cfg.Mode != "" && !contains(SupportedModes, cfg.Mode) {
        return ErrInvalidMode
    }

    if cfg.KeyType != "" && !contains(SupportedKeyTypes, cfg.KeyType) {
        return ErrInvalidKeyType
    }

    if cfg.BlockTime == 0 {
        return ErrInvalidBlockTime
    }

    if cfg.ConsensusTimeout <= cfg.BlockTime {
        return ErrInvalidConsensusTimeout
    }

    return cfg.validateIntervals()
}

// Helper functions

func getConfigFormat(file string) string {
    format := strings.ToLower(filepath.Ext(file))
    if format != "" {
        format = format[1:]
    }
    return format
}

func parseConfig(format string, data []byte, cfg *Config) error {
    switch format {
    case FormatTOML:
        if _, err := toml.Decode(string(data), cfg); err != nil {
            return fmt.Errorf("failed to parse TOML config: %w", err)
        }
    case FormatJSON:
        if err := json.Unmarshal(data, cfg); err != nil {
            return fmt.Errorf("failed to parse JSON config: %w", err)
        }
    default:
        return fmt.Errorf("unsupported config format %q", format)
    }
    return nil
}

func marshalConfig(format string, cfg *Config) ([]byte, error) {
    switch format {
    case FormatTOML:
        buf := new(strings.Builder)
        if err := toml.NewEncoder(buf).Encode(cfg); err != nil {
            return nil, fmt.Errorf("failed to encode TOML config: %w", err)
        }
        return []byte(buf.String()), nil
    case FormatJSON:
        data, err := json.MarshalIndent(cfg, "", "  ")
        if err != nil {
            return nil, fmt.Errorf("failed to encode JSON config: %w", err)
        }
        return data, nil
    default:
        return nil, fmt.Errorf("unsupported config format %q", format)
    }
}

func contains(slice []string, str string) bool {
    for _, s := range slice {
        if s == str {
            return true
        }
    }
    return false
}

func (cfg *Config) validateIntervals() error {
    if cfg.PersistInterval == 0 {
        return errors.New("persist_interval must be greater than 0")
    }
    if cfg.SnapshotInterval > 0 && cfg.SnapshotInterval < cfg.PersistInterval {
        return errors.New("snapshot_interval must be greater than persist_interval")
    }
    if cfg.RetainBlocks > 0 && cfg.RetainBlocks < cfg.SnapshotInterval {
        return errors.New("retain_blocks must be greater than snapshot_interval")
    }
    return nil
}
