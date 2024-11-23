package main

import (
	"os"
	"path/filepath"

	cmd "github.com/baron-chain/cometbft-bc/cmd/cometbft/commands"
	"github.com/baron-chain/cometbft-bc/cmd/cometbft/commands/debug"
	cfg "github.com/baron-chain/cometbft-bc/config"
	"github.com/baron-chain/cometbft-bc/libs/cli"
	nm "github.com/baron-chain/cometbft-bc/node"
	"github.com/baron-chain/cometbft-bc/crypto/kyber"
)

const (
	defaultHomeDir = ".baronchain"
	appName       = "BARON"
)

func initCommands(rootCmd *cmd.RootCmd, nodeFunc nm.NodeFunc) {
	rootCmd.AddCommand(
		cmd.GenValidatorCmd,
		cmd.InitFilesCmd,
		cmd.LightCmd,
		cmd.ReplayCmd,
		cmd.ResetAllCmd,
		cmd.ShowValidatorCmd,
		cmd.ShowNodeIDCmd,
		cmd.GenNodeKeyCmd,
		cmd.VersionCmd,
		cmd.RollbackStateCmd,
		debug.DebugCmd,
		cli.NewCompletionCmd(rootCmd, true),
		cmd.NewRunNodeCmd(nodeFunc),
	)
}

func setupCrypto() error {
	return kyber.InitQuantumSafe()
}

func main() {
	if err := setupCrypto(); err != nil {
		panic("Failed to initialize quantum-safe cryptography: " + err.Error())
	}

	rootCmd := cmd.RootCmd
	nodeFunc := nm.NewQuantumSafeNode

	initCommands(rootCmd, nodeFunc)

	homeDir := os.ExpandEnv(filepath.Join("$HOME", defaultHomeDir))
	cmd := cli.PrepareBaseCmd(rootCmd, appName, homeDir)
	
	if err := cmd.Execute(); err != nil {
		panic("Failed to execute command: " + err.Error())
	}
}
