# go-office — common development targets (Linux / WSL)
#
#   make setup    # once: install Go module dependencies
#   make build    # fetch Euro-Office assets + compile binaries
#   make serve    # build (if needed) and run the document server
#   make test     # unit tests (no assets required)
#
# Targets stack: setup → build → serve

.DEFAULT_GOAL := help

GO ?= go
OFFICE_ASSETS ?= $(CURDIR)/assets
ADDR ?= :8080
BIN_DIR ?= bin
UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)

GO_OFFICE_BIN := $(BIN_DIR)/go-office
FETCH_ASSETS_BIN := $(BIN_DIR)/fetch-assets

API_JS := $(OFFICE_ASSETS)/web-apps/apps/api/documents/api.js
API_JS_TPL := $(OFFICE_ASSETS)/web-apps/apps/api/documents/api.js.tpl
ALL_FONTS := $(OFFICE_ASSETS)/sdkjs/common/AllFonts.js
FONT_SELECTION := $(OFFICE_ASSETS)/converter/bin/font_selection.bin
X2T_BIN := $(OFFICE_ASSETS)/converter/bin/x2t
SAMPLES_DIR ?= sample-files
SAMPLE_DOC ?= $(SAMPLES_DIR)/sample.doc
DOCKER_BUILD_TIMEOUT ?= 10m
DOCKER_BUILD = timeout $(DOCKER_BUILD_TIMEOUT) docker build

.PHONY: help setup build serve doctor fonts test test-integration clean \
        check-linux check-go mod-download fetch-assets compile check-assets check-samples test-x2t \
        playwright-base playwright-npm test-playwright test-playwright-ui check-sample-matrix extract-sample-manifest \
        build-docker build-docker-image build-docker-builder run-docker stop-docker ensure-assets

help:
	@echo "go-office"
	@echo ""
	@echo "  make setup   Verify Go 1.25+, install dependencies (Go modules)"
	@echo "  make build   Fetch Euro-Office assets and compile bin/go-office"
	@echo "  make serve   Build (if needed) and run the document server on $(ADDR)"
	@echo "  make test    Run unit tests (no assets required)"
	@echo "  make test-x2t  Run x2t conversion on the sample .doc (needs assets)"
	@echo "  make doctor    Diagnose x2t permissions, libs, and sample conversion"
	@echo "  make fonts     Regenerate AllFonts.js and font_selection.bin"
	@echo ""
	@echo "  make test-integration   Integration tests (runs build first)"
	@echo "  make check-sample-matrix  Verify all Playwright sample files exist (git-tracked under sample-files/)"
	@echo "  make extract-sample-manifest  Regenerate Playwright content expectations from sample-files/"
	@echo "  make test-playwright    E2E Playwright tests in Docker (runs build first)"
	@echo "  make test-playwright-ui Local Playwright UI (server in Docker, tests on host)"
	@echo "  make build-docker       Build Docker image (office-server)"
	@echo "  make build-docker-image Build Docker image only (debian-slim runtime for x2t)"
	@echo "  make build-docker-builder  Build reusable Alpine Go builder image"
	@echo "  make run-docker         Run server from existing Docker image"
	@echo "  make clean              Remove bin/ and downloaded assets/"
	@echo ""
	@echo "Variables: ADDR=$(ADDR)  OFFICE_ASSETS=$(OFFICE_ASSETS)  SAMPLES_DIR=$(SAMPLES_DIR)"
	@echo "           DOCKER_IMAGE=$(DOCKER_IMAGE)  DOCKER_PORT=$(DOCKER_PORT)"
	@echo ""
	@echo "Typical flow:"
	@echo "  make setup && make serve"
	@echo ""
	@echo "Then open http://localhost:8080/ and http://localhost:8080/demo/"

setup: check-linux check-go mod-download
	@chmod +x scripts/fix-x2t-perms.sh 2>/dev/null || true
	@echo ""
	@echo "Setup complete. Next: make build  (or make serve to build and run)"

