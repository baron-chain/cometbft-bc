package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	bcos "github.com/baron-chain/cometbft-bc/libs/os"
	"github.com/baron-chain/cometbft-bc/crypto/kyber"
)

const (
	DefaultDirPerm = 0700
	defaultHomeDir = ".baronchain"
)

var configTemplate *template.Template

func init() {
	tmpl := template.New("configFileTemplate").Funcs(template.FuncMap{
		"StringsJoin": strings.Join,
	})
	var err error
	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(fmt.Sprintf("Failed to parse config template: %v", err))
	}
}

func EnsureRoot(rootDir string) {
	dirs := []string{
		rootDir,
		filepath.Join(rootDir, defaultConfigDir),
		filepath.Join(rootDir, defaultDataDir),
	}

	for _, dir := range dirs {
		if err := bcos.EnsureDir(dir, DefaultDirPerm); err != nil {
			panic(fmt.Sprintf("Failed to create directory %s: %v", dir, err))
		}
	}

	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	if !bcos.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
}

func writeDefaultConfigFile(configFilePath string) {
	WriteConfigFile(configFilePath, DefaultConfig())
}

func WriteConfigFile(configFilePath string, config *Config) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(fmt.Sprintf("Failed to execute config template: %v", err))
	}

	bcos.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}

func ResetTestRoot(testName string) *Config {
	return ResetTestRootWithChainID(testName, "")
}

func ResetTestRootWithChainID(testName string, chainID string) *Config {
	rootDir, err := os.MkdirTemp("", fmt.Sprintf("%s-%s_", chainID, testName))
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp directory: %v", err))
	}

	for _, dir := range []string{defaultConfigDir, defaultDataDir} {
		if err := bcos.EnsureDir(filepath.Join(rootDir, dir), DefaultDirPerm); err != nil {
			panic(fmt.Sprintf("Failed to create directory: %v", err))
		}
	}

	baseConfig := DefaultBaseConfig()
	paths := map[string]string{
		"config":  filepath.Join(rootDir, defaultConfigFilePath),
		"genesis": filepath.Join(rootDir, baseConfig.Genesis),
		"privKey": filepath.Join(rootDir, baseConfig.PrivValidatorKey),
		"privState": filepath.Join(rootDir, baseConfig.PrivValidatorState),
	}

	if !bcos.FileExists(paths["config"]) {
		writeDefaultConfigFile(paths["config"])
	}

	if !bcos.FileExists(paths["genesis"]) {
		if chainID == "" {
			chainID = "baron-chain-test"
		}
		writeGenesisFile(paths["genesis"], chainID)
	}

	// Generate quantum-safe keys
	privKey, pubKey, err := kyber.GenerateKeyPair()
	if err != nil {
		panic(fmt.Sprintf("Failed to generate quantum-safe keys: %v", err))
	}

	writePrivValidatorFiles(paths["privKey"], paths["privState"], privKey, pubKey)

	return TestConfig().SetRoot(rootDir)
}

func writeGenesisFile(path, chainID string) {
	genesis := fmt.Sprintf(testGenesisFmt, chainID)
	bcos.MustWriteFile(path, []byte(genesis), 0644)
}

func writePrivValidatorFiles(keyPath, statePath string, privKey, pubKey []byte) {
	keyData := generatePrivValidatorKey(privKey, pubKey)
	stateData := generatePrivValidatorState()

	bcos.MustWriteFile(keyPath, keyData, 0644)
	bcos.MustWriteFile(statePath, stateData, 0644)
}

func generatePrivValidatorKey(privKey, pubKey []byte) []byte {
	return []byte(fmt.Sprintf(`{
  "address": "%x",
  "pub_key": {
    "type": "baronchain/PubKeyKyber",
    "value": "%x"
  },
  "priv_key": {
    "type": "baronchain/PrivKeyKyber",
    "value": "%x"
  }
}`, kyber.AddressFromPubKey(pubKey), pubKey, privKey))
}

func generatePrivValidatorState() []byte {
	return []byte(`{
  "height": "0",
  "round": 0,
  "step": 0
}`)
}

const testGenesisFmt = `{
  "genesis_time": "2024-01-01T00:00:00.000000000Z",
  "chain_id": "%s",
  "initial_height": "1",
  "consensus_params": {
    "block": {
      "max_bytes": "22020096",
      "max_gas": "-1",
      "time_iota_ms": "10"
    },
    "evidence": {
      "max_age_num_blocks": "100000",
      "max_age_duration": "172800000000000",
      "max_bytes": "1048576"
    },
    "validator": {
      "pub_key_types": [
        "kyber"
      ]
    },
    "version": {}
  },
  "validators": [],
  "app_hash": ""
}`

// Main config template definition remains the same but with Baron Chain specific changes
const defaultConfigTemplate = `# Baron Chain Configuration File
...
`
/****** these are for test settings ***********/

func ResetTestRoot(testName string) *Config {
	return ResetTestRootWithChainID(testName, "")
}

func ResetTestRootWithChainID(testName string, chainID string) *Config {
	// create a unique, concurrency-safe test directory under os.TempDir()
	rootDir, err := os.MkdirTemp("", fmt.Sprintf("%s-%s_", chainID, testName))
	if err != nil {
		panic(err)
	}
	// ensure config and data subdirs are created
	if err := cmtos.EnsureDir(filepath.Join(rootDir, defaultConfigDir), DefaultDirPerm); err != nil {
		panic(err)
	}
	if err := cmtos.EnsureDir(filepath.Join(rootDir, defaultDataDir), DefaultDirPerm); err != nil {
		panic(err)
	}

	baseConfig := DefaultBaseConfig()
	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)
	genesisFilePath := filepath.Join(rootDir, baseConfig.Genesis)
	privKeyFilePath := filepath.Join(rootDir, baseConfig.PrivValidatorKey)
	privStateFilePath := filepath.Join(rootDir, baseConfig.PrivValidatorState)

	// Write default config file if missing.
	if !cmtos.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
	if !cmtos.FileExists(genesisFilePath) {
		if chainID == "" {
			chainID = "cometbft_test"
		}
		testGenesis := fmt.Sprintf(testGenesisFmt, chainID)
		cmtos.MustWriteFile(genesisFilePath, []byte(testGenesis), 0644)
	}
	// we always overwrite the priv val
	cmtos.MustWriteFile(privKeyFilePath, []byte(testPrivValidatorKey), 0644)
	cmtos.MustWriteFile(privStateFilePath, []byte(testPrivValidatorState), 0644)

	config := TestConfig().SetRoot(rootDir)
	return config
}

var testGenesisFmt = `{
  "genesis_time": "2018-10-10T08:20:13.695936996Z",
  "chain_id": "%s",
  "initial_height": "1",
	"consensus_params": {
		"block": {
			"max_bytes": "22020096",
			"max_gas": "-1",
			"time_iota_ms": "10"
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800000000000",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": [
				"ed25519"
			]
		},
		"version": {}
	},
  "validators": [
    {
      "pub_key": {
        "type": "tendermint/PubKeyEd25519",
        "value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="
      },
      "power": "10",
      "name": ""
    }
  ],
  "app_hash": ""
}`

var testPrivValidatorKey = `{
  "address": "A3258DCBF45DCA0DF052981870F2D1441A36D145",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "EVkqJO/jIXp3rkASXfh9YnyToYXRXhBr6g9cQVxPFnQBP/5povV4HTjvsy530kybxKHwEi85iU8YL0qQhSYVoQ=="
  }
}`

var testPrivValidatorState = `{
  "height": "0",
  "round": 0,
  "step": 0
}`
