package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/baron-chain/cometbft-bc/crypto/kyber"
	"github.com/baron-chain/cometbft-bc/libs/log"
	bcnet "github.com/baron-chain/cometbft-bc/libs/net"
	bcos "github.com/baron-chain/cometbft-bc/libs/os"
	"github.com/baron-chain/cometbft-bc/privval"
)

const (
	defaultTimeout = 3 * time.Second
	defaultPort    = "26659"
	moduleName     = "priv_val"
)

type ValidatorConfig struct {
	addr           string
	chainID        string
	keyPath        string
	statePath      string
	connTimeout    time.Duration
	quantumSafe    bool
}

type PrivValidator struct {
	config  ValidatorConfig
	logger  log.Logger
	server  *privval.SignerServer
	dialer  privval.SocketDialer
	kyberPK *kyber.PrivateKey
}

func parseFlags() ValidatorConfig {
	config := ValidatorConfig{
		connTimeout: defaultTimeout,
		quantumSafe: true,
	}

	flag.StringVar(&config.addr, "addr", ":"+defaultPort, "Address of client to connect to")
	flag.StringVar(&config.chainID, "chain-id", "baron-chain", "chain id")
	flag.StringVar(&config.keyPath, "priv-key", "", "priv val key file path")
	flag.StringVar(&config.statePath, "priv-state", "", "priv val state file path")
	flag.BoolVar(&config.quantumSafe, "quantum-safe", true, "enable quantum-safe cryptography")
	
	flag.Parse()
	return config
}

func NewPrivValidator(config ValidatorConfig) (*PrivValidator, error) {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", moduleName)

	// Initialize quantum-safe keys if enabled
	var kyberPK *kyber.PrivateKey
	var err error
	if config.quantumSafe {
		kyberPK, err = kyber.GeneratePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate quantum-safe keys: %w", err)
		}
	}

	pv := &PrivValidator{
		config:  config,
		logger:  logger,
		kyberPK: kyberPK,
	}

	return pv, nil
}

func (pv *PrivValidator) initDialer() error {
	protocol, address := bcnet.ProtocolAndAddress(pv.config.addr)
	
	switch protocol {
	case "unix":
		pv.dialer = privval.DialUnixFn(address)
	case "tcp":
		if pv.config.quantumSafe {
			pv.dialer = privval.NewQuantumSafeDialTCPFn(address, pv.config.connTimeout, pv.kyberPK)
		} else {
			pv.dialer = privval.DialTCPFn(address, pv.config.connTimeout, pv.kyberPK)
		}
	default:
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
	
	return nil
}

func (pv *PrivValidator) Start(ctx context.Context) error {
	pv.logger.Info("Starting private validator",
		"addr", pv.config.addr,
		"chainID", pv.config.chainID,
		"keyPath", pv.config.keyPath,
		"statePath", pv.config.statePath,
		"quantumSafe", pv.config.quantumSafe,
	)

	filePV := privval.LoadFilePV(pv.config.keyPath, pv.config.statePath)

	if err := pv.initDialer(); err != nil {
		return fmt.Errorf("failed to initialize dialer: %w", err)
	}

	endpoint := privval.NewSignerDialerEndpoint(pv.logger, pv.dialer)
	pv.server = privval.NewSignerServer(endpoint, pv.config.chainID, filePV)

	if err := pv.server.Start(); err != nil {
		return fmt.Errorf("failed to start signer server: %w", err)
	}

	return nil
}

func (pv *PrivValidator) Stop() error {
	if pv.server != nil {
		if err := pv.server.Stop(); err != nil {
			return fmt.Errorf("failed to stop signer server: %w", err)
		}
	}
	return nil
}

func main() {
	config := parseFlags()
	
	pv, err := NewPrivValidator(config)
	if err != nil {
		fmt.Printf("Failed to initialize validator: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pv.Start(ctx); err != nil {
		fmt.Printf("Failed to start validator: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	bcos.TrapSignal(pv.logger, func() {
		if err := pv.Stop(); err != nil {
			pv.logger.Error("Error stopping validator", "err", err)
		}
	})

	// Run forever
	<-ctx.Done()
}
