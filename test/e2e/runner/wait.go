package main

import (
	"time"
	
	"github.com/baron-chain/cometbft-bc/libs/log"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

func Wait(testnet *e2e.Testnet, blocks int64) error {
	block, _, err := waitForHeight(testnet, 0)
	if err != nil {
		return err
	}
	return WaitUntil(testnet, block.Height+blocks)
}

func WaitUntil(testnet *e2e.Testnet, height int64) error {
	logger.Info("wait until", "msg", log.NewLazySprintf("Waiting for all nodes to reach height %v...", height))
	_, err := waitForAllNodes(testnet, height, calculateWaitingTime(len(testnet.Nodes), height))
	return err
}

func calculateWaitingTime(nodes int, height int64) time.Duration {
	baseWaitTime := int64(20)
	waitTimePerNode := baseWaitTime + (int64(nodes) * height)
	return time.Duration(waitTimePerNode) * time.Second
}
