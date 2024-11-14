package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/baron-chain/cometbft-bc/abci/types"
	"github.com/baron-chain/cometbft-bc/crypto"
	"github.com/baron-chain/cometbft-bc/libs/log"
)

const (
	snapshotChunkSize = 1 << 20 // 1MB chunks
	metadataFile     = "metadata.json"
	snapshotFileExt  = ".json"
	defaultFileMode  = 0644
	defaultDirMode   = 0755
)

// SnapshotStore manages state sync snapshots for Baron Chain
type SnapshotStore struct {
	sync.RWMutex
	dir      string
	metadata []types.Snapshot
	logger   log.Logger
}

// SnapshotMetadata represents snapshot metadata stored on disk
type SnapshotMetadata struct {
	Height    uint64            `json:"height"`
	Format    uint32            `json:"format"`
	Hash      []byte           `json:"hash"`
	Chunks    uint32           `json:"chunks"`
	Timestamp int64            `json:"timestamp"`
	Version   string           `json:"version"`
}

// NewSnapshotStore creates a new snapshot store instance
func NewSnapshotStore(dir string) (*SnapshotStore, error) {
	if err := os.MkdirAll(dir, defaultDirMode); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	store := &SnapshotStore{
		dir:    dir,
		logger: log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", "snapshots"),
	}

	if err := store.loadMetadata(); err != nil {
		return nil, fmt.Errorf("failed to load snapshot metadata: %w", err)
	}

	return store, nil
}

// loadMetadata loads snapshot metadata from disk
func (s *SnapshotStore) loadMetadata() error {
	path := filepath.Join(s.dir, metadataFile)
	
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s.metadata = make([]types.Snapshot, 0)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata []types.Snapshot
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	s.metadata = metadata
	s.logger.Info("Loaded snapshot metadata", "count", len(metadata))
	return nil
}

// saveMetadata saves snapshot metadata atomically
func (s *SnapshotStore) saveMetadata() error {
	data, err := json.Marshal(s.metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	tempPath := filepath.Join(s.dir, metadataFile+".tmp")
	finalPath := filepath.Join(s.dir, metadataFile)

	if err := atomicWrite(tempPath, finalPath, data); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	s.logger.Info("Saved snapshot metadata", "count", len(s.metadata))
	return nil
}

// Create creates a new snapshot of the current state
func (s *SnapshotStore) Create(state *State) (types.Snapshot, error) {
	s.Lock()
	defer s.Unlock()

	// Export state data
	data, err := state.Export()
	if err != nil {
		return types.Snapshot{}, fmt.Errorf("failed to export state: %w", err)
	}

	// Create snapshot
	snapshot := types.Snapshot{
		Height:  state.Height,
		Format:  1,
		Hash:    crypto.Sha256(data), // Use Baron Chain's crypto package
		Chunks:  calculateChunks(len(data)),
	}

	// Save snapshot file
	filename := fmt.Sprintf("%d%s", state.Height, snapshotFileExt)
	if err := s.saveSnapshotFile(filename, data); err != nil {
		return types.Snapshot{}, err
	}

	// Update metadata
	s.metadata = append(s.metadata, snapshot)
	if err := s.saveMetadata(); err != nil {
		return types.Snapshot{}, err
	}

	s.logger.Info("Created new snapshot", 
		"height", snapshot.Height,
		"chunks", snapshot.Chunks,
		"size", len(data))

	return snapshot, nil
}

// List returns available snapshots
func (s *SnapshotStore) List() ([]*types.Snapshot, error) {
	s.RLock()
	defer s.RUnlock()

	snapshots := make([]*types.Snapshot, len(s.metadata))
	for i := range s.metadata {
		snapshots[i] = &s.metadata[i]
	}
	return snapshots, nil
}

// LoadChunk loads a specific chunk of a snapshot
func (s *SnapshotStore) LoadChunk(height uint64, format uint32, chunk uint32) ([]byte, error) {
	s.RLock()
	defer s.RUnlock()

	snapshot := s.findSnapshot(height, format)
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot not found: height=%d format=%d", height, format)
	}

	data, err := os.ReadFile(s.getSnapshotPath(height))
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	return extractChunk(data, chunk), nil
}

// Helper functions

func (s *SnapshotStore) findSnapshot(height uint64, format uint32) *types.Snapshot {
	for i := range s.metadata {
		if s.metadata[i].Height == height && s.metadata[i].Format == format {
			return &s.metadata[i]
		}
	}
	return nil
}

func (s *SnapshotStore) getSnapshotPath(height uint64) string {
	return filepath.Join(s.dir, fmt.Sprintf("%d%s", height, snapshotFileExt))
}

func (s *SnapshotStore) saveSnapshotFile(filename string, data []byte) error {
	path := filepath.Join(s.dir, filename)
	return atomicWrite(path+".tmp", path, data)
}

func calculateChunks(size int) uint32 {
	return uint32(math.Ceil(float64(size) / float64(snapshotChunkSize)))
}

func extractChunk(data []byte, index uint32) []byte {
	start := index * snapshotChunkSize
	if start >= uint32(len(data)) {
		return nil
	}

	end := uint32(math.Min(float64(start+snapshotChunkSize), float64(len(data))))
	return data[start:end]
}

func atomicWrite(tempPath, finalPath string, data []byte) error {
	if err := os.WriteFile(tempPath, data, defaultFileMode); err != nil {
		return err
	}
	return os.Rename(tempPath, finalPath)
}
