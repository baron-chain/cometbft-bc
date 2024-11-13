package main

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/spf13/viper"

    "github.com/baron-chain/cometbft-bc/abci/server"
    "github.com/baron-chain/cometbft-bc/config"
    "github.com/baron-chain/cometbft-bc/crypto/ed25519"
    "github.com/baron-chain/cometbft-bc/crypto/kyber"
    bcflags "github.com/baron-chain/cometbft-bc/libs/cli/flags"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcnet "github.com/baron-chain/cometbft-bc/libs/net"
    "github.com/baron-chain/cometbft-bc/light"
    lproxy "github.com/baron-chain/cometbft-bc/light/proxy"
    lrpc "github.com/baron-chain/cometbft-bc/light/rpc"
    dbs "github.com/baron-chain/cometbft-bc/light/store/db"
    "github.com/baron-chain/cometbft-bc/node"
    "github.com/baron-chain/cometbft-bc/p2p"
    "github.com/baron-chain/cometbft-bc/privval"
    "github.com/baron-chain/cometbft-bc/proxy"
    rpcserver "github.com/baron-chain/cometbft-bc/rpc/jsonrpc/server"
    "github.com/baron-chain/cometbft-bc/test/e2e/app"
    e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

const (
    defaultConfigFile = "config.toml"
    defaultTimeout    = 3 * time.Second
    defaultRetries    = 100
    rpcDefaultPort   = 26657
    p2pDefaultPort   = 26656
    metricsPort      = 26660
)

var (
    logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
    envHome = "BARON_HOME"
)

func main() {
    if err := run(parseArgs()); err != nil {
        logger.Error("application failed", "err", err)
        os.Exit(1)
    }
}

func parseArgs() string {
    if len(os.Args) != 2 {
        fmt.Printf("Usage: %v <configfile>\n", os.Args[0])
        os.Exit(1)
    }
    return os.Args[1]
}

func run(configFile string) error {
    cfg, err := LoadConfig(configFile)
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    if cfg.PrivValServer != "" {
        if err = setupPrivValidator(cfg); err != nil {
            return fmt.Errorf("failed to setup private validator: %w", err)
        }
    }

    return startService(cfg)
}

func startService(cfg *Config) error {
    switch cfg.Protocol {
    case "socket", "grpc":
        return startApp(cfg)
    case "builtin":
        if cfg.Mode == string(e2e.ModeLight) {
            return startLightClient(cfg)
        }
        return startNode(cfg)
    default:
        return fmt.Errorf("unsupported protocol: %q", cfg.Protocol)
    }
}

func startApp(cfg *Config) error {
    app, err := app.NewApplication(cfg.App())
    if err != nil {
        return fmt.Errorf("failed to create application: %w", err)
    }

    server, err := server.NewServer(cfg.Listen, cfg.Protocol, app)
    if err != nil {
        return fmt.Errorf("failed to create server: %w", err)
    }

    if err := server.Start(); err != nil {
        return fmt.Errorf("failed to start server: %w", err)
    }

    logger.Info("application started", 
        "listen_addr", cfg.Listen,
        "protocol", cfg.Protocol)
    
    return waitForever()
}

func startNode(cfg *Config) error {
    app, err := app.NewApplication(cfg.App())
    if err != nil {
        return fmt.Errorf("failed to create application: %w", err)
    }

    nodeCfg, nodeLogger, nodeKey, err := setupNodeConfig()
    if err != nil {
        return fmt.Errorf("failed to setup node: %w", err)
    }

    node, err := createNode(nodeCfg, nodeLogger, nodeKey, app)
    if err != nil {
        return fmt.Errorf("failed to create node: %w", err)
    }

    if err := node.Start(); err != nil {
        return fmt.Errorf("failed to start node: %w", err)
    }

    return nil
}

func createNode(cfg *config.Config, logger log.Logger, nodeKey *p2p.NodeKey, app *app.Application) (*node.Node, error) {
    return node.NewNode(
        cfg,
        privval.LoadOrGenFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile()),
        nodeKey,
        proxy.NewLocalClientCreator(app),
        node.DefaultGenesisDocProviderFunc(cfg),
        node.DefaultDBProvider,
        node.DefaultMetricsProvider(cfg.Instrumentation),
        logger,
    )
}

func startLightClient(cfg *Config) error {
    nodeCfg, nodeLogger, _, err := setupNodeConfig()
    if err != nil {
        return fmt.Errorf("failed to setup light client: %w", err)
    }

    client, err := setupLightClient(cfg, nodeCfg, nodeLogger)
    if err != nil {
        return fmt.Errorf("failed to setup light client: %w", err)
    }

    proxy, err := createLightClientProxy(cfg, nodeCfg, client, nodeLogger)
    if err != nil {
        return fmt.Errorf("failed to create light client proxy: %w", err)
    }

    logger.Info("starting light client proxy", "addr", nodeCfg.RPC.ListenAddress)
    if err := proxy.ListenAndServe(); err != http.ErrServerClosed {
        return fmt.Errorf("proxy server failed: %w", err)
    }

    return nil
}

func setupPrivValidator(cfg *Config) error {
    filePV := privval.LoadFilePV(cfg.PrivValKey, cfg.PrivValState)
    protocol, address := bcnet.ProtocolAndAddress(cfg.PrivValServer)

    dialFn := getDialer(protocol, address)
    if dialFn == nil {
        return fmt.Errorf("unsupported privval protocol: %q", protocol)
    }

    endpoint := privval.NewSignerDialerEndpoint(
        logger,
        dialFn,
        privval.SignerDialerEndpointRetryWaitInterval(time.Second),
        privval.SignerDialerEndpointConnRetries(defaultRetries),
    )

    if err := privval.NewSignerServer(endpoint, cfg.ChainID, filePV).Start(); err != nil {
        return fmt.Errorf("failed to start signer server: %w", err)
    }

    logger.Info("private validator started", 
        "server", cfg.PrivValServer)

    return nil
}

// Additional helper functions omitted for brevity...
