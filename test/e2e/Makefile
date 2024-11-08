# Build configuration
DOCKER_TAG := cometbft/e2e-node:local-version
BUILD_DIR := build
DB_TAGS := badgerdb,boltdb,cleveldb,rocksdb

# Binary targets
BINS := node generator runner

# Default target
.DEFAULT_GOAL := all

# Create build directory if it doesn't exist
$(BUILD_DIR):
	@mkdir -p $@

# Build all binaries
.PHONY: all
all: $(BINS)

# Docker image build
.PHONY: docker
docker:
	@echo "Building E2E Docker image..."
	@docker build \
		--tag $(DOCKER_TAG) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(DOCKER_TAG) \
		-f docker/Dockerfile \
		../..

# Binary build targets
.PHONY: $(BINS)
$(BINS): | $(BUILD_DIR)
	@echo "Building $@..."
	@go build -v \
		-o $(BUILD_DIR)/$@ \
		$(if $(filter node,$@),-tags $(DB_TAGS)) \
		./$@

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all        - Build all binaries (default)"
	@echo "  docker     - Build E2E Docker image"
	@echo "  node       - Build node binary with DB support"
	@echo "  generator  - Build generator binary"
	@echo "  runner     - Build runner binary"
	@echo "  clean      - Remove build artifacts"
	@echo "  help       - Show this help message"
