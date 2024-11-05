PACKAGES := $(shell go list ./...)
BUILDDIR ?= $(CURDIR)/build
OUTPUT ?= $(BUILDDIR)/cometbft
BUILD_TAGS ?= cometbft
COMMIT_HASH := $(shell git rev-parse --short HEAD)
LD_FLAGS := -X github.com/cometbft/cometbft/version.TMGitCommitHash=$(COMMIT_HASH)
BUILD_FLAGS := -mod=readonly -ldflags "$(LD_FLAGS)"
HTTPS_GIT := https://github.com/cometbft/cometbft.git
CGO_ENABLED ?= 0

# Build options
ifeq (,$(findstring nostrip,$(COMETBFT_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
  LD_FLAGS += -s -w
endif

ifeq (race,$(findstring race,$(COMETBFT_BUILD_OPTIONS)))
  CGO_ENABLED = 1
  BUILD_FLAGS += -race
endif

# Database options
ifeq (cleveldb,$(findstring cleveldb,$(COMETBFT_BUILD_OPTIONS)))
  CGO_ENABLED = 1
  BUILD_FLAGS += cleveldb
endif

ifeq (badgerdb,$(findstring badgerdb,$(COMETBFT_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif

ifeq (rocksdb,$(findstring rocksdb,$(COMETBFT_BUILD_OPTIONS)))
  CGO_ENABLED = 1
  BUILD_TAGS += rocksdb
endif

ifeq (boltdb,$(findstring boltdb,$(COMETBFT_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

LD_FLAGS += $(LDFLAGS)

# Platform settings
TARGETPLATFORM ?=
GOOS ?= linux
GOARCH ?= amd64
GOARM ?=

# Platform detection
define set_platform
  GOOS = linux
  GOARCH = $(2)
  $(if $(3),GOARM = $(3),)
endef

ifneq (,$(findstring linux/arm,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,arm,7))
endif

ifneq (,$(findstring linux/arm/v6,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,arm,6))
endif

ifneq (,$(findstring linux/arm64,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,arm64,7))
endif

ifneq (,$(findstring linux/386,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,386))
endif

ifneq (,$(findstring linux/amd64,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,amd64))
endif

ifneq (,$(findstring linux/mips,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,mips))
endif

ifneq (,$(findstring linux/mipsle,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,mipsle))
endif

ifneq (,$(findstring linux/mips64,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,mips64))
endif

ifneq (,$(findstring linux/mips64le,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,mips64le))
endif

ifneq (,$(findstring linux/riscv64,$(TARGETPLATFORM)))
  $(eval $(call set_platform,linux,riscv64))
endif

# Main targets
build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(OUTPUT) ./cmd/cometbft/

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) ./cmd/cometbft

metrics: testdata-metrics
	go generate -run="scripts/metricsgen" ./...

testdata-metrics:
	ls ./scripts/metricsgen/testdata | xargs -I{} go generate -v -run="scripts/metricsgen" ./scripts/metricsgen/testdata/{}

mockery:
	go generate -run="./scripts/mockery_generate.sh" ./...

# Protobuf targets
proto-gen: check-proto-deps
	go run github.com/bufbuild/buf/cmd/buf generate
	mv ./proto/tendermint/abci/types.pb.go ./abci/types/
	cp ./proto/tendermint/rpc/grpc/types.pb.go ./rpc/grpc

# Testing and formatting
format:
	find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local github.com/cometbft/cometbft

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Docker targets
build-docker:
	docker build --label=cometbft --tag="cometbft/cometbft" -f DOCKER/Dockerfile .

build-linux:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(MAKE) build

.PHONY: build install metrics testdata-metrics mockery proto-gen format lint vulncheck build-docker build-linux
