package main

import (
    "bufio"
    "encoding/hex"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/spf13/cobra"
    "github.com/baron-chain/cometbft-bc/libs/log"
    bcos "github.com/baron-chain/cometbft-bc/libs/os"
    abcicli "github.com/baron-chain/cometbft-bc/abci/client"
    "github.com/baron-chain/cometbft-bc/abci/server"
    servertest "github.com/baron-chain/cometbft-bc/abci/tests/server"
    "github.com/baron-chain/cometbft-bc/abci/types"
    "github.com/baron-chain/cometbft-bc/abci/version"
    "github.com/baron-chain/cometbft-bc/proto/baronchain/crypto"
)

var (
    client abcicli.Client
    logger log.Logger
    
    flagAddress  = "tcp://0.0.0.0:26658"
    flagAbci     = "socket"
    flagVerbose  bool
    flagLogLevel = "debug"
    flagPath     = "/store"
    flagHeight   int
    flagProve    bool
    flagPersist  string
)

type response struct {
    Data   []byte
    Code   uint32
    Info   string
    Log    string
    Status int32
    Query  *queryResponse
}

type queryResponse struct {
    Key      []byte
    Value    []byte
    Height   int64
    ProofOps *crypto.ProofOps
}

func Execute() error {
    addGlobalFlags()
    addCommands()
    return RootCmd.Execute()
}

var RootCmd = &cobra.Command{
    Use:   "baron-cli", 
    Short: "Baron Chain ABCI CLI tool",
    Long:  "Baron Chain ABCI CLI tool for interacting with the Baron Chain network",
    PersistentPreRunE: setupClient,
}

func setupClient(cmd *cobra.Command, args []string) error {
    switch cmd.Use {
    case "kvstore", "version":
        return nil
    }

    if logger == nil {
        allowLevel, err := log.AllowLevel(flagLogLevel)
        if err != nil {
            return err
        }
        logger = log.NewFilter(log.NewTMLogger(log.NewSyncWriter(os.Stdout)), allowLevel)
    }

    if client == nil {
        var err error
        client, err = abcicli.NewClient(flagAddress, flagAbci, false)
        if err != nil {
            return err
        }
        client.SetLogger(logger.With("module", "baron-client"))
        if err := client.Start(); err != nil {
            return err
        }
    }
    return nil
}

// Core command implementations
func cmdEcho(cmd *cobra.Command, args []string) error {
    msg := ""
    if len(args) > 0 {
        msg = args[0]
    }
    res, err := client.EchoSync(msg)
    if err != nil {
        return err
    }
    printResponse(cmd, args, response{Data: []byte(res.Message)})
    return nil
}

func cmdDeliverTx(cmd *cobra.Command, args []string) error {
    if len(args) == 0 {
        return errors.New("tx required")
    }
    
    txBytes, err := stringOrHexToBytes(args[0])
    if err != nil {
        return err
    }
    
    res, err := client.DeliverTxSync(types.RequestDeliverTx{Tx: txBytes})
    if err != nil {
        return err
    }
    
    printResponse(cmd, args, response{
        Code: res.Code,
        Data: res.Data,
        Info: res.Info,
        Log:  res.Log,
    })
    return nil
}

// Optimized utility functions
func stringOrHexToBytes(s string) ([]byte, error) {
    if strings.HasPrefix(strings.ToLower(s), "0x") {
        return hex.DecodeString(s[2:])
    }
    
    if !strings.HasPrefix(s, "\"") || !strings.HasSuffix(s, "\"") {
        return nil, fmt.Errorf("invalid string arg: \"%s\". Must be quoted or a \"0x\"-prefixed hex string", s)
    }
    
    return []byte(s[1 : len(s)-1]), nil
}
