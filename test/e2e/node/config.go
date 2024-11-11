//BC GEN TEST - #1023811F
// Package config provides configuration management for the application.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cometbft/cometbft/test/e2e/app"
)

// Supported configuration formats
const (
	FormatTOML = "toml"
	FormatJSON = "json"
)

// Default configuration values
const (
	DefaultListenAddr     = "unix:///var/run/app.sock"
	DefaultProtocol      = "socket"
	DefaultPersistInterval = 1
	DefaultMode          = "full"
	DefaultKeyType       = "ed25519"
)

var (
	// ErrEmptyChainID indicates missing chain ID
	ErrEmptyChainID = errors.New("chain_id parameter is required")
	
	// ErrInvalidListenAddr indicates invalid listen address
	ErrInvalidListenAddr = errors.New("invalid listen address")
	
	// ErrInvalidProtocol indicates unsupported protocol
	ErrInvalidProtocol = fmt.Errorf("protocol must be one of: %v", SupportedProtocols)
	
	// ErrInvalidMode indicates unsupported mode
	ErrInvalidMode = fmt.Errorf("mode must be one of: %v", SupportedModes)
	
	// ErrInvalidKeyType indicates unsupported key type
	ErrInvalidKeyType = fmt.Errorf("key_type must be one of: %v", SupportedKeyTypes)
)

// Supported values for various fields
var (
	SupportedProtocols = []string{"socket", "grpc", "builtin"}
	SupportedModes     = []string{"full", "validator", "seed"}
	SupportedKeyTypes  = []string{"ed25519", "secp256k1"}
)

// Config represents the application configuration.
type Config struct {
	// Core settings
	ChainID          string `toml:"chain_id" json:"chain_id"`
	Listen           string `toml:"listen" json:"listen"`
	Protocol         string `toml:"protocol" json:"protocol"`
	Mode             string `toml:"mode" json:"mode"`
	
	// Directory settings
	Dir              string `toml:"dir" json:"dir"`
	
	// State management
	PersistInterval  uint64 `toml:"persist_interval" json:"persist_interval"`
	SnapshotInterval uint64 `toml:"snapshot_interval" json:"snapshot_interval"`
	RetainBlocks     uint64 `toml:"retain_blocks" json:"retain_blocks"`
	
	// Validator settings
	ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update" json:"validator_update"`
	
	// PrivVal settings
	PrivValServer    string `toml:"privval_server" json:"privval_server"`
	PrivValKey       string `toml:"privval_key" json:"privval_key"`
	PrivValState     string `toml:"privval_state" json:"privval_state"`
	KeyType          string `toml:"key_type" json:"key_type"`
}

// NewDefaultConfig returns a configuration with default values.
func NewDefaultConfig() *Config {
	return &Config{
		Listen:          DefaultListenAddr,
		Protocol:        DefaultProtocol,
		Mode:            DefaultMode,
		PersistInterval: DefaultPersistInterval,
		KeyType:         DefaultKeyType,
		ValidatorUpdates: make(map[string]map[string]uint8),
	}
}

// LoadConfig loads the configuration from a file.
func LoadConfig(file string) (*Config, error) {
	cfg := NewDefaultConfig()

	// Determine file format
	format := strings.ToLower(filepath.Ext(file))
	if format != "" {
		format = format[1:] // Remove leading dot
	}

	// Read file
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", file, err)
	}

	// Parse configuration
	switch format {
	case FormatTOML:
		if _, err := toml.Decode(string(data), cfg); err != nil {
			return nil, fmt.Errorf("failed to parse TOML config from %q: %w", file, err)
		}
	case FormatJSON:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config from %q: %w", file, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format %q", format)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to a file.
func (cfg *Config) Save(file string) error {
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Determine format
	format := strings.ToLower(filepath.Ext(file))
	if format != "" {
		format = format[1:] // Remove leading dot
	}

	// Marshal configuration
	var data []byte
	var err error
	switch format {
	case FormatTOML:
		buf := new(strings.Builder)
		if err := toml.NewEncoder(buf).Encode(cfg); err != nil {
			return fmt.Errorf("failed to encode TOML config: %w", err)
		}
		data = []byte(buf.String())
	case FormatJSON:
		if data, err = json.MarshalIndent(cfg, "", "  "); err != nil {
			return fmt.Errorf("failed to encode JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config format %q", format)
	}

	// Write file
	if err := os.WriteFile(file, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// App extracts application-specific configuration parameters.
func (cfg *Config) App() *app.Config {
	return &app.Config{
		Dir:              cfg.Dir,
		SnapshotInterval: cfg.SnapshotInterval,
		RetainBlocks:     cfg.RetainBlocks,
		KeyType:          cfg.KeyType,
		ValidatorUpdates: cfg.ValidatorUpdates,
		PersistInterval:  cfg.PersistInterval,
	}
}

// Validate performs configuration validation.
func (cfg *Config) Validate() error {
	// Check required fields
	if cfg.ChainID == "" {
		return ErrEmptyChainID
	}

	// Validate protocol
	if !contains(SupportedProtocols, cfg.Protocol) {
		return ErrInvalidProtocol
	}

	// Validate listen address if needed
	if cfg.Protocol != "builtin" && cfg.Listen == "" {
		return ErrInvalidListenAddr
	}

	// Validate mode
	if cfg.Mode != "" && !contains(SupportedModes, cfg.Mode) {
		return ErrInvalidMode
	}

	// Validate key type
	if cfg.KeyType != "" && !contains(SupportedKeyTypes, cfg.KeyType) {
		return ErrInvalidKeyType
	}

	// Validate intervals
	if err := cfg.validateIntervals(); err != nil {
		return err
	}

	return nil
}

// validateIntervals performs validation of interval-related settings.
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

// String returns a string representation of the configuration.
func (cfg *Config) String() string {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}

// Contains checks if a string is present in a slice.
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
