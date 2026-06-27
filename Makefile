# ═══════════════════════════════════════════════════════════════════════
#  Orion Makefile
# ═══════════════════════════════════════════════════════════════════════

# Read metadata from metadata.yaml (single source of truth)
PROGRAM_NAME ?= $(shell grep '^project_name:' metadata.yaml | sed 's/^project_name: *//')
BIN          ?= $(PROGRAM_NAME)
CMD          ?= ./cmd/orion/
GO           ?= go
GOFLAGS      ?= -trimpath
LDFLAGS      ?= -s -w
BUILDDIR     ?= build
VERSION      ?= $(shell grep '^version:' metadata.yaml | head -1 | sed 's/^version: *//' || git describe --tags --always 2>/dev/null || echo "dev")
OWNER        ?= $(shell grep -E '^\s+owner:' metadata.yaml | sed 's/.*owner: *//')
PROG_PKG     ?= github.com/$(OWNER)/$(PROGRAM_NAME)
LDFLAGS_ALL  = $(LDFLAGS) \
	-X main.version=$(VERSION) \
	-X main.programName=$(PROGRAM_NAME) \
	-X $(PROG_PKG)/internal/passive.programName=$(PROGRAM_NAME)

MKDIR    ?= mkdir -p
override RM = rm -rf
Q        ?= @

# ─── Default goal ──────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# ─── Help ──────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	$(Q)echo ""
	$(Q)echo " Usage: make [target] [VAR=value ...]"
	$(Q)echo ""
	$(Q)echo " Targets:"
	$(Q)grep -Eh '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| sort -t':' -k1,1 \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "   \033[36m%-28s\033[0m %s\n", $$1, $$2}'
	$(Q)echo ""
	$(Q)echo " Variables:"
	$(Q)echo "   GO=go          Go compiler binary"
	$(Q)echo "   BUILDDIR=build Artifact output directory"
	$(Q)echo "   GOFLAGS=-trimpath  Extra build flags"
	$(Q)echo "   Q=@            Set to empty (\033[33mQ=\033[0m) for verbose output"
	$(Q)echo ""

# ─── All (full CI pipeline) ──────────────────────────────────────────.
.PHONY: all
all: lint-fast build install ## Lint, build for current platform, install

# ─── Build for current platform ──────────────────────────────────────.
.PHONY: build
build: ## Build binary for the current platform
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)$(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN) $(CMD)

PREFIX  ?= /usr/local
DESTDIR ?=

.PHONY: install
install: ## Build and install for the current platform (PREFIX=, DESTDIR=)
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)goos=$$(go env GOOS); goarch=$$(go env GOARCH); goarm=$$(go env GOARM); \
	termux=; [ -n "$${TERMUX_VERSION:-}" ] && termux=1; \
	[ "$$goos" = "android" ] || [ -n "$$termux" ] && echo "==> Detected Termux / Android"; \
	echo "==> Building for $$goos/$$goarch$${goarm:+/armv}$$goarm"; \
	GOOS=$$goos GOARCH=$$goarch GOARM=$$goarm \
		$(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN) $(CMD); \
	if [ "$$goos" = "windows" ]; then \
		echo "==> Built: $(BUILDDIR)/$(BIN).exe  (copy manually)"; \
	elif [ "$$goos" = "android" ] || [ -n "$$termux" ]; then \
		$(MKDIR) "$(DESTDIR)$(PREFIX)/bin"; \
		install -m 755 "$(BUILDDIR)/$(BIN)" "$(DESTDIR)$(PREFIX)/bin/$(BIN)"; \
		echo "==> Installed $(BIN) to $(DESTDIR)$(PREFIX)/bin"; \
	else \
		dest="$${GOBIN:-$$(go env GOPATH)/bin}"; \
		$(MKDIR) "$$dest"; \
		install -m 755 "$(BUILDDIR)/$(BIN)" "$$dest/$(BIN)"; \
		echo "==> Installed $(BIN) to $$dest"; \
	fi

.PHONY: run
run: build ## Build and run (pass ARGS for subcommand flags)
	$(Q)$(BUILDDIR)/$(BIN) $(ARGS)

