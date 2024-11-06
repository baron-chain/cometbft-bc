//go:build tools
// Package tools tracks build-time development tool dependencies.
// This ensures consistent versions of these tools across development environments.
//
// For more information about this pattern, see:
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

// Development tool imports
// Each tool serves a specific purpose in the development workflow:
import (
	// buf - Protocol buffer tooling
	_ "github.com/bufbuild/buf/cmd/buf"

	// golangci-lint - Go linting aggregator
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"

	// peg - Parser generator for parsing expression grammars
	_ "github.com/pointlander/peg"

	// mockery - Mock code generator
	_ "github.com/vektra/mockery/v2"
)

// Tool versions are managed in go.mod.
// To update a tool version:
// 1. Run: go get -u <tool-path>@<version>
// 2. Run: go mod tidy
//
// Current tools:
// - buf: Protocol buffer toolchain for schema management and generation
// - golangci-lint: Fast, parallel runner for dozens of Go linters
// - peg: Parser generator for creating parsers from PEG grammars
// - mockery: Generates type-safe mocks for Go interfaces
