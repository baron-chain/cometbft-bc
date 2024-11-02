#!/usr/bin/make -f

BINDIR ?= $(GOPATH)/bin
PACKAGES ?= ./...
TEST_FLAGS ?= -p 1
DOCKER_TEST_IMAGE := tester

.PHONY: test test_race test_deadlock test_release test100 \
        test_cover test_apps test_abci_apps test_abci_cli \
        test_integrations vagrant_test

build_docker_test_image:
	docker build -t $(DOCKER_TEST_IMAGE) -f ./test/docker/Dockerfile .

test_cover:
	bash test/test_cover.sh

test_apps:
	bash test/app/test.sh

test_abci_apps:
	bash abci/tests/test_app/test.sh

test_abci_cli:
	bash abci/tests/test_cli/test.sh

test_integrations: build_docker_test_image tools install install_abci \
                  test_cover test_apps test_abci_apps test_abci_cli test_libs

test_release:
	go test -tags release $(PACKAGES)

test100:
	for i in {1..100}; do $(MAKE) test; done

vagrant_test:
	vagrant up
	vagrant ssh -c 'make test_integrations'

test:
	go test $(TEST_FLAGS) $(PACKAGES) -tags deadlock

test_race:
	go test $(TEST_FLAGS) -v -race $(PACKAGES)

test_deadlock:
	go test $(TEST_FLAGS) -v $(PACKAGES) -tags deadlock
