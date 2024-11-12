package main

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
)

// CommandExecutor handles command execution operations
type CommandExecutor struct {
	workDir string
	verbose bool
}

// NewCommandExecutor creates a new CommandExecutor instance
func NewCommandExecutor(workDir string, verbose bool) *CommandExecutor {
	return &CommandExecutor{
		workDir: workDir,
		verbose: verbose,
	}
}

// ExecResult represents the result of a command execution
type ExecResult struct {
	Output []byte
	Error  error
}

// Execute runs a command and returns its output
func (e *CommandExecutor) Execute(args ...string) error {
	result := e.executeCommand(args, false)
	return result.Error
}

// ExecuteWithOutput runs a command and returns its output
func (e *CommandExecutor) ExecuteWithOutput(args ...string) ([]byte, error) {
	result := e.executeCommand(args, false)
	return result.Output, result.Error
}

// ExecuteVerbose runs a command with output displayed to stdout/stderr
func (e *CommandExecutor) ExecuteVerbose(args ...string) error {
	result := e.executeCommand(args, true)
	return result.Error
}

// executeCommand is the core command execution function
func (e *CommandExecutor) executeCommand(args []string, showOutput bool) ExecResult {
	if len(args) == 0 {
		return ExecResult{nil, fmt.Errorf("no command provided")}
	}

	cmd := osexec.Command(args[0], args[1:]...) //nolint:gosec
	
	if e.workDir != "" {
		cmd.Dir = e.workDir
	}

	if showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return ExecResult{nil, cmd.Run()}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*osexec.ExitError); ok {
			return ExecResult{nil, fmt.Errorf("failed to run %q:\n%v", args, string(output))}
		}
		return ExecResult{nil, err}
	}

	return ExecResult{output, nil}
}

// DockerExecutor handles Docker-specific command execution
type DockerExecutor struct {
	*CommandExecutor
	composeFile string
}

// NewDockerExecutor creates a new DockerExecutor instance
func NewDockerExecutor(dir string, verbose bool) *DockerExecutor {
	return &DockerExecutor{
		CommandExecutor: NewCommandExecutor(dir, verbose),
		composeFile:    filepath.Join(dir, "docker-compose.yml"),
	}
}

// ComposeCmd executes a Docker Compose command
func (d *DockerExecutor) ComposeCmd(args ...string) error {
	fullArgs := d.buildComposeArgs(args...)
	return d.Execute(fullArgs...)
}

// ComposeCmdWithOutput executes a Docker Compose command and returns its output
func (d *DockerExecutor) ComposeCmdWithOutput(args ...string) ([]byte, error) {
	fullArgs := d.buildComposeArgs(args...)
	return d.ExecuteWithOutput(fullArgs...)
}

// ComposeCmdVerbose executes a Docker Compose command with output displayed
func (d *DockerExecutor) ComposeCmdVerbose(args ...string) error {
	fullArgs := d.buildComposeArgs(args...)
	return d.ExecuteVerbose(fullArgs...)
}

// DockerCmd executes a Docker command
func (d *DockerExecutor) DockerCmd(args ...string) error {
	fullArgs := append([]string{"docker"}, args...)
	return d.Execute(fullArgs...)
}

// buildComposeArgs builds the full Docker Compose command arguments
func (d *DockerExecutor) buildComposeArgs(args ...string) []string {
	return append(
		[]string{"docker-compose", "-f", d.composeFile},
		args...,
	)
}

// Convenience functions for backward compatibility
func exec(args ...string) error {
	executor := NewCommandExecutor("", false)
	return executor.Execute(args...)
}

func execOutput(args ...string) ([]byte, error) {
	executor := NewCommandExecutor("", false)
	return executor.ExecuteWithOutput(args...)
}

func execVerbose(args ...string) error {
	executor := NewCommandExecutor("", true)
	return executor.ExecuteVerbose(args...)
}

func execCompose(dir string, args ...string) error {
	docker := NewDockerExecutor(dir, false)
	return docker.ComposeCmd(args...)
}

func execComposeOutput(dir string, args ...string) ([]byte, error) {
	docker := NewDockerExecutor(dir, false)
	return docker.ComposeCmdWithOutput(args...)
}

func execComposeVerbose(dir string, args ...string) error {
	docker := NewDockerExecutor(dir, true)
	return docker.ComposeCmdVerbose(args...)
}

func execDocker(args ...string) error {
	docker := NewDockerExecutor("", false)
	return docker.DockerCmd(args...)
}
