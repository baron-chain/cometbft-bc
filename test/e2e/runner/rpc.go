package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	rpchttp "github.com/baron-chain/cometbft-bc/rpc/client/http"
	rpctypes "github.com/baron-chain/cometbft-bc/rpc/core/types"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
	"github.com/baron-chain/cometbft-bc/types"
)

const (
	defaultQueryTimeout  = 1 * time.Second
	defaultPollInterval = 300 * time.Millisecond
	stallTimeout        = 20 * time.Second
)

type heightWatcher struct {
	clients      map[string]*rpchttp.HTTP
	maxResult    *rpctypes.ResultBlock
	lastIncrease time.Time
}

func newHeightWatcher() *heightWatcher {
	return &heightWatcher{
		clients:      make(map[string]*rpchttp.HTTP),
		lastIncrease: time.Now(),
	}
}

func waitForHeight(testnet *e2e.Testnet, height int64) (*types.Block, *types.BlockID, error) {
	watcher := newHeightWatcher()
	
	for {
		if err := watcher.pollNodes(testnet, height); err != nil {
			return nil, nil, err
		}
		
		if watcher.hasReachedHeight(height) {
			return watcher.maxResult.Block, &watcher.maxResult.BlockID, nil
		}
		
		if watcher.isStalled() {
			return nil, nil, watcher.getStallError()
		}
		
		time.Sleep(defaultPollInterval)
	}
}

func (w *heightWatcher) pollNodes(testnet *e2e.Testnet, targetHeight int64) error {
	for _, node := range testnet.Nodes {
		if node.Mode == e2e.ModeSeed {
			continue
		}
		
		if err := w.pollNode(node, targetHeight); err != nil {
			continue
		}
	}
	
	if len(w.clients) == 0 {
		return errors.New("unable to connect to any network nodes")
	}
	return nil
}

func (w *heightWatcher) pollNode(node *e2e.Node, targetHeight int64) error {
	client, err := w.getOrCreateClient(node)
	if err != nil {
		return err
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	
	result, err := client.Block(ctx, nil)
	if err != nil {
		return err
	}
	
	w.updateMaxResult(result)
	return nil
}

func (w *heightWatcher) getOrCreateClient(node *e2e.Node) (*rpchttp.HTTP, error) {
	if client, ok := w.clients[node.Name]; ok {
		return client, nil
	}
	
	client, err := node.Client()
	if err != nil {
		return nil, err
	}
	w.clients[node.Name] = client
	return client, nil
}

func (w *heightWatcher) updateMaxResult(result *rpctypes.ResultBlock) {
	if result.Block != nil && (w.maxResult == nil || result.Block.Height >= w.maxResult.Block.Height) {
		w.maxResult = result
		w.lastIncrease = time.Now()
	}
}

func (w *heightWatcher) hasReachedHeight(height int64) bool {
	return w.maxResult != nil && w.maxResult.Block.Height >= height
}

func (w *heightWatcher) isStalled() bool {
	return time.Since(w.lastIncrease) >= stallTimeout
}

func (w *heightWatcher) getStallError() error {
	if w.maxResult == nil {
		return errors.New("chain stalled at unknown height")
	}
	return fmt.Errorf("chain stalled at height %v", w.maxResult.Block.Height)
}

func waitForNode(node *e2e.Node, height int64, timeout time.Duration) (*rpctypes.ResultStatus, error) {
	client, err := node.Client()
	if err != nil {
		return nil, err
	}
	
	status := &nodeStatus{
		curHeight:    0,
		lastChanged:  time.Now(),
		targetHeight: height,
	}
	
	for {
		if err := status.update(client); err != nil {
			continue
		}
		
		if status.hasReachedTarget() {
			return status.current, nil
		}
		
		if time.Since(status.lastChanged) > timeout {
			return nil, fmt.Errorf("timed out waiting for %v to reach height %v", node.Name, height)
		}
		
		time.Sleep(defaultPollInterval)
	}
}

type nodeStatus struct {
	curHeight    int64
	lastChanged  time.Time
	targetHeight int64
	current      *rpctypes.ResultStatus
}

func (s *nodeStatus) update(client *rpchttp.HTTP) error {
	status, err := client.Status(context.Background())
	if err != nil {
		return err
	}
	
	s.current = status
	if s.curHeight < status.SyncInfo.LatestBlockHeight {
		s.curHeight = status.SyncInfo.LatestBlockHeight
		s.lastChanged = time.Now()
	}
	return nil
}

func (s *nodeStatus) hasReachedTarget() bool {
	return s.current.SyncInfo.LatestBlockHeight >= s.targetHeight && 
		(s.targetHeight == 0 || !s.current.SyncInfo.CatchingUp)
}

func waitForAllNodes(testnet *e2e.Testnet, height int64, timeout time.Duration) (int64, error) {
	var lastHeight int64
	deadline := time.Now().Add(timeout)
	
	for _, node := range testnet.Nodes {
		if node.Mode == e2e.ModeSeed {
			continue
		}
		
		status, err := waitForNode(node, height, time.Until(deadline))
		if err != nil {
			return 0, err
		}
		
		if status.SyncInfo.LatestBlockHeight > lastHeight {
			lastHeight = status.SyncInfo.LatestBlockHeight
		}
	}
	
	return lastHeight, nil
}
