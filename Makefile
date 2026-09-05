# go-office — development targets (native Linux or Docker on macOS/Windows)
#
#   make setup    # once: install Go module dependencies
#   make build    # fetch Euro-Office assets + compile binaries
#   make serve    # build (if needed) and run the document server
#   make test     # unit tests (no assets required)
#
# On macOS/Windows, targets that need Linux/x2t run inside a Linux dev container
# (requires Docker). Euro-Office assets persist in Docker volume go-office-assets
# and are only downloaded when missing. On Linux, assets live in ./assets/.
#
# Targets stack: setup → build → serve

.DEFAULT_GOAL := help

GO ?= go
OFFICE_ASSETS ?= $(CURDIR)/assets
ADDR ?= :8080
BIN_DIR ?= bin
UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)
USE_DOCKER_DEV := $(if $(filter Linux,$(UNAME_S)),,1)
SERVE_PORT ?= $(if $(findstring :,$(ADDR)),$(lastword $(subst :, ,$(ADDR))),$(ADDR))

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
ifeq ($(shell command -v timeout 2>/dev/null),)
DOCKER_BUILD = docker build
else
DOCKER_BUILD = timeout $(DOCKER_BUILD_TIMEOUT) docker build
endif

DOCKER_DEV_IMAGE ?= go-office:dev
DOCKER_ASSETS_VOLUME ?= go-office-assets
DOCKER_DEV_MOUNTS = -v "$(CURDIR):/src" -v "$(DOCKER_ASSETS_VOLUME):/src/assets"
DOCKER_DEV_RUN = docker run --rm $(DOCKER_DEV_MOUNTS) -w /src

.PHONY: help setup build serve doctor fonts test test-integration clean \
        check-docker docker-dev-image check-go mod-download fetch-assets compile check-assets check-samples test-x2t test-x2t-concurrent \
        playwright-npm test-playwright test-playwright-ui check-sample-matrix extract-sample-manifest \
        build-docker build-docker-image build-docker-builder run-docker stop-docker ensure-assets \
        build-native serve-native fetch-assets-native compile-native fonts-native \
        test-integration-native test-convert-linux-native test-x2t-native test-x2t-concurrent-native doctor-native

help:
	@echo "go-office"
	@echo ""
	@echo "  make setup   Verify Go 1.27+, install dependencies (Go modules)"
	@echo "  make build   Fetch Euro-Office assets and compile bin/go-office"
	@echo "  make serve   Build (if needed) and run the document server on $(ADDR)"
	@echo "  make test    Run unit tests (no assets required)"
	@echo "  make test-x2t  Run x2t conversion on the sample .doc (needs assets)"
	@echo "  make test-x2t-concurrent  Concurrent CSV save regression (10 workers, limit 6)"
	@echo "  make doctor    Diagnose x2t permissions, libs, and sample conversion"
	@echo "  make fonts     Regenerate AllFonts.js and font_selection.bin (host paths; remapped at runtime)"
	@echo ""
	@echo "  make test-integration   Integration tests in ./integration/ (runs build first)"
	@echo "  make test-convert-linux x2t convert tests in ./internal/convert/ (runs build first)"
	@echo "  make check-sample-matrix  Verify all Playwright sample files exist (git-tracked under sample-files/)"
	@echo "  make extract-sample-manifest  Regenerate Playwright content expectations from sample-files/"
	@echo "  make test-playwright    E2E Playwright tests in Docker (runs build first)"
	@echo "  make test-playwright-ui Local Playwright UI (server in Docker, tests on host)"
	@echo "  make build-docker       Build Docker image (office-server)"
	@echo "  make build-docker-image Build Docker image only (debian-slim runtime for x2t)"
	@echo "  make build-docker-builder  Build Linux toolchain image (same as docker-dev-image)"
	@echo "  make run-docker         Run server from existing Docker image"
	@echo "  make docker-dev-image   Build Linux toolchain image (used automatically on macOS/Windows)"
	@echo "  make clean              Remove bin/ and downloaded assets/"
	@echo ""
	@echo "Variables: ADDR=$(ADDR)  OFFICE_ASSETS=$(OFFICE_ASSETS)  SAMPLES_DIR=$(SAMPLES_DIR)"
	@echo "           DOCKER_IMAGE=$(DOCKER_IMAGE)  DOCKER_PORT=$(DOCKER_PORT)"
ifeq ($(USE_DOCKER_DEV),1)
	@echo "           DOCKER_ASSETS_VOLUME=$(DOCKER_ASSETS_VOLUME)  (persistent Euro-Office assets)"
endif
	@echo ""
ifeq ($(USE_DOCKER_DEV),1)
	@echo "Platform: $(UNAME_S) — Linux/x2t targets run in Docker ($(DOCKER_DEV_IMAGE))"
	@echo "Euro-Office assets persist in Docker volume $(DOCKER_ASSETS_VOLUME) (fetched once)."
	@echo "Requires Docker Desktop (or another Docker engine) to be installed and running."
	@echo ""
