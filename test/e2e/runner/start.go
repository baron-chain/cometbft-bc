package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/baron-chain/cometbft-bc/libs/log"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

const (
	initialNodeTimeout = 15 * time.Second
	catchupNodeTimeout = 3 * time.Minute
)

func Start(testnet *e2e.Testnet) error {
	if len(testnet.Nodes) == 0 {
		return fmt.Errorf("no nodes in testnet")
	}

	nodeQueue := sortNodes(testnet.Nodes)
	if nodeQueue[0].StartAt > 0 {
		return fmt.Errorf("no initial nodes in testnet")
	}

	if err := startInitialNodes(testnet, &nodeQueue); err != nil {
		return err
	}

	networkHeight := testnet.InitialHeight
	block, blockID, err := waitForHeight(testnet, networkHeight)
	if err != nil {
		return err
	}

	if err := updateStateSyncNodes(nodeQueue, block.Height, blockID.Hash.Bytes()); err != nil {
		return err
	}

	return startRemainingNodes(testnet, nodeQueue, networkHeight)
}

func sortNodes(nodes []*e2e.Node) []*e2e.Node {
	nodeQueue := append([]*e2e.Node{}, nodes...)
	
	sort.SliceStable(nodeQueue, func(i, j int) bool {
		a, b := nodeQueue[i], nodeQueue[j]
		switch {
		case a.Mode == b.Mode:
			return false
		case a.Mode == e2e.ModeSeed:
			return true
		case a.Mode == e2e.ModeValidator && b.Mode == e2e.ModeFull:
			return true
		}
		return false
	})

	sort.SliceStable(nodeQueue, func(i, j int) bool {
		return nodeQueue[i].StartAt < nodeQueue[j].StartAt
	})

	return nodeQueue
}

func startInitialNodes(testnet *e2e.Testnet, nodeQueue *[]*e2e.Node) error {
	logger.Info("Starting initial network nodes...")
	
	for len(*nodeQueue) > 0 && (*nodeQueue)[0].StartAt == 0 {
		node := (*nodeQueue)[0]
		*nodeQueue = (*nodeQueue)[1:]

		if err := execCompose(testnet.Dir, "up", "-d", node.Name); err != nil {
			return err
		}

		if _, err := waitForNode(node, 0, initialNodeTimeout); err != nil {
			return err
		}

		logNodeStatus(node)
	}
	return nil
}

func updateStateSyncNodes(nodes []*e2e.Node, height int64, hash []byte) error {
	for _, node := range nodes {
		if node.StateSync || node.Mode == e2e.ModeLight {
			if err := UpdateConfigStateSync(node, height, hash); err != nil {
				return err
			}
		}
	}
	return nil
}

func startRemainingNodes(testnet *e2e.Testnet, nodes []*e2e.Node, networkHeight int64) error {
	for _, node := range nodes {
		if node.StartAt > networkHeight {
			networkHeight = node.StartAt
			logger.Info("Waiting for network to advance before starting catch up node",
				"node", node.Name,
				"height", networkHeight)
				
			if _, _, err := waitForHeight(testnet, networkHeight); err != nil {
				return err
			}
		}

		if err := startCatchupNode(testnet, node); err != nil {
			return err
		}
	}
	return nil
}

func startCatchupNode(testnet *e2e.Testnet, node *e2e.Node) error {
	logger.Info("Starting catch up node", "node", node.Name, "height", node.StartAt)
	
	if err := execCompose(testnet.Dir, "up", "-d", node.Name); err != nil {
		return err
	}

	status, err := waitForNode(node, node.StartAt, catchupNodeTimeout)
	if err != nil {
		return err
	}

	logger.Info("start", "msg", log.NewLazySprintf("Node %v up on http://127.0.0.1:%v at height %v",
		node.Name, node.ProxyPort, status.SyncInfo.LatestBlockHeight))
	return nil
}

func logNodeStatus(node *e2e.Node) {
	if node.PrometheusProxyPort > 0 {
		logger.Info("start", "msg", log.NewLazySprintf(
			"Node %v up on http://127.0.0.1:%v; with Prometheus on http://127.0.0.1:%v/metrics",
			node.Name, node.ProxyPort, node.PrometheusProxyPort))
	} else {
		logger.Info("start", "msg", log.NewLazySprintf(
			"Node %v up on http://127.0.0.1:%v",
			node.Name, node.ProxyPort))
	}
}