# ─── Cross-compilation ───────────────────────────────────────────────.

## Linux (amd64, 386, arm64, armv6, armv7)
.PHONY: build-linux
build-linux: ## Build all Linux targets
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-amd64 $(CMD)
	$(Q)GOOS=linux GOARCH=386   $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-386   $(CMD)
	$(Q)GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-arm64 $(CMD)
	$(Q)GOOS=linux GOARCH=arm GOARM=6 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-armv6 $(CMD)
	$(Q)GOOS=linux GOARCH=arm GOARM=7 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-armv7 $(CMD)

## Linux distro aliases — share the same binary; packaging is separate.
.PHONY: build-linux-rpm build-linux-deb build-linux-arch
build-linux-rpm:  build-linux ## Alias for build-linux (RPM packaging)
build-linux-deb:  build-linux ## Alias for build-linux (DEB packaging)
build-linux-arch: build-linux ## Alias for build-linux (Arch Linux packaging)

.PHONY: build-linux-alpine
build-linux-alpine: ## Build for Alpine Linux (musl, CGO_ENABLED=0)
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-linux-amd64 $(CMD)

## Termux / Android (CGo-free targets)
.PHONY: build-termux
build-termux: ## Build for Termux / Android (CGo-free)
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)CGO_ENABLED=0 GOOS=android GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-termux-arm64 $(CMD)

## Termux / Android (amd64 — needs Android NDK)
.PHONY: build-termux-cgo
build-termux-cgo: ## Build for Termux / Android (requires Android NDK)
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)GOOS=android GOARCH=amd64 CGO_ENABLED=1 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-termux-amd64 $(CMD)

## macOS
.PHONY: build-darwin
build-darwin: ## Build all macOS targets
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-darwin-amd64 $(CMD)
	$(Q)GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-darwin-arm64 $(CMD)

## Windows
.PHONY: build-windows
build-windows: ## Build all Windows targets
	$(Q)$(MKDIR) $(BUILDDIR)
	$(Q)GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-windows-amd64.exe $(CMD)
	$(Q)GOOS=windows GOARCH=386   $(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS_ALL)' -o $(BUILDDIR)/$(BIN)-windows-386.exe   $(CMD)

## Build for ALL platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin build-termux ## Build for all supported platforms (CGo-free)

# ─── Release ──────────────────────────────────────────────────────────.
.PHONY: release
release: ## Create a GitHub release (BUMP=patch|minor|major|vX.Y.Z)
	$(Q)scripts/release.sh $(BUMP)

# ─── Testing ──────────────────────────────────────────────────────────.
.PHONY: test
test: ## Run all tests
	$(Q)$(GO) test ./... -count=1

.PHONY: test-race
test-race: ## Run all tests with the race detector
	$(Q)$(GO) test -race ./... -count=1

.PHONY: test-short
test-short: ## Run all tests in short mode (skips expensive ones)
	$(Q)$(GO) test ./... -count=1 -short

.PHONY: test-cover
test-cover: ## Run all tests with code coverage
	$(Q)$(GO) test ./... -count=1 -coverprofile=build/coverage.out
	$(Q)$(GO) tool cover -func=build/coverage.out | tail -1

# ─── Linting ──────────────────────────────────────────────────────────.
.PHONY: lint
lint: ## Run full lint suite (format, vet, race, static analysis)
	$(Q)scripts/lint.sh

.PHONY: lint-fast
lint-fast: ## Run lint in short mode (skips gosec)
	$(Q)scripts/lint.sh --short

# ─── Static analysis helpers ──────────────────────────────────────────.
.PHONY: fmt
fmt: ## Format all Go code and tidy module
	$(Q)$(GO) fmt ./...
	$(Q)$(GO) mod tidy

.PHONY: vet
vet: ## Run go vet
	$(Q)$(GO) vet ./...

# ─── Housekeeping ─────────────────────────────────────────────────────.
.PHONY: clean
clean: ## Remove all build artifacts
	$(Q)$(RM) $(BUILDDIR)


