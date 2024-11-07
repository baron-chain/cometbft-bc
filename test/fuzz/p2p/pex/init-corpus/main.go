package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/p2p"
	tmp2p "github.com/cometbft/cometbft/proto/tendermint/p2p"
)

const (
	defaultBaseDir    = "."
	corpusDirName    = "corpus"
	dirPermissions   = 0o755
	filePermissions  = 0o644
	defaultPort      = 26656
	ipv6TestAddress  = "ff02::1:114"
	randSeed         = 10
)

var (
	// Test network sizes for corpus generation
	testSizes = []int{0, 1, 2, 17, 5, 31}

	// Error definitions
	errDirCreation   = errors.New("failed to create directory")
	errAddrCreation  = errors.New("failed to create network address")
	errMarshaling    = errors.New("failed to marshal message")
	errFileWrite     = errors.New("failed to write file")
)

// Config holds the application configuration
type Config struct {
	BaseDir     string
	Logger      *log.Logger
	RandSource  rand.Source
}

// AddressGenerator handles the generation of test network addresses
type AddressGenerator struct {
	rand *rand.Rand
}

// NewAddressGenerator creates a new address generator with a specific seed
func NewAddressGenerator(seed int64) *AddressGenerator {
	return &AddressGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// generateIPv4 generates a random IPv4 address string
func (g *AddressGenerator) generateIPv4() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		g.rand.Int()%256,
		g.rand.Int()%256,
		g.rand.Int()%256,
		g.rand.Int()%256,
	)
}

// GenerateAddress generates a complete network address with ID and IP
func (g *AddressGenerator) GenerateAddress(isIPv6 bool) (*p2p.NetAddress, error) {
	privKey := ed25519.GenPrivKey()
	nodeID := p2p.PubKeyToID(privKey.PubKey())

	var addrString string
	if isIPv6 {
		addrString = fmt.Sprintf("%s@[%s]:%d", nodeID, ipv6TestAddress, defaultPort)
	} else {
		addrString = fmt.Sprintf("%s@%s:%d", nodeID, g.generateIPv4(), defaultPort)
	}

	addr, err := p2p.NewNetAddressString(addrString)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAddrCreation, err)
	}

	return addr, nil
}

// CreateCorpusDirectory creates the corpus directory if it doesn't exist
func CreateCorpusDirectory(baseDir string) (string, error) {
	corpusDir := filepath.Join(baseDir, corpusDirName)
	if err := os.MkdirAll(corpusDir, dirPermissions); err != nil {
		return "", fmt.Errorf("%w: %v", errDirCreation, err)
	}
	return corpusDir, nil
}

// GenerateAddresses generates a set of network addresses for testing
func GenerateAddresses(gen *AddressGenerator, count int) ([]*p2p.NetAddress, error) {
	addrs := make([]*p2p.NetAddress, 0, count+1) // +1 for IPv6 address

	// Generate IPv4 addresses
	for i := 0; i < count; i++ {
		addr, err := gen.GenerateAddress(false)
		if err != nil {
			return nil, fmt.Errorf("failed to generate IPv4 address %d: %w", i, err)
		}
		addrs = append(addrs, addr)
	}

	// Add an IPv6 address
	addr, err := gen.GenerateAddress(true)
	if err != nil {
		return nil, fmt.Errorf("failed to generate IPv6 address: %w", err)
	}
	addrs = append(addrs, addr)

	return addrs, nil
}

// CreatePexMessage creates a PEX message with the given addresses
func CreatePexMessage(addrs []*p2p.NetAddress) ([]byte, error) {
	msg := tmp2p.Message{
		Sum: &tmp2p.Message_PexAddrs{
			PexAddrs: &tmp2p.PexAddrs{
				Addrs: p2p.NetAddressesToProto(addrs),
			},
		},
	}

	bz, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errMarshaling, err)
	}

	return bz, nil
}

// WriteCorpusFile writes the message bytes to a corpus file
func WriteCorpusFile(corpusDir string, size int, data []byte) error {
	filename := filepath.Join(corpusDir, fmt.Sprintf("%d", size))
	if err := os.WriteFile(filename, data, filePermissions); err != nil {
		return fmt.Errorf("%w: %v", errFileWrite, err)
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

// InitCorpus initializes the corpus with test data
func InitCorpus(cfg *Config) error {
	corpusDir, err := CreateCorpusDirectory(cfg.BaseDir)
	if err != nil {
		return err
	}

	addrGen := NewAddressGenerator(randSeed)

	for _, size := range testSizes {
		addrs, err := GenerateAddresses(addrGen, size)
		if err != nil {
			return fmt.Errorf("failed to generate addresses for size %d: %w", size, err)
		}

		msg, err := CreatePexMessage(addrs)
		if err != nil {
			return fmt.Errorf("failed to create message for size %d: %w", size, err)
		}

		if err := WriteCorpusFile(corpusDir, size, msg); err != nil {
			return fmt.Errorf("failed to write corpus file for size %d: %w", size, err)
		}

		cfg.Logger.Printf("wrote corpus file for size %d", size)
	}

	return nil
}

func main() {
	cfg := InitConfig()

	if err := InitCorpus(cfg); err != nil {
		cfg.Logger.Fatalf("Initialization failed: %v", err)
	}
}
