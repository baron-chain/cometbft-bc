package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultBaseDir = "."
	corpusDirName = "corpus"
	dirPermissions = 0o755
	filePermissions = 0o644
)

var (
	// ErrEmptyPath is returned when the provided path is empty
	ErrEmptyPath = errors.New("empty path provided")
	// ErrInvalidPath is returned when the path is invalid
	ErrInvalidPath = errors.New("invalid path")
	// ErrFileCreation is returned when file creation fails
	ErrFileCreation = errors.New("failed to create file")
)

// Config holds the application configuration
type Config struct {
	BaseDir string
	Logger  *log.Logger
}

// CorpusData represents the test corpus data
type CorpusData struct {
	Samples []string
}

// NewDefaultCorpusData returns the default corpus data samples
func NewDefaultCorpusData() *CorpusData {
	return &CorpusData{
		Samples: []string{
			"dadc04c2-cfb1-4aa9-a92a-c0bf780ec8b6", // UUID format
			"",                                       // Empty string
			" ",                                      // Single space
			"           a                                   ", // Padded string
			`{"a": 12, "tsp": 999, k: "blue"}`,            // JSON-like format
			`9999.999`,                                     // Number format
			`""`,                                           // Quoted empty string
			`CometBFT fuzzing`,                             // Plain text
		},
	}
}

// InitConfig initializes the application configuration
func InitConfig() *Config {
	cfg := &Config{
		Logger: log.New(os.Stdout, "", 0),
	}

	flag.StringVar(&cfg.BaseDir, "base", defaultBaseDir, `where the "corpus" directory will live`)
	flag.Parse()

	return cfg
}

// ValidatePath ensures the provided path is valid
func ValidatePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	// Convert to absolute path for better validation
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}

	// Check if parent directory exists and is accessible
	parentDir := filepath.Dir(absPath)
	if _, err := os.Stat(parentDir); err != nil {
		return fmt.Errorf("%w: parent directory not accessible: %v", ErrInvalidPath, err)
	}

	return nil
}

// CreateCorpusDirectory creates the corpus directory if it doesn't exist
func CreateCorpusDirectory(baseDir string) (string, error) {
	if err := ValidatePath(baseDir); err != nil {
		return "", err
	}

	corpusDir := filepath.Join(baseDir, corpusDirName)
	if err := os.MkdirAll(corpusDir, dirPermissions); err != nil {
		return "", fmt.Errorf("failed to create corpus directory: %w", err)
	}

	return corpusDir, nil
}

// WriteCorpusSample writes a single corpus sample to file
func WriteCorpusSample(corpusDir, filename string, data []byte) error {
	fullPath := filepath.Join(corpusDir, filename)
	
	if err := os.WriteFile(fullPath, data, filePermissions); err != nil {
		return fmt.Errorf("%w: %v", ErrFileCreation, err)
	}
	
	return nil
}

// InitCorpus initializes the corpus directory and writes sample data
func InitCorpus(cfg *Config) error {
	corpusDir, err := CreateCorpusDirectory(cfg.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to initialize corpus directory: %w", err)
	}

	corpus := NewDefaultCorpusData()
	for i, sample := range corpus.Samples {
		filename := fmt.Sprintf("%d", i)
		if err := WriteCorpusSample(corpusDir, filename, []byte(sample)); err != nil {
			return fmt.Errorf("failed to write sample %d: %w", i, err)
		}
		cfg.Logger.Printf("wrote %q", filepath.Join(corpusDir, filename))
	}

	return nil
}

func main() {
	cfg := InitConfig()

	if err := InitCorpus(cfg); err != nil {
		cfg.Logger.Fatalf("Initialization failed: %v", err)
	}
}
