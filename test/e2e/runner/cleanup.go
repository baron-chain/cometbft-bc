package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/baron-chain/cometbft-bc/libs/log"
	e2e "github.com/baron-chain/cometbft-bc/test/e2e/pkg"
)

const (
	e2eLabel          = "e2e=True"
	e2eNodeImage      = "cometbft/e2e-node"
	networkMountPath  = "/network"
)

var (
	ErrNoDirectory = errors.New("no directory set")
)

// Cleaner handles cleanup operations for the testnet
type Cleaner struct {
	testnet *e2e.Testnet
	dockerExecutor *DockerExecutor
}

// NewCleaner creates a new Cleaner instance
func NewCleaner(testnet *e2e.Testnet) *Cleaner {
	return &Cleaner{
		testnet: testnet,
		dockerExecutor: NewDockerExecutor("", false),
	}
}

// Cleanup orchestrates the cleanup of both Docker resources and the testnet directory
func Cleanup(testnet *e2e.Testnet) error {
	cleaner := NewCleaner(testnet)
	return cleaner.Cleanup()
}

func (c *Cleaner) Cleanup() error {
	if err := c.cleanupDocker(); err != nil {
		return fmt.Errorf("failed to cleanup Docker resources: %w", err)
	}

	if err := c.cleanupDirectory(); err != nil {
		return fmt.Errorf("failed to cleanup directory: %w", err)
	}

	return nil
}

// cleanupDocker removes all E2E Docker resources
func (c *Cleaner) cleanupDocker() error {
	logger.Info("Removing Docker containers and networks")

	xargsFlag := c.getXargsFlag()
	
	if err := c.removeDockerContainers(xargsFlag); err != nil {
		return fmt.Errorf("failed to remove containers: %w", err)
	}

	if err := c.removeDockerNetworks(xargsFlag); err != nil {
		return fmt.Errorf("failed to remove networks: %w", err)
	}

	return nil
}

func (c *Cleaner) getXargsFlag() string {
	if runtime.GOOS == "linux" {
		return "-r"
	}
	return ""
}

func (c *Cleaner) removeDockerContainers(xargsFlag string) error {
	cmd := fmt.Sprintf(
		"docker container ls -qa --filter label=%s | xargs %s docker container rm -f",
		e2eLabel, xargsFlag,
	)
	return exec("bash", "-c", cmd)
}

func (c *Cleaner) removeDockerNetworks(xargsFlag string) error {
	cmd := fmt.Sprintf(
		"docker network ls -q --filter label=%s | xargs %s docker network rm",
		e2eLabel, xargsFlag,
	)
	return exec("bash", "-c", cmd)
}

// cleanupDirectory handles the cleanup of the testnet directory
func (c *Cleaner) cleanupDirectory() error {
	if c.testnet.Dir == "" {
		return ErrNoDirectory
	}

	if err := c.validateDirectory(); err != nil {
		return err
	}

	logger.Info("cleanup dir", "msg", log.NewLazySprintf("Removing testnet directory %q", c.testnet.Dir))

	if err := c.cleanupDirectoryContents(); err != nil {
		return fmt.Errorf("failed to cleanup directory contents: %w", err)
	}

	if err := os.RemoveAll(c.testnet.Dir); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	return nil
}

func (c *Cleaner) validateDirectory() error {
	_, err := os.Stat(c.testnet.Dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}
	return nil
}

func (c *Cleaner) cleanupDirectoryContents() error {
	absDir, err := filepath.Abs(c.testnet.Dir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// On Linux, clean up files owned by root from within a container
	return c.dockerExecutor.DockerCmd(
		"run",
		"--rm",
		"--entrypoint",
		"",
		"-v",
		fmt.Sprintf("%v:%v", absDir, networkMountPath),
		e2eNodeImage,
		"sh",
		"-c",
		fmt.Sprintf("rm -rf %v/*/", networkMountPath),
	)
}
