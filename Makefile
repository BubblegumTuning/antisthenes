# Antisthenes Makefile
# Build and package the agent binary for local use and CI release.
#
# Targets:
#   make build     - Compile ./antisthenes (native GOOS/GOARCH)
#   make test      - go test ./...
#   make vet       - go vet ./...
#   make release   - Static linux/amd64 binary + tarball under dist/
#   make clean     - Remove build artifacts

GO         ?= go
# Prefer git tag (v0.1.5 → 0.1.5); fall back to module default.
VERSION_RAW := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.5")
VERSION     := $(shell echo "$(VERSION_RAW)" | sed 's/^v//')
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w -X main.version=$(VERSION)

# Cross-compile defaults (CI release uses these)
TARGET_OS   ?= linux
TARGET_ARCH ?= amd64

# Pure Go (modernc/sqlite) — static binary, no C toolchain required
CGO_ENABLED ?= 0

BIN_NAME     := antisthenes
RELEASE_DIR  := dist/$(VERSION)-$(TARGET_OS)-$(TARGET_ARCH)
TARBALL_NAME := $(BIN_NAME)-$(VERSION)-$(TARGET_OS)-$(TARGET_ARCH).tar.gz

.PHONY: all build test vet fmt release tarball clean verify version

all: build

# ---------------------------------------------------------------------------
# Local / native build (repo root binary, matches README ./antisthenes)
# ---------------------------------------------------------------------------

build:
	@echo "[BUILD] $(BIN_NAME) (native) v$(VERSION)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_NAME) ./cmd/antisthenes

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -l -s -w .

verify: fmt vet test build
	@./$(BIN_NAME) version

version:
	@echo $(VERSION)

# ---------------------------------------------------------------------------
# Release artifact (static cross-compile + tarball)
# ---------------------------------------------------------------------------

$(RELEASE_DIR)/$(BIN_NAME):
	@echo "[BUILD] $(BIN_NAME) ($(TARGET_OS)/$(TARGET_ARCH)) v$(VERSION)"
	mkdir -p $(RELEASE_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) \
		$(GO) build -ldflags="$(LDFLAGS)" -o $@ ./cmd/antisthenes

tarball: $(RELEASE_DIR)/$(BIN_NAME)
	@echo "[TARBALL] $(RELEASE_DIR)/$(TARBALL_NAME)"
	cp README.md config.example.json SOUL.md $(RELEASE_DIR)/ 2>/dev/null || true
	tar -czf $(RELEASE_DIR)/$(TARBALL_NAME) \
		-C $(RELEASE_DIR) \
		$(BIN_NAME) README.md config.example.json SOUL.md
	@ls -lh $(RELEASE_DIR)/$(TARBALL_NAME)

release: tarball
	@echo "[RELEASE] ready: $(RELEASE_DIR)/$(TARBALL_NAME)"

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------

clean:
	rm -rf dist/ $(BIN_NAME)
