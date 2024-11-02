SHELL := /bin/bash

# Directory configuration
CURDIR := $(shell pwd)
SCRIPTS_DIR := $(CURDIR)

# Docker image configuration
DOCKER_REPO := cometbft/cometbft
DOCKER_TESTING_IMAGE := cometbft/testing
DOCKER_AMAZONLINUX_IMAGE := $(DOCKER_REPO):build_c-amazonlinux

# Dockerfile paths
DOCKERFILE_TESTING := Dockerfile.testing
DOCKERFILE_AMAZONLINUX := Dockerfile.build_c-amazonlinux

# Docker build options
DOCKER_BUILD_OPTS := --no-cache=false --pull

# Common docker build command
define docker_build
	docker build $(DOCKER_BUILD_OPTS) \
		--tag $(1) \
		--file $(2) \
		.
endef

.PHONY: all build push build_testing build_amazonlinux clean help

# Default target
all: build build_testing build_amazonlinux

# Main build targets
build:
	@echo "==> Building main Docker image"
	@$(SCRIPTS_DIR)/build.sh

push:
	@echo "==> Pushing Docker images"
	@$(SCRIPTS_DIR)/push.sh

build_testing:
	@echo "==> Building testing image"
	$(call docker_build,$(DOCKER_TESTING_IMAGE),$(DOCKERFILE_TESTING))

build_amazonlinux:
	@echo "==> Building Amazon Linux image"
	$(call docker_build,$(DOCKER_AMAZONLINUX_IMAGE),$(DOCKERFILE_AMAZONLINUX))

# Cleanup target
clean:
	@echo "==> Cleaning Docker images"
	docker rmi -f $(DOCKER_TESTING_IMAGE) $(DOCKER_AMAZONLINUX_IMAGE) 2>/dev/null || true

# Help target
help:
	@echo "Available targets:"
	@echo "  all                    - Build all Docker images"
	@echo "  build                  - Build main Docker image"
	@echo "  push                   - Push Docker images to registry"
	@echo "  build_testing          - Build testing Docker image"
	@echo "  build_amazonlinux      - Build Amazon Linux Docker image"
	@echo "  clean                  - Remove built Docker images"
	@echo "  help                   - Show this help message"
