# go-office — common development targets (Linux / WSL)
#
#   make setup    # once: install Go module dependencies
#   make build    # fetch Euro-Office assets + compile binaries
#   make demo     # build (if needed) and run the demo server
#   make test     # unit tests (no assets required)
#
# Targets stack: setup → build → demo

.DEFAULT_GOAL := help

GO ?= go
GO_OFFICE_ASSETS ?= $(CURDIR)/assets
ADDR ?= :8080
BIN_DIR ?= bin
UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)

GO_OFFICE_BIN := $(BIN_DIR)/go-office
FETCH_ASSETS_BIN := $(BIN_DIR)/fetch-assets

API_JS := $(GO_OFFICE_ASSETS)/web-apps/apps/api/documents/api.js
API_JS_TPL := $(GO_OFFICE_ASSETS)/web-apps/apps/api/documents/api.js.tpl
ALL_FONTS := $(GO_OFFICE_ASSETS)/sdkjs/common/AllFonts.js
SAMPLES_DIR ?= sample-files

.PHONY: help setup build demo test test-integration clean \
        check-linux mod-download fetch-assets compile check-assets check-samples

help:
	@echo "go-office"
	@echo ""
	@echo "  make setup   Install dependencies (Go modules)"
	@echo "  make build   Fetch Euro-Office assets and compile bin/go-office"
	@echo "  make demo    Build (if needed) and run the demo server on $(ADDR)"
	@echo "  make test    Run unit tests (no assets required)"
	@echo ""
	@echo "  make test-integration   Integration tests (runs build first)"
	@echo "  make clean              Remove bin/ and downloaded assets/"
	@echo ""
	@echo "Variables: ADDR=$(ADDR)  GO_OFFICE_ASSETS=$(GO_OFFICE_ASSETS)  SAMPLES_DIR=$(SAMPLES_DIR)"
	@echo ""
	@echo "Typical flow:"
	@echo "  make setup && make demo"
	@echo ""
	@echo "Then open http://localhost:8080/ and http://localhost:8080/office/demo/"

setup: check-linux mod-download
	@echo ""
	@echo "Setup complete. Next: make build  (or make demo to build and run)"

build: setup fetch-assets compile check-assets
	@echo ""
	@echo "Build complete: $(GO_OFFICE_BIN)"
	@echo "  make demo   — run the demo server"

demo: build check-samples
	@echo ""
	@echo "Demo server on http://localhost:8080/ (debug logging enabled, Ctrl+C to stop)"
	$(GO_OFFICE_BIN) -assets "$(GO_OFFICE_ASSETS)" -addr "$(ADDR)" -samples "$(SAMPLES_DIR)" -demo

check-linux:
	@if [ "$(UNAME_S)" != "Linux" ]; then \
		echo "error: go-office requires Linux (use WSL on Windows/macOS)"; \
		exit 1; \
	fi

mod-download:
	@echo "==> Go modules"
	$(GO) mod download

check-samples:
	@if [ ! -d "$(SAMPLES_DIR)" ]; then \
		echo "error: samples directory not found: $(SAMPLES_DIR)/"; \
		echo "       Add documents there or set SAMPLES_DIR=..."; \
		exit 1; \
	fi

fetch-assets: $(ALL_FONTS)

$(API_JS) $(API_JS_TPL) $(ALL_FONTS): check-linux
	@echo "==> Euro-Office assets → $(GO_OFFICE_ASSETS)/"
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(FETCH_ASSETS_BIN)" ./cmd/fetch-assets
	$(FETCH_ASSETS_BIN) -out "$(GO_OFFICE_ASSETS)"

compile: $(GO_OFFICE_BIN)

$(GO_OFFICE_BIN): check-linux
	@echo "==> go-office binary"
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office

check-assets:
	@if [ ! -f "$(API_JS)" ] && [ ! -f "$(API_JS_TPL)" ]; then \
		echo "error: Euro-Office assets missing in $(GO_OFFICE_ASSETS)/"; \
		echo "       Run: make build"; \
		exit 1; \
	fi
	@if [ ! -f "$(ALL_FONTS)" ]; then \
		echo "error: AllFonts.js missing — run: make build"; \
		exit 1; \
	fi

test:
	$(GO) test ./...

test-integration: build
	GO_OFFICE_ASSETS="$(GO_OFFICE_ASSETS)" $(GO) test -tags=integration ./...

clean:
	rm -rf "$(BIN_DIR)" "$(GO_OFFICE_ASSETS)"
	$(GO) clean -testcache
