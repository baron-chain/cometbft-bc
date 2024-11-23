package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/baron-chain/cometbft-bc/crypto/kyber"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDataDir = "data"
)

type configTest struct {
	rootDir string
	cleanup func()
}

func setupTestDir(t *testing.T) *configTest {
	tmpDir, err := os.MkdirTemp("", "baron-config-test")
	require.NoError(t, err, "Failed to create temp directory")
	
	return &configTest{
		rootDir: tmpDir,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}
}

func ensureFiles(t *testing.T, rootDir string, files ...string) {
	for _, f := range files {
		path := filepath.Join(rootDir, f)
		_, err := os.Stat(path)
		assert.NoError(t, err, "File not found: %s", path)
	}
}

func validateQuantumSafeKeys(t *testing.T, keyPath string) {
	data, err := os.ReadFile(keyPath)
	require.NoError(t, err, "Failed to read validator key file")

	var keyData struct {
		PubKey struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"pub_key"`
	}

	err = json.Unmarshal(data, &keyData)
	require.NoError(t, err, "Failed to parse validator key file")
	assert.Equal(t, "baronchain/PubKeyKyber", keyData.PubKey.Type, "Invalid key type")

	// Validate key format
	_, err = kyber.ParsePubKey(keyData.PubKey.Value)
	assert.NoError(t, err, "Invalid quantum-safe public key format")
}

func TestEnsureRoot(t *testing.T) {
	test := setupTestDir(t)
	defer test.cleanup()

	// Create root directory
	EnsureRoot(test.rootDir)

	// Validate config file
	configPath := filepath.Join(test.rootDir, defaultConfigFilePath)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read config file")
	
	assert.True(t, validateConfig(string(data)), "Config file missing required fields")
	
	// Validate directory structure
	ensureFiles(t, test.rootDir, testDataDir)
}

func TestEnsureTestRoot(t *testing.T) {
	testName := "baron-ensure-test-root"
	
	// Create test configuration
	cfg := ResetTestRoot(testName)
	defer os.RemoveAll(cfg.RootDir)

	// Validate config file
	configPath := filepath.Join(cfg.RootDir, defaultConfigFilePath)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read config file")
	assert.True(t, validateConfig(string(data)), "Config file missing required fields")

	// Validate required files
	baseConfig := DefaultBaseConfig()
	requiredFiles := []string{
		defaultDataDir,
		baseConfig.Genesis,
		baseConfig.PrivValidatorKey,
		baseConfig.PrivValidatorState,
	}
	ensureFiles(t, cfg.RootDir, requiredFiles...)

	// Validate quantum-safe keys
	validateQuantumSafeKeys(t, filepath.Join(cfg.RootDir, baseConfig.PrivValidatorKey))
}

func TestCustomChainID(t *testing.T) {
	testName := "baron-custom-chain"
	chainID := "baron-test-chain"
	
	cfg := ResetTestRootWithChainID(testName, chainID)
	defer os.RemoveAll(cfg.RootDir)

	// Validate genesis file contains correct chain ID
	genesisPath := filepath.Join(cfg.RootDir, DefaultBaseConfig().Genesis)
	data, err := os.ReadFile(genesisPath)
	require.NoError(t, err, "Failed to read genesis file")

	var genesis struct {
		ChainID string `json:"chain_id"`
	}
	err = json.Unmarshal(data, &genesis)
	require.NoError(t, err, "Failed to parse genesis file")
	assert.Equal(t, chainID, genesis.ChainID, "Incorrect chain ID in genesis file")
}

func validateConfig(configFile string) bool {
	requiredFields := []string{
		"moniker",
		"seeds",
		"proxy_app",
		"quantum_safe",
		"create_empty_blocks",
		"peer",
		"timeout",
		"broadcast",
		"send",
		"addr",
		"wal",
		"propose",
		"max",
		"genesis",
		"kyber",
	}

	for _, field := range requiredFields {
		if !strings.Contains(configFile, field) {
			return false
		}
	}
	return true
}

func TestConfigValidation(t *testing.T) {
	test := setupTestDir(t)
	defer test.cleanup()

	// Test invalid config
	invalidConfig := `moniker = "test-node"`
	assert.False(t, validateConfig(invalidConfig), "Invalid config should fail validation")

	// Test valid config
	validConfig := strings.Join([]string{
		`moniker = "baron-test-node"`,
		`quantum_safe = true`,
		`seeds = ""`,
		`proxy_app = "tcp://127.0.0.1:26658"`,
		`create_empty_blocks = true`,
		`peer_gossip_sleep_duration = "100ms"`,
		`timeout_propose = "3s"`,
		`broadcast_rate = "5120000"`,
		`send_rate = "5120000"`,
		`addr_book_strict = false`,
		`wal_file = "data/cs.wal"`,
		`propose_timeout = "3s"`,
		`max_packet_msg_payload_size = 1024`,
		`genesis_file = "config/genesis.json"`,
		`kyber_key_file = "config/priv_validator_key.json"`,
	}, "\n")
	assert.True(t, validateConfig(validConfig), "Valid config should pass validation")
}