build: setup fetch-assets compile check-assets
	@scripts/fix-x2t-perms.sh 2>/dev/null || true
	@echo ""
	@echo "Build complete: $(GO_OFFICE_BIN)"
	@echo "  make serve  — run the document server"

serve: build check-samples
	@echo ""
	@echo "Document server on http://localhost:8080/ (debug logging enabled, Ctrl+C to stop)"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO_OFFICE_BIN) -assets "$(OFFICE_ASSETS)" -addr "$(ADDR)" -data "." -samples "$(SAMPLES_DIR)" -debug

check-linux:
	@if [ "$(UNAME_S)" != "Linux" ]; then \
		echo "error: go-office requires Linux (use WSL on Windows/macOS)"; \
		exit 1; \
	fi

check-go:
	@ver=$$($(GO) env GOVERSION 2>/dev/null | sed 's/^go//'); \
	if [ -z "$$ver" ]; then ver=$$($(GO) version | awk '{print $$3}' | sed 's/^go//'); fi; \
	maj=$$(echo $$ver | cut -d. -f1); \
	min=$$(echo $$ver | cut -d. -f2); \
	if [ "$$maj" -lt 1 ] || { [ "$$maj" -eq 1 ] && [ "$$min" -lt 25 ]; }; then \
		echo "error: Go 1.25+ required (found go$$ver)"; \
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

check-sample-matrix: check-samples
	@chmod +x scripts/check-sample-matrix.sh
	@scripts/check-sample-matrix.sh "$(SAMPLES_DIR)"

extract-sample-manifest:
	@echo "==> Playwright sample manifest"
	$(GO) run ./scripts/extract-sample-expectations.go

fetch-assets: check-linux
	@echo "==> Euro-Office assets → $(OFFICE_ASSETS)/"
	@mkdir -p "$(BIN_DIR)" "$(OFFICE_ASSETS)"
	$(GO) build -o "$(FETCH_ASSETS_BIN)" ./cmd/fetch-assets
	$(FETCH_ASSETS_BIN) -out "$(OFFICE_ASSETS)"
	@test -f "$(ALL_FONTS)" || (echo "error: fetch-assets did not create $(ALL_FONTS)" && exit 1)
	@test -s "$(FONT_SELECTION)" || (echo "error: fetch-assets did not create $(FONT_SELECTION)" && exit 1)

ensure-assets: fetch-assets check-assets

compile: $(GO_OFFICE_BIN)

$(GO_OFFICE_BIN): check-linux
	@echo "==> go-office binary"
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office

check-assets:
	@if [ ! -f "$(API_JS)" ] && [ ! -f "$(API_JS_TPL)" ]; then \
		echo "error: Euro-Office assets missing in $(OFFICE_ASSETS)/"; \
		echo "       Run: make build"; \
		exit 1; \
	fi
	@if [ ! -f "$(ALL_FONTS)" ] || [ ! -s "$(ALL_FONTS)" ]; then \
		echo "error: AllFonts.js missing or empty — run: make fonts"; \
		exit 1; \
	fi
	@if [ ! -s "$(FONT_SELECTION)" ]; then \
		echo "error: font_selection.bin missing — run: make fonts"; \
		exit 1; \
	fi
	@if [ ! -f "$(X2T_BIN)" ]; then \
		echo "error: x2t converter missing — run: make build (re-fetches converter binaries)"; \
		exit 1; \
	fi
	@chmod +x "$(OFFICE_ASSETS)/converter/bin/"* 2>/dev/null || true
	@chmod +x "$(X2T_BIN)" || (echo "error: chmod +x failed for $(X2T_BIN)" && exit 1)
	@test -x "$(X2T_BIN)" || (echo "error: x2t is not executable: $(X2T_BIN)" && ls -la "$(X2T_BIN)" && exit 1)
	@ls -la "$(X2T_BIN)"

test-x2t: build check-samples
	@echo "==> x2t conversion test"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) run ./cmd/test-x2t -assets "$(OFFICE_ASSETS)" -sample "$(SAMPLE_DOC)"

