package main

import (
	"fmt"
	"time"

	"github.com/baron-chain/cometbft-bc/libs/log"
	rpctypes "github.com/baron-chain/cometbft-bc/rpc/core/types"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

const (
	perturbationRecoveryTime = 10 * time.Second
	perturbationWaitTime     = 3 * time.Second
	nodeRecoveryTimeout      = 20 * time.Second
)

type NodePerturber struct {
	node     *e2e.Node
	testnet  *e2e.Testnet
	upgraded bool
	name     string
}

func NewNodePerturber(node *e2e.Node) (*NodePerturber, error) {
	out, err := execComposeOutput(node.Testnet.Dir, "ps", "-q", node.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check node status: %w", err)
	}

	name := node.Name
	upgraded := false
	if len(out) == 0 {
		name = name + "_u"
		upgraded = true
		logger.Info("perturb node", "msg",
			log.NewLazySprintf("Node %v already upgraded, operating on alternate container %v",
				node.Name, name))
	}

	return &NodePerturber{
		node:     node,
		testnet:  node.Testnet,
		upgraded: upgraded,
		name:     name,
	}, nil
}

func Perturb(testnet *e2e.Testnet) error {
	for _, node := range testnet.Nodes {
		if err := perturbNode(node); err != nil {
			return fmt.Errorf("failed to perturb node %s: %w", node.Name, err)
		}
	}
	return nil
}

func perturbNode(node *e2e.Node) error {
	for _, perturbation := range node.Perturbations {
		if _, err := PerturbNode(node, perturbation); err != nil {
			return err
		}
		time.Sleep(perturbationWaitTime)
	}
	return nil
}

func PerturbNode(node *e2e.Node, perturbation e2e.Perturbation) (*rpctypes.ResultStatus, error) {
	perturber, err := NewNodePerturber(node)
	if err != nil {
		return nil, err
	}

	if err := perturber.applyPerturbation(perturbation); err != nil {
		return nil, err
	}

	status, err := waitForNode(node, 0, nodeRecoveryTimeout)
	if err != nil {
		return nil, fmt.Errorf("node failed to recover: %w", err)
	}

	logger.Info("perturb node",
		"msg",
		log.NewLazySprintf("Node %v recovered at height %v", node.Name, status.SyncInfo.LatestBlockHeight))

	return status, nil
}

func (p *NodePerturber) applyPerturbation(perturbation e2e.Perturbation) error {
	switch perturbation {
	case e2e.PerturbationDisconnect:
		return p.handleDisconnect()
	case e2e.PerturbationKill:
		return p.handleKill()
	case e2e.PerturbationPause:
		return p.handlePause()
	case e2e.PerturbationRestart:
		return p.handleRestart()
	case e2e.PerturbationUpgrade:
		return p.handleUpgrade()
	default:
		return fmt.Errorf("unexpected perturbation %q", perturbation)
	}
}

func (p *NodePerturber) handleDisconnect() error {
	logger.Info("perturb node", "msg", log.NewLazySprintf("Disconnecting node %v...", p.node.Name))
	
	networkName := p.testnet.Name + "_" + p.testnet.Name
	if err := execDocker("network", "disconnect", networkName, p.name); err != nil {
		return fmt.Errorf("failed to disconnect network: %w", err)
	}
	
	time.Sleep(perturbationRecoveryTime)
	
	if err := execDocker("network", "connect", networkName, p.name); err != nil {
		return fmt.Errorf("failed to reconnect network: %w", err)
	}
	
	return nil
}

func (p *NodePerturber) handleKill() error {
	logger.Info("perturb node", "msg", log.NewLazySprintf("Killing node %v...", p.node.Name))
	
	if err := execCompose(p.testnet.Dir, "kill", "-s", "SIGKILL", p.name); err != nil {
		return fmt.Errorf("failed to kill node: %w", err)
	}
	
	if err := execCompose(p.testnet.Dir, "start", p.name); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}
	
	return nil
}

func (p *NodePerturber) handlePause() error {
	logger.Info("perturb node", "msg", log.NewLazySprintf("Pausing node %v...", p.node.Name))
	
	if err := execCompose(p.testnet.Dir, "pause", p.name); err != nil {
		return fmt.Errorf("failed to pause node: %w", err)
	}
	
	time.Sleep(perturbationRecoveryTime)
	
	if err := execCompose(p.testnet.Dir, "unpause", p.name); err != nil {
		return fmt.Errorf("failed to unpause node: %w", err)
	}
	
	return nil
}

func (p *NodePerturber) handleRestart() error {
	logger.Info("perturb node", "msg", log.NewLazySprintf("Restarting node %v...", p.node.Name))
	
	if err := execCompose(p.testnet.Dir, "restart", p.name); err != nil {
		return fmt.Errorf("failed to restart node: %w", err)
	}
	
	return nil
}

func (p *NodePerturber) handleUpgrade() error {
	oldVersion := p.node.Version
	newVersion := p.testnet.UpgradeVersion

	if p.upgraded {
		return fmt.Errorf("node %v can't be upgraded twice from version '%v' to version '%v'",
			p.node.Name, oldVersion, newVersion)
	}

	if oldVersion == newVersion {
		logger.Info("perturb node", "msg",
			log.NewLazySprintf("Skipping upgrade of node %v to version '%v'; versions are equal.",
				p.node.Name, newVersion))
		return nil
	}

	logger.Info("perturb node", "msg",
		log.NewLazySprintf("Upgrading node %v from version '%v' to version '%v'...",
			p.node.Name, oldVersion, newVersion))

	if err := execCompose(p.testnet.Dir, "stop", p.name); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	time.Sleep(perturbationRecoveryTime)

	if err := execCompose(p.testnet.Dir, "up", "-d", p.name+"_u"); err != nil {
		return fmt.Errorf("failed to start upgraded node: %w", err)
	}

	return nil
}
