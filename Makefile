APP ?= csgclaw
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP)
DIST_DIR ?= dist
GOCACHE ?= $(CURDIR)/.gocache
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG ?= csgclaw/internal/version
LDFLAGS ?= -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)
CLI_LDFLAGS ?= -s -w $(LDFLAGS)
CMD_PATH ?= ./cmd/$(APP)
BOXLITE_CLI_VERSION ?= v0.9.0
BOXLITE_CLI_BASE_URL ?= https://github.com/boxlite-ai/boxlite/releases/download

GO ?= go
GOFMT ?= gofmt
TARGET_OS ?= $(shell $(GO) env GOOS)
TARGET_ARCH ?= $(shell $(GO) env GOARCH)
CLI_BIN ?= $(BIN_DIR)/csgclaw-cli

IMAGE ?= opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/picoclaw
TAG ?= 2026.4.27.0
LOCAL_IMAGE ?= picoclaw:local

.DEFAULT_GOAL := build-all

.PHONY: help fmt test build build-csgclaw build-csgclaw-cli build-csgclaw-cli-for-picoclaw build-all run clean package package-all release tag push publish sync-agent-runtimes

help:
	@printf '%s\n' \
		'make fmt       - format Go files' \
		'make sync-agent-runtimes - stage PicoClaw runtime workspaces for Go embed' \
		'make test      - run Go tests' \
		'make build     - build $(BIN)' \
		'make build-csgclaw-cli - build $(CLI_BIN) for TARGET_OS/TARGET_ARCH (defaults to current platform)' \
		'make build-csgclaw-cli-for-picoclaw - build PicoClaw CLI binaries for linux/amd64 and linux/arm64' \
		'make build-all - build bin/csgclaw and bin/csgclaw-cli' \
		'make run       - run the server in foreground' \
		'make package   - package APP binary into dist/' \
		'make package-all - package csgclaw and csgclaw-cli for current platform' \
		'make release   - build csgclaw and csgclaw-cli release archives for macOS/Linux' \
		'make clean     - remove local build outputs' \
		'make tag       - tag local manager image' \
		'make push      - push manager image' \
		'make publish   - tag and push manager image'

fmt:
	$(GOFMT) -w $(shell find cli cmd internal -name '*.go')

sync-agent-runtimes:
	$(CURDIR)/scripts/sync-agent-runtimes.sh

test: sync-agent-runtimes
	env GOCACHE=$(GOCACHE) $(GO) test ./...

build: sync-agent-runtimes
	mkdir -p $(BIN_DIR)
	env GOCACHE=$(GOCACHE) $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD_PATH)

build-csgclaw:
	$(MAKE) build APP=csgclaw

build-csgclaw-cli: sync-agent-runtimes
	mkdir -p $(BIN_DIR)
	env GOCACHE=$(GOCACHE) GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) $(GO) build -ldflags "$(CLI_LDFLAGS)" -o $(CLI_BIN) ./cmd/csgclaw-cli

build-csgclaw-cli-for-picoclaw:
	$(MAKE) build-csgclaw-cli TARGET_OS=linux TARGET_ARCH=amd64 CLI_BIN=$(BIN_DIR)/csgclaw-cli_linux_amd64
	$(MAKE) build-csgclaw-cli TARGET_OS=linux TARGET_ARCH=arm64 CLI_BIN=$(BIN_DIR)/csgclaw-cli_linux_arm64

build-all: build-csgclaw build-csgclaw-cli

run: build-csgclaw
	$(BIN) serve

package: sync-agent-runtimes
	mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=$(APP) GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=$(INCLUDE_BOXLITE) BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh $$(go env GOOS) $$(go env GOARCH)

package-all: sync-agent-runtimes
	mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=$(INCLUDE_BOXLITE) BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh $$(go env GOOS) $$(go env GOARCH)
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw-cli GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=0 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh $$(go env GOOS) $$(go env GOARCH)

release: sync-agent-runtimes
	mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=1 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh darwin arm64
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw-cli GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=0 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh darwin arm64
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=1 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh linux amd64
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw-cli GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=0 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh linux amd64
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=1 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh linux arm64
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) DIST_DIR=$(DIST_DIR) APP=csgclaw-cli GOCACHE=$(GOCACHE) INCLUDE_BOXLITE=0 BOXLITE_CLI_VERSION=$(BOXLITE_CLI_VERSION) BOXLITE_CLI_BASE_URL=$(BOXLITE_CLI_BASE_URL) $(CURDIR)/scripts/package-release.sh linux arm64

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) $(GOCACHE)

tag:
	docker tag $(LOCAL_IMAGE) $(IMAGE):$(TAG)

push:
	docker push $(IMAGE):$(TAG)

publish: tag push
