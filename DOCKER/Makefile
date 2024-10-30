SHELL := /bin/bash
CURDIR := $(shell pwd)

DOCKER_REPO := cometbft/cometbft
DOCKER_TESTING_IMAGE := cometbft/testing
DOCKER_AMAZONLINUX_IMAGE := $(DOCKER_REPO):build_c-amazonlinux

.PHONY: build push build_testing build_amazonlinux_buildimage

build:
	$(CURDIR)/build.sh

push:
	$(CURDIR)/push.sh

build_testing:
	docker build \
		--tag $(DOCKER_TESTING_IMAGE) \
		--file Dockerfile.testing \
		.

build_amazonlinux_buildimage:
	docker build \
		--tag $(DOCKER_AMAZONLINUX_IMAGE) \
		--file Dockerfile.build_c-amazonlinux \
		.
