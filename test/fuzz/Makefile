#!/usr/bin/make -f

# Fuzzing targets configuration for CometBFT
# Each target specifies whether it needs corpus initialization

# Common commands
RM := rm -f
GO_FUZZ_BUILD := go-fuzz-build
GO_FUZZ := go-fuzz
GO_RUN := go run

# Directories that need corpus initialization
CORPUS_DIRS := p2p/addrbook p2p/pex p2p/secret_connection

# Default target
.DEFAULT_GOAL := help

# Mark all targets as phony since they don't produce files
.PHONY: all help clean $(FUZZING_TARGETS)

# Fuzzing targets list
FUZZING_TARGETS := fuzz-mempool fuzz-p2p-addrbook fuzz-p2p-pex fuzz-p2p-sc fuzz-rpc-server

# Target to run all fuzz tests
all: $(FUZZING_TARGETS)
	@echo "All fuzzing tests completed"

# Help target
help:
	@echo "Available targets:"
	@echo "  all               - Run all fuzzing tests"
	@echo "  fuzz-mempool     - Run mempool fuzzing"
	@echo "  fuzz-p2p-addrbook - Run p2p addrbook fuzzing"
	@echo "  fuzz-p2p-pex     - Run p2p pex fuzzing"
	@echo "  fuzz-p2p-sc      - Run p2p secret connection fuzzing"
	@echo "  fuzz-rpc-server  - Run RPC server fuzzing"
	@echo "  clean            - Remove all fuzzing artifacts"
	@echo "  help             - Show this help message"

# Clean target
clean:
	@echo "Cleaning fuzzing artifacts..."
	@for dir in mempool p2p/addrbook p2p/pex p2p/secret_connection rpc/jsonrpc/server; do \
		echo "Cleaning $$dir"; \
		$(RM) $$dir/*-fuzz.zip; \
	done

# Define function for running fuzz tests
define run_fuzz_test
	@echo "Running fuzzing tests for $(1)..."
	@cd $(1) && \
		$(RM) *-fuzz.zip && \
		$(if $(filter $(1),$(CORPUS_DIRS)), \
			$(GO_RUN) ./init-corpus/main.go && \
			echo "Corpus initialized for $(1)" || exit 1;) \
		$(GO_FUZZ_BUILD) || exit 1; \
		echo "Fuzzer built successfully for $(1)"; \
		$(GO_FUZZ) || exit 1;
	@echo "Completed fuzzing tests for $(1)"
endef

# Individual fuzzing targets
fuzz-mempool:
	$(call run_fuzz_test,mempool)

fuzz-p2p-addrbook:
	$(call run_fuzz_test,p2p/addrbook)

fuzz-p2p-pex:
	$(call run_fuzz_test,p2p/pex)

fuzz-p2p-sc:
	$(call run_fuzz_test,p2p/secret_connection)

fuzz-rpc-server:
	$(call run_fuzz_test,rpc/jsonrpc/server)

# Error handling for missing dependencies
CHECK_DEPS := $(shell command -v $(GO_FUZZ) 2> /dev/null)
ifndef CHECK_DEPS
$(error "go-fuzz is not installed. Please run: go install github.com/dvyukov/go-fuzz/go-fuzz@latest")
endif

CHECK_DEPS_BUILD := $(shell command -v $(GO_FUZZ_BUILD) 2> /dev/null)
ifndef CHECK_DEPS_BUILD
$(error "go-fuzz-build is not installed. Please run: go install github.com/dvyukov/go-fuzz/go-fuzz-build@latest")
endif
