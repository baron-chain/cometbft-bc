package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/baron-chain/cometbft-bc/crypto"
	"github.com/baron-chain/cometbft-bc/libs/log"
)

const (
	currentStateFile  = "baron_state.json"
	backupStateFile  = "baron_state.bak"
	defaultFilePerms = 0644
	defaultDirPerms  = 0755
)

// State represents Baron Chain's application state
type State struct {
	sync.RWMutex
	Height        uint64            `json:"height"`
	Values        map[string]string `json:"values"`
	Hash          []byte           `json:"hash"`
	Version       string           `json:"version"`
	
	// private fields
	logger          log.Logger
	currentFilePath string
	backupFilePath  string
	persistInterval uint64
	initialHeight   uint64
}

// StateConfig holds state configuration
type StateConfig struct {
	DataDir         string
	PersistInterval uint64
	Logger          log.Logger
}

// NewState creates a new Baron Chain state instance
func NewState(cfg *StateConfig) (*State, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir, defaultDirPerms); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	state := &State{
		Values:          make(map[string]string),
		currentFilePath: filepath.Join(cfg.DataDir, currentStateFile),
		backupFilePath:  filepath.Join(cfg.DataDir, backupStateFile),
		persistInterval: cfg.PersistInterval,
		logger:         cfg.Logger.With("module", "state"),
		Version:       "1.0.0", // Baron Chain version
	}

	if err := state.load(); err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	state.Hash = state.computeHash()
	return state, nil
}

func validateConfig(cfg *StateConfig) error {
	if cfg.DataDir == "" {
		return fmt.Errorf("data directory not specified")
	}
	if cfg.Logger == nil {
		return fmt.Errorf("logger not specified")
	}
	return nil
}

// load loads state from disk
func (s *State) load() error {
	data, err := os.ReadFile(s.currentFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read current state: %w", err)
		}
		
		// Try loading backup state
		data, err = os.ReadFile(s.backupFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				s.logger.Info("No existing state found, starting fresh")
				return nil
			}
			return fmt.Errorf("failed to read backup state: %w", err)
		}
		s.logger.Info("Recovered state from backup")
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	s.logger.Info("Loaded state", "height", s.Height, "values", len(s.Values))
	return nil
}

// save persists state to disk atomically
func (s *State) save() error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tempFile := s.currentFilePath + ".tmp"
	if err := atomicWrite(tempFile, s.currentFilePath, s.backupFilePath, data); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	s.logger.Info("Saved state", "height", s.Height, "hash", fmt.Sprintf("%X", s.Hash))
	return nil
}

// Export exports state for snapshots
func (s *State) Export() ([]byte, error) {
	s.RLock()
	defer s.RUnlock()

	data, err := json.Marshal(s.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to export state: %w", err)
	}
	return data, nil
}

// Import imports state from snapshots or genesis
func (s *State) Import(height uint64, data []byte) error {
	s.Lock()
	defer s.Unlock()

	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("failed to import state data: %w", err)
	}

	s.Height = height
	s.Values = values
	s.Hash = s.computeHash()

	if err := s.save(); err != nil {
		return fmt.Errorf("failed to save imported state: %w", err)
	}

	s.logger.Info("Imported state", "height", height, "values", len(values))
	return nil
}

// Get retrieves a value from state
func (s *State) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.Values[key]
}

// Set sets a value in state
func (s *State) Set(key, value string) error {
	s.Lock()
	defer s.Unlock()

	if value == "" {
		delete(s.Values, key)
	} else {
		s.Values[key] = value
	}
	return nil
}

// Commit commits state changes
func (s *State) Commit() (height uint64, hash []byte, err error) {
	s.Lock()
	defer s.Unlock()

	s.Hash = s.computeHash()
	s.Height = s.nextHeight()

	if s.shouldPersist() {
		if err := s.save(); err != nil {
			return 0, nil, fmt.Errorf("failed to commit state: %w", err)
		}
	}

	return s.Height, s.Hash, nil
}

// Rollback restores previous state
func (s *State) Rollback() error {
	s.Lock()
	defer s.Unlock()

	data, err := os.ReadFile(s.backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to read backup state: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to restore backup state: %w", err)
	}

	s.logger.Info("Rolled back state", "height", s.Height)
	return nil
}

// Helper functions

func (s *State) computeHash() []byte {
	keys := make([]string, 0, len(s.Values))
	for key := range s.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hasher := crypto.New256()
	for _, key := range keys {
		hasher.Write([]byte(key))
		hasher.Write([]byte{0})
		hasher.Write([]byte(s.Values[key]))
		hasher.Write([]byte{0})
	}
	return hasher.Sum(nil)
}

func (s *State) nextHeight() uint64 {
	if s.Height > 0 {
		return s.Height + 1
	}
	if s.initialHeight > 0 {
		return s.initialHeight
	}
	return 1
}

func (s *State) shouldPersist() bool {
	return s.persistInterval > 0 && s.Height%s.persistInterval == 0
}

func atomicWrite(tempPath, finalPath, backupPath string, data []byte) error {
	if err := os.WriteFile(tempPath, data, defaultFilePerms); err != nil {
		return err
	}

	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Rename(finalPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	return os.Rename(tempPath, finalPath)
}
