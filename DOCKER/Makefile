SHELL := /bin/bash
CURDIR := $(shell pwd)
SCRIPTS_DIR := $(CURDIR)

# Baron Chain Docker configuration
DOCKER_REPO := baronchain/node
DOCKER_TESTING_IMAGE := baronchain/testing
DOCKER_AMAZONLINUX_IMAGE := $(DOCKER_REPO):build-amazonlinux

# Build configuration
DOCKERFILES := $(CURDIR)/docker
DOCKERFILE_TESTING := $(DOCKERFILES)/Dockerfile.testing
DOCKERFILE_AMAZONLINUX := $(DOCKERFILES)/Dockerfile.build-amazonlinux

# Build arguments
BUILD_DATE := $(shell date -u +'%Y-%m-%d')
BUILD_VERSION := $(shell git describe --tags --always)
DOCKER_BUILD_OPTS := --no-cache=false --pull \
					--build-arg BUILD_DATE=$(BUILD_DATE) \
					--build-arg BUILD_VERSION=$(BUILD_VERSION)

# Docker build template
define docker_build
	docker build $(DOCKER_BUILD_OPTS) \
		--tag $(1) \
		--file $(2) \
		--label "org.opencontainers.image.created=$(BUILD_DATE)" \
		--label "org.opencontainers.image.version=$(BUILD_VERSION)" \
		.
endef

.PHONY: all build push build-testing build-amazonlinux clean help

all: build build-testing build-amazonlinux

build:
	@echo "Building Baron Chain node image"
	@$(SCRIPTS_DIR)/build.sh

push:
	@echo "Pushing Baron Chain images to registry"
	@$(SCRIPTS_DIR)/push.sh

build-testing:
	@echo "Building Baron Chain testing environment"
	$(call docker_build,$(DOCKER_TESTING_IMAGE),$(DOCKERFILE_TESTING))

build-amazonlinux:
	@echo "Building Baron Chain Amazon Linux environment"
	$(call docker_build,$(DOCKER_AMAZONLINUX_IMAGE),$(DOCKERFILE_AMAZONLINUX))

clean:
	@echo "Cleaning Baron Chain Docker images"
	-docker rmi -f $(DOCKER_TESTING_IMAGE) $(DOCKER_AMAZONLINUX_IMAGE) 2>/dev/null

help:
	@echo "Baron Chain Docker Build System"
	@echo
	@echo "Available targets:"
	@echo "  all              - Build all Baron Chain images"
	@echo "  build            - Build Baron Chain node image"
	@echo "  push             - Push images to registry"
	@echo "  build-testing    - Build testing environment"
	@echo "  build-amazonlinux- Build Amazon Linux environment"
	@echo "  clean            - Remove built images"
	@echo "  help             - Show this help"