endif
	@echo "Typical flow:"
	@echo "  make setup && make serve"
	@echo ""
	@echo "Then open http://localhost:$(SERVE_PORT)/ and http://localhost:$(SERVE_PORT)/demo/"

setup: check-go mod-download
	@chmod +x scripts/fix-x2t-perms.sh 2>/dev/null || true
	@echo ""
	@echo "Setup complete. Next: make build  (or make serve to build and run)"

build:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make build-native
else
	@$(MAKE) build-native
endif

build-native: setup fetch-assets-native compile-native check-assets
	@scripts/fix-x2t-perms.sh 2>/dev/null || true
	@echo ""
	@echo "Build complete: $(GO_OFFICE_BIN)"
	@echo "  make serve  — run the document server"

serve:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-serve
else
	@$(MAKE) serve-native
endif

docker-serve: docker-dev-image
	@echo ""
	@echo "Document server (Docker dev) on http://localhost:$(SERVE_PORT)/ (Ctrl+C to stop)"
	docker run --rm -it -p "$(SERVE_PORT):$(SERVE_PORT)" \
		$(DOCKER_DEV_MOUNTS) -w /src \
		$(DOCKER_DEV_IMAGE) make serve-native ADDR="$(ADDR)"

serve-native: ensure-assets compile-native check-samples
	@echo ""
	@echo "Document server on http://localhost:$(SERVE_PORT)/ (debug logging enabled, Ctrl+C to stop)"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO_OFFICE_BIN) -assets "$(OFFICE_ASSETS)" -addr "$(ADDR)" -data "." -samples "$(SAMPLES_DIR)" -debug

check-docker:
	@command -v docker >/dev/null 2>&1 || { echo "error: docker is required but not installed"; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "error: docker daemon is not running"; exit 1; }

docker-dev-image: check-docker
	@echo "==> Docker toolchain image $(DOCKER_DEV_IMAGE)"
	$(DOCKER_BUILD) -t "$(DOCKER_DEV_IMAGE)" -f _docker/Dockerfile.builder .

build-docker-builder: docker-dev-image
	@docker tag "$(DOCKER_DEV_IMAGE)" "$(DOCKER_BUILDER_IMAGE)"

check-go:
	@ver=$$($(GO) env GOVERSION 2>/dev/null | sed 's/^go//'); \
	if [ -z "$$ver" ]; then ver=$$($(GO) version | awk '{print $$3}' | sed 's/^go//'); fi; \
	maj=$$(echo $$ver | cut -d. -f1); \
	min=$$(echo $$ver | cut -d. -f2); \
	if [ "$$maj" -lt 1 ] || { [ "$$maj" -eq 1 ] && [ "$$min" -lt 27 ]; }; then \
		echo "error: Go 1.27+ required (found go$$ver)"; \
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

fetch-assets:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make fetch-assets-native
else
	@$(MAKE) fetch-assets-native
endif

fetch-assets-native:
	@mkdir -p "$(BIN_DIR)" "$(OFFICE_ASSETS)"
	@if [ -f "$(OFFICE_ASSETS)/.extracted" ] && [ -f "$(ALL_FONTS)" ] && [ -s "$(FONT_SELECTION)" ] && [ -f "$(X2T_BIN)" ]; then \
		echo "==> Euro-Office assets already present in $(OFFICE_ASSETS)/"; \
	else \
		echo "==> Euro-Office assets → $(OFFICE_ASSETS)/"; \
		$(GO) build -o "$(FETCH_ASSETS_BIN)" ./cmd/fetch-assets; \
		$(FETCH_ASSETS_BIN) -out "$(OFFICE_ASSETS)"; \
		test -f "$(ALL_FONTS)" || (echo "error: fetch-assets did not create $(ALL_FONTS)" && exit 1); \
		test -s "$(FONT_SELECTION)" || (echo "error: fetch-assets did not create $(FONT_SELECTION)" && exit 1); \
	fi

ensure-assets: fetch-assets check-assets

compile:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make compile-native
else
	@$(MAKE) compile-native
endif

compile-native:
	@echo "==> go-office binary"
	@mkdir -p "$(BIN_DIR)"
	$(GO) run ./cmd/gen-assets-version
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
	@arch=$$(uname -m); \
	case "$$arch" in \
	  aarch64|arm64) want="ARM aarch64" ;; \
	  x86_64|amd64) want="x86-64" ;; \
	  *) echo "error: unsupported host arch $$arch for x2t check"; exit 1 ;; \
	esac; \
	file "$(X2T_BIN)" | grep -qi "$$want" || ( \
	  echo "error: x2t architecture mismatch ($$(file "$(X2T_BIN)"))"; \
	  echo "       delete assets/ and re-run: make ensure-assets"; \
	  exit 1)
	@ls -la "$(X2T_BIN)"

test-x2t:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make test-x2t-native
else
	@$(MAKE) test-x2t-native
endif

test-x2t-native: build-native check-samples
	@echo "==> x2t conversion test"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) run ./cmd/test-x2t -assets "$(OFFICE_ASSETS)" -sample "$(SAMPLE_DOC)"

test-x2t-concurrent:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make test-x2t-concurrent-native
else
	@$(MAKE) test-x2t-concurrent-native
