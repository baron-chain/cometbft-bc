package e2e

import (
    "fmt"
    "os"
    "time"

    "github.com/BurntSushi/toml"
)

type Manifest struct {
    IPv6 bool `toml:"ipv6"`
    InitialHeight int64 `toml:"initial_height"`
    InitialState map[string]string `toml:"initial_state"`
    Validators *map[string]int64 `toml:"validators"`
    ValidatorUpdates map[string]map[string]int64 `toml:"validator_update"`
    Nodes map[string]*ManifestNode `toml:"node"`
    KeyType string `toml:"key_type"`
    Evidence int `toml:"evidence"`
    ABCIProtocol string `toml:"abci_protocol"`
    PrepareProposalDelay time.Duration `toml:"prepare_proposal_delay"`
    ProcessProposalDelay time.Duration `toml:"process_proposal_delay"`
    CheckTxDelay time.Duration `toml:"check_tx_delay"`
    UpgradeVersion string `toml:"upgrade_version"`
    LoadTxSizeBytes int `toml:"load_tx_size_bytes"`
    LoadTxBatchSize int `toml:"load_tx_batch_size"`
    LoadTxConnections int `toml:"load_tx_connections"`
    Prometheus bool `toml:"prometheus"`
}

type ManifestNode struct {
    Mode string `toml:"mode"`
    Version string `toml:"version"`
    Seeds []string `toml:"seeds"`
    PersistentPeers []string `toml:"persistent_peers"`
    Database string `toml:"database"`
    PrivvalProtocol string `toml:"privval_protocol"`
    StartAt int64 `toml:"start_at"`
    BlockSync string `toml:"block_sync"`
    Mempool string `toml:"mempool_version"`
    StateSync bool `toml:"state_sync"`
    PersistInterval *uint64 `toml:"persist_interval"`
    SnapshotInterval uint64 `toml:"snapshot_interval"`
    RetainBlocks uint64 `toml:"retain_blocks"`
    Perturb []string `toml:"perturb"`
    SendNoLoad bool `toml:"send_no_load"`
}

func (m *Manifest) Save(file string) error {
    f, err := os.Create(file)
    if err != nil {
        return fmt.Errorf("failed to create manifest file %q: %w", file, err)
    }
    defer f.Close()

    encoder := toml.NewEncoder(f)
    encoder.Indent = "    "
    return encoder.Encode(m)
}

func LoadManifest(file string) (Manifest, error) {
    var manifest Manifest
    if _, err := toml.DecodeFile(file, &manifest); err != nil {
        return manifest, fmt.Errorf("failed to load manifest %q: %w", file, err)
    }

    if err := validateManifest(&manifest); err != nil {
        return manifest, fmt.Errorf("invalid manifest: %w", err)
    }

    manifest = applyDefaults(manifest)
    return manifest, nil
}

func validateManifest(m *Manifest) error {
    if len(m.Nodes) == 0 {
        return fmt.Errorf("no nodes specified")
    }

    for name, node := range m.Nodes {
        if err := validateNode(name, node); err != nil {
            return fmt.Errorf("invalid node %q: %w", name, err)
        }
    }

    return nil
}

func validateNode(name string, node *ManifestNode) error {
    switch node.Mode {
    case "", "validator", "full", "light", "seed":
    default:
        return fmt.Errorf("invalid mode: %q", node.Mode)
    }

    switch node.Database {
    case "", "goleveldb", "cleveldb", "rocksdb", "boltdb", "badgerdb":
    default:
        return fmt.Errorf("invalid database: %q", node.Database)
    }

    switch node.BlockSync {
    case "", "v0", "v2":
    default:
        return fmt.Errorf("invalid block sync: %q", node.BlockSync)
    }

    switch node.Mempool {
    case "", "v0", "v1":
    default:
        return fmt.Errorf("invalid mempool version: %q", node.Mempool)
    }

    switch node.PrivvalProtocol {
    case "", "file", "unix", "tcp":
    default:
        return fmt.Errorf("invalid privval protocol: %q", node.PrivvalProtocol)
    }

    return nil
}

func applyDefaults(m Manifest) Manifest {
    if m.InitialHeight == 0 {
        m.InitialHeight = 1
    }

    if m.ABCIProtocol == "" {
        m.ABCIProtocol = "builtin"
    }

    for _, node := range m.Nodes {
        if node.Mode == "" {
            node.Mode = "validator"
        }
        if node.Database == "" {
            node.Database = "goleveldb"
        }
        if node.PrivvalProtocol == "" {
            node.PrivvalProtocol = "file"
        }
        if node.PersistInterval == nil {
            interval := uint64(1)
            node.PersistInterval = &interval
        }
    }

    return m
}
