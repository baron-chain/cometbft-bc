# CometBFT Load Testing Makefile

# Project configuration
GOMOD := github.com/cometbft/cometbft/test/loadtime
OUTPUT ?= build/
BUILD_TAGS ?= 
BUILD_FLAGS ?=

# Tool versions
GO_VERSION := $(shell go version | cut -d' ' -f3)
REQUIRED_GO_VERSION := go1.21

# Tools and commands
GO := go
PROTOC := protoc
CLANG_FORMAT := clang-format
FIND := find
MKDIR := mkdir -p
GOLANGCI_LINT := golangci-lint

# Colors for output
YELLOW := \033[1;33m
RED := \033[0;31m
GREEN := \033[0;32m
NC := \033[0m # No Color

# Binary targets
LOAD_BIN := $(OUTPUT)load
REPORT_BIN := $(OUTPUT)report
BINS := $(LOAD_BIN) $(REPORT_BIN)

# Protobuf files
PROTO_FILES := $(shell find . -name '*.proto')
PROTO_GO_FILES := $(PROTO_FILES:.proto=.pb.go)

.DEFAULT_GOAL := build

## Help target
help:
	@echo "Available targets:"
	@echo "  build           - Build load and report binaries"
	@echo "  clean           - Remove build artifacts"
	@echo "  proto-gen       - Generate protobuf files"
	@echo "  proto-format    - Format protobuf files"
	@echo "  test            - Run tests"
	@echo "  lint            - Run linter"
	@echo "  check           - Run all checks"
	@echo "  install-deps    - Install development dependencies"
.PHONY: help

## Build targets
$(OUTPUT):
	@$(MKDIR) $(OUTPUT)

build: check-go-version $(OUTPUT) $(BINS)
	@echo "$(GREEN)Build completed successfully!$(NC)"
.PHONY: build

$(LOAD_BIN): $(GO_FILES)
	@echo "$(YELLOW)Building load binary...$(NC)"
	@$(GO) build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(LOAD_BIN) ./cmd/load/

$(REPORT_BIN): $(GO_FILES)
	@echo "$(YELLOW)Building report binary...$(NC)"
	@$(GO) build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(REPORT_BIN) ./cmd/report/

## Clean targets
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(OUTPUT)
	@echo "$(GREEN)Cleanup completed!$(NC)"
.PHONY: clean

## Protobuf targets
check-proto-deps:
	@echo "$(YELLOW)Checking protobuf dependencies...$(NC)"
	@which $(PROTOC) > /dev/null || (echo "$(RED)protoc is required. Install it first.$(NC)" && exit 1)
	@which protoc-gen-go > /dev/null || (echo "$(RED)protoc-gen-go is required. Install it first.$(NC)" && exit 1)
.PHONY: check-proto-deps

check-format-deps:
	@echo "$(YELLOW)Checking format dependencies...$(NC)"
	@which $(CLANG_FORMAT) > /dev/null || (echo "$(RED)clang-format is required. Install it first.$(NC)" && exit 1)
.PHONY: check-format-deps

proto-format: check-format-deps
	@echo "$(YELLOW)Formatting protobuf files...$(NC)"
	@$(FIND) . -name '*.proto' -exec $(CLANG_FORMAT) -i {} \;
	@echo "$(GREEN)Protobuf formatting completed!$(NC)"
.PHONY: proto-format

proto-gen: check-proto-deps
	@echo "$(YELLOW)Generating protobuf files...$(NC)"
	@for file in $(PROTO_FILES); do \
		$(PROTOC) --go_out=paths=source_relative:. $$file || exit 1; \
	done
	@echo "$(GREEN)Protobuf generation completed!$(NC)"
.PHONY: proto-gen

## Test targets
test:
	@echo "$(YELLOW)Running tests...$(NC)"
	@$(GO) test ./... -race -cover
	@echo "$(GREEN)Tests completed!$(NC)"
.PHONY: test

## Lint targets
lint:
	@echo "$(YELLOW)Running linter...$(NC)"
	@$(GOLANGCI_LINT) run
	@echo "$(GREEN)Lint completed!$(NC)"
.PHONY: lint

## Check targets
check-go-version:
	@echo "$(YELLOW)Checking Go version...$(NC)"
	@if [ ! "$(shell echo "$(GO_VERSION)" | cut -c1-8)" = "$(REQUIRED_GO_VERSION)" ]; then \
		echo "$(RED)Required Go version is $(REQUIRED_GO_VERSION), but you have $(GO_VERSION)$(NC)"; \
		exit 1; \
	fi
.PHONY: check-go-version

check: check-go-version lint test
	@echo "$(GREEN)All checks passed!$(NC)"
.PHONY: check

## Install dependencies
install-deps:
	@echo "$(YELLOW)Installing development dependencies...$(NC)"
	@$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@which $(CLANG_FORMAT) > /dev/null || \
		(echo "$(YELLOW)Please install clang-format manually for your platform$(NC)")
	@which $(GOLANGCI_LINT) > /dev/null || \
		(echo "$(YELLOW)Installing golangci-lint...$(NC)" && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin)
	@echo "$(GREEN)Dependencies installed!$(NC)"
.PHONY: install-deps

# Ensure targets are always run
.PHONY: $(BINS)
