package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/p2p"
)

const (
	defaultBaseDir    = "."
	corpusDirName    = "corpus"
	dirPermissions   = 0o755
	filePermissions  = 0o644
	fileExtension    = ".json"
)

var (
	// Error definitions
	ErrDirCreation   = errors.New("failed to create directory")
	ErrMarshaling    = errors.New("failed to marshal address")
	ErrFileWrite     = errors.New("failed to write file")
)

// Config holds the application configuration
type Config struct {
	BaseDir string
	Logger  *log.Logger
}

// TestAddress represents a test network address configuration
type TestAddress struct {
	IP   net.IP
	Port uint16
}

// AddressGenerator handles the generation of test network addresses
type AddressGenerator struct {
	privKey ed25519.PrivKey
	nodeID  p2p.ID
}

// NewAddressGenerator creates a new address generator
func NewAddressGenerator() *AddressGenerator {
	privKey := ed25519.GenPrivKey()
	return &AddressGenerator{
		privKey: privKey,
		nodeID:  p2p.PubKeyToID(privKey.PubKey()),
	}
}

// CreateNetAddress creates a p2p.NetAddress from TestAddress
func (g *AddressGenerator) CreateNetAddress(ta TestAddress) *p2p.NetAddress {
	return &p2p.NetAddress{
		ID:   g.nodeID,
		IP:   ta.IP,
		Port: ta.Port,
	}
}

// GetTestAddresses returns the list of test addresses to generate
func GetTestAddresses() []TestAddress {
	return []TestAddress{
		{IP: net.IPv4(0, 0, 0, 0), Port: 0},
		{IP: net.IPv4(127, 0, 0, 0), Port: 80},
		{IP: net.IPv4(213, 87, 10, 200), Port: 8808},
		{IP: net.IPv4(111, 111, 111, 111), Port: 26656},
		{IP: net.ParseIP("2001:db8::68"), Port: 26656},
	}
}

// CreateCorpusDirectory creates the corpus directory if it doesn't exist
func CreateCorpusDirectory(baseDir string) (string, error) {
	corpusDir := filepath.Join(baseDir, corpusDirName)
	if err := os.MkdirAll(corpusDir, dirPermissions); err != nil {
		return "", fmt.Errorf("%w: %v", ErrDirCreation, err)
	}
	return corpusDir, nil
}

// WriteAddressFile writes a single address to a corpus file
func WriteAddressFile(corpusDir string, index int, addr *p2p.NetAddress) error {
	filename := filepath.Join(corpusDir, fmt.Sprintf("%d%s", index, fileExtension))
	
	data, err := json.MarshalIndent(addr, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMarshaling, err)
	}
	
	if err := os.WriteFile(filename, data, filePermissions); err != nil {
		return fmt.Errorf("%w: %v", ErrFileWrite, err)
	}
	
	return nil
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

// InitCorpus initializes the corpus with test addresses
func InitCorpus(cfg *Config) error {
	corpusDir, err := CreateCorpusDirectory(cfg.BaseDir)
	if err != nil {
		return err
	}

	generator := NewAddressGenerator()
	testAddrs := GetTestAddresses()

	for i, testAddr := range testAddrs {
		addr := generator.CreateNetAddress(testAddr)
		
		if err := WriteAddressFile(corpusDir, i, addr); err != nil {
			return fmt.Errorf("failed to write address %d: %w", i, err)
		}
		
		cfg.Logger.Printf("wrote address %d to corpus", i)
	}

	return nil
}

func main() {
	cfg := InitConfig()

	if err := InitCorpus(cfg); err != nil {
		cfg.Logger.Fatalf("Initialization failed: %v", err)
	}
}
