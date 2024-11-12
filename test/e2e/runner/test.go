package main

import (
	"os"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

func Test(testnet *e2e.Testnet) error {
	logger.Info("Running tests in ./tests/...")
	
	if err := os.Setenv("E2E_MANIFEST", testnet.File); err != nil {
		return err
	}
	
	testArgs := []string{"test", "-count", "1", "./tests/..."}
	return execVerbose("go", testArgs...)