doctor: build check-samples
	@echo "==> go-office doctor"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) run ./cmd/doctor -assets "$(OFFICE_ASSETS)" -sample "$(SAMPLE_DOC)" -report "$(CURDIR)/doctor-report.txt"

fonts: check-linux
	@echo "==> Regenerating font files"
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(FETCH_ASSETS_BIN)" ./cmd/fetch-assets
	$(FETCH_ASSETS_BIN) -fonts -out "$(OFFICE_ASSETS)"

test:
	$(GO) test ./...

PLAYWRIGHT_BASE_IMAGE ?= go-office-playwright-base
PLAYWRIGHT_TEST_IMAGE ?= go-office-playwright-tests
PLAYWRIGHT_LOCAL_CONTAINER ?= go-office-playwright-local

DOCKER_IMAGE ?= ghcr.io/quantumx-apps/office-server:local
DOCKER_BUILDER_IMAGE ?= go-office:builder
DOCKER_CONTAINER ?= go-office-serve
DOCKER_PORT ?= 8080
DOCKER_PUBLIC ?= http://localhost:$(DOCKER_PORT)

build-docker: build-docker-image

build-docker-image: check-linux check-samples
	@mkdir -p assets
	@echo "==> Docker image $(DOCKER_IMAGE)"
	$(DOCKER_BUILD) -t "$(DOCKER_IMAGE)" -f _docker/Dockerfile \
		$(if $(OFFICE_DEBUG_LOGGING),--build-arg OFFICE_DEBUG_LOGGING=$(OFFICE_DEBUG_LOGGING),) .

build-docker-builder:
	@echo "==> Docker builder image $(DOCKER_BUILDER_IMAGE)"
	$(DOCKER_BUILD) -t "$(DOCKER_BUILDER_IMAGE)" -f _docker/Dockerfile.builder .

run-docker: stop-docker
	@echo ""
	@echo "Document server (Docker) on $(DOCKER_PUBLIC)/"
	@echo "  site:    $(DOCKER_PUBLIC)/"
	@echo "  samples: $(DOCKER_PUBLIC)/demo/"
	@echo "Press Ctrl+C to stop"
	docker run --rm -p "$(DOCKER_PORT):80" --name "$(DOCKER_CONTAINER)" "$(DOCKER_IMAGE)" \
		-public "$(DOCKER_PUBLIC)"

stop-docker:
	@docker rm -f "$(DOCKER_CONTAINER)" 2>/dev/null || true

test-integration: build
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) test -tags=integration ./...

test-save-integration: build
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) test -tags=integration ./integration/... -race -count=1

playwright-base:
	@echo "==> Playwright base image"
	$(DOCKER_BUILD) -t "$(PLAYWRIGHT_BASE_IMAGE)" -f _docker/Dockerfile.playwright-base .

playwright-npm:
	@echo "==> Playwright npm dependencies"
	cd frontend && npm install

test-playwright: ensure-assets check-sample-matrix
	@echo "==> Playwright E2E (Docker)"
	GOOS=linux $(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office
	$(DOCKER_BUILD) -t "$(PLAYWRIGHT_TEST_IMAGE)" -f _docker/Dockerfile.playwright-office .

test-playwright-ui: build check-sample-matrix
	@echo "==> Playwright UI (server in Docker, tests on host)"
	GOOS=linux $(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office
	docker rm -f "$(PLAYWRIGHT_LOCAL_CONTAINER)" 2>/dev/null || true
	$(DOCKER_BUILD) -t "$(PLAYWRIGHT_LOCAL_CONTAINER)" -f _docker/Dockerfile.playwright-local .
	docker run -d -p 8080:8080 --name "$(PLAYWRIGHT_LOCAL_CONTAINER)" "$(PLAYWRIGHT_LOCAL_CONTAINER)"
	cd frontend && npm install && npx playwright install --with-deps firefox
	@echo "Open Playwright UI — server at http://127.0.0.1:8080/"
	cd frontend && npx playwright test --ui

clean:
	rm -rf "$(BIN_DIR)" "$(OFFICE_ASSETS)"
	$(GO) clean -testcache