endif

test-x2t-concurrent-native: build-native check-samples
	@echo "==> Concurrent CSV save (Playwright load regression, needs x2t)"
	$(GO) test ./internal/convert/ -run 'TestSaveChangesCSVConcurrent|TestPrepareX2TRunDir' -count=3 -v

doctor:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make doctor-native
else
	@$(MAKE) doctor-native
endif

doctor-native: build-native check-samples
	@echo "==> go-office doctor"
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) run ./cmd/doctor -assets "$(OFFICE_ASSETS)" -sample "$(SAMPLE_DOC)" -report "$(CURDIR)/doctor-report.txt"

fonts:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make fonts-native
else
	@$(MAKE) fonts-native
endif

fonts-native:
	@echo "==> Regenerating font files (paths are host-specific until Converter.New remaps them)"
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(FETCH_ASSETS_BIN)" ./cmd/fetch-assets
	$(FETCH_ASSETS_BIN) -fonts -out "$(OFFICE_ASSETS)"

test:
	$(GO) test -race ./...

lint:
	$(GO) tool golangci-lint run ./...

PLAYWRIGHT_IMAGE ?= go-office-playwright
PLAYWRIGHT_DOCKERFILE := _docker/Dockerfile.playwright
PLAYWRIGHT_LOCAL_CONTAINER ?= go-office-playwright-server

DOCKER_IMAGE ?= ghcr.io/quantumx-apps/office-server:local
DOCKER_BUILDER_IMAGE ?= go-office:builder
DOCKER_CONTAINER ?= go-office-serve
DOCKER_PORT ?= 8080
DOCKER_PUBLIC ?= http://localhost:$(DOCKER_PORT)
SKIP_ASSET_FETCH ?= false

build-docker: build-docker-image

build-docker-image: check-docker check-samples
	@echo "==> Docker image $(DOCKER_IMAGE)$(if $(filter true,$(SKIP_ASSET_FETCH)), (stub assets),)"
	$(DOCKER_BUILD) -t "$(DOCKER_IMAGE)" -f _docker/Dockerfile \
		--build-arg SKIP_ASSET_FETCH=$(SKIP_ASSET_FETCH) \
		$(if $(OFFICE_DEBUG_LOGGING),--build-arg OFFICE_DEBUG_LOGGING=$(OFFICE_DEBUG_LOGGING),) .

run-docker: stop-docker check-docker
	@echo ""
	@echo "Document server (Docker) on $(DOCKER_PUBLIC)/"
	@echo "  site:    $(DOCKER_PUBLIC)/"
	@echo "  samples: $(DOCKER_PUBLIC)/demo/"
	@echo "Press Ctrl+C to stop"
	docker run --rm -p "$(DOCKER_PORT):80" --name "$(DOCKER_CONTAINER)" "$(DOCKER_IMAGE)" \
		-public "$(DOCKER_PUBLIC)"

stop-docker:
	@docker rm -f "$(DOCKER_CONTAINER)" 2>/dev/null || true

test-integration:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make test-integration-native
else
	@$(MAKE) test-integration-native
endif

test-convert-linux:
ifeq ($(USE_DOCKER_DEV),1)
	@$(MAKE) docker-dev-image
	$(DOCKER_DEV_RUN) $(DOCKER_DEV_IMAGE) make test-convert-linux-native
else
	@$(MAKE) test-convert-linux-native
endif

test-integration-native: build-native
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) test -tags=integration ./integration/... -count=1

test-convert-linux-native: build-native
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) test ./internal/convert/... -count=1

test-save-integration: build
	OFFICE_ASSETS="$(OFFICE_ASSETS)" $(GO) test -tags=integration ./integration/... -race -count=1

playwright-npm:
	@echo "==> Playwright npm dependencies"
	cd frontend && npm install

test-playwright: ensure-assets check-sample-matrix check-docker
	@echo "==> Playwright E2E (Docker)"
	GOOS=linux $(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office
	$(DOCKER_BUILD) -t "$(PLAYWRIGHT_IMAGE)" --target test -f "$(PLAYWRIGHT_DOCKERFILE)" .

test-playwright-ui: build check-sample-matrix check-docker
	@echo "==> Playwright UI (server in Docker, tests on host)"
	GOOS=linux $(GO) build -o "$(GO_OFFICE_BIN)" ./cmd/go-office
	docker rm -f "$(PLAYWRIGHT_LOCAL_CONTAINER)" 2>/dev/null || true
	$(DOCKER_BUILD) -t "$(PLAYWRIGHT_IMAGE)" --target server -f "$(PLAYWRIGHT_DOCKERFILE)" .
	docker run -d -p 8080:8080 --name "$(PLAYWRIGHT_LOCAL_CONTAINER)" "$(PLAYWRIGHT_IMAGE)"
	cd frontend && npm install && npx playwright install chromium
	@echo "Open Playwright UI — server at http://127.0.0.1:8080/"
	cd frontend && npx playwright test --ui

clean:
	rm -rf "$(BIN_DIR)" "$(OFFICE_ASSETS)"
	$(GO) clean -testcache
