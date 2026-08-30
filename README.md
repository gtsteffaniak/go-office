# go-office

Embedded Go document server library compatible with ONLYOFFICE / Euro-Office browser clients.

`go-office` lets host applications (such as [FileBrowser](https://github.com/filebrowser/filebrowser)) serve the ONLYOFFICE editor UI and coauthoring protocol without running a separate Document Server container. Host apps implement a `Storage` interface for direct VFS access.

**License:** [GNU Affero General Public License v3.0](LICENSE)

**Platform:** Linux only (`linux/amd64`, `linux/arm64`). Euro-Office assets and x2t are Linux binaries. The Go library is portable and can be cross-compiled into a Windows/macOS host binary, but the document server must run on Linux with a Linux `AssetDir`.

**Go:** 1.25 or newer. `go.mod` pins `toolchain go1.25.14` so the Go command auto-downloads a full patch release (bare `go1.25` is not published). `make setup` verifies your toolchain.

## Status

**Phase 0 complete** — static assets, coauthoring polling handshake, x2t document open, demo UI, full sample matrix, Playwright E2E in CI.

**Phase 1 in progress** — save/force-save back to disk (`Storage.Save` + reverse x2t). Multi-user co-editing is Phase 3.

## Quick start (Makefile)

**Develop in WSL** — use the Linux clone at `~/git/go-office` (not the Windows path under `/mnt/c/`).

Linux or WSL only. Requires **Go 1.25+**. Targets stack: **setup → build → demo**.

```bash
cd ~/git/go-office
make setup    # once: Go module dependencies
make build    # fetch Euro-Office assets (~600MB) + compile bin/go-office
make demo     # build (if needed) and start server
make test     # unit tests (no assets)
```

| Target | What it does |
|--------|----------------|
| `make setup` | Verify Go 1.25+, `go mod download`, verify Linux |
| `make build` | Download Euro-Office assets into `./assets/`, compile `bin/go-office` |
| `make demo` | Runs `build` then starts the server on `:8080` |
| `make test` | `go test ./...` (no assets) |
| `make test-integration` | `build` then integration tests |
| `make test-playwright` | `build` then Playwright E2E in Docker |
| `make test-playwright-ui` | Demo server in Docker + Playwright UI on host |
| `make clean` | Remove `bin/` and `assets/` |

Useful variables: `ADDR=:8080`, `GO_OFFICE_ASSETS=./assets`, `SAMPLES_DIR=sample-files`.

## Euro-Office assets (Linux developers & CI)

Euro-Office editor files are **not** committed to this repo (AGPL, ~600MB). On **Linux**, fetch them once into `./assets/` (downloads the official `.deb` from GitHub releases; uses `dpkg-deb`, no Docker or root required):

```bash
make build
# or: go run ./cmd/fetch-assets
# or: bash scripts/fetch-assets.sh
```

The release is pinned in `scripts/euro-office.version` (currently `v9.3.4-hotfix.1`). Bump that file when upgrading Euro-Office, then re-run the fetch command.

**What gets extracted**

- `assets/web-apps/` — editor UI + `api.js`
- `assets/sdkjs/` — document engine
- `assets/converter/bin/` — Linux x2t binaries (Phase 1)
- `assets/VERSION` — protocol version for `Options.ProtocolVersion`

**Environment variable**

```bash
export GO_OFFICE_ASSETS=./assets
go run ./cmd/go-office -assets "$GO_OFFICE_ASSETS"
```

**CI** (`.github/workflows/ci.yml`, `ubuntu-latest` only)

- Unit tests on every push (no assets).
- Integration job: `go run ./cmd/fetch-assets`, cache `assets/`, then `go test -tags=integration ./...`.
- Playwright job: cache `assets/`, `make test-playwright` (skips sample files not yet in `sample-files/`).

## Playwright E2E tests

End-to-end tests mirror the [FileBrowser Playwright pattern](https://github.com/filebrowser/filebrowser): build the Linux server, start it in Docker, run Playwright (Firefox) against the demo UI.

```bash
make build              # Euro-Office assets required
# add samples under sample-files/ (see sample-files/README.md)
make test-playwright    # full CI-style run in Docker
make test-playwright-ui # server in Docker, Playwright --ui on host
```

Variables:

- `PLAYWRIGHT_SAMPLE_TIER=1|2|3` — which sample tiers to test (default `3`)
- `PLAYWRIGHT_STRICT=1` — fail if tier-1 samples are missing (enabled in Docker CI)

**Non-Linux workstations:** use WSL or let CI populate `assets/` — `fetch-assets` exits immediately on other OSes.

**Local without building from source**

You do **not** need to clone [Euro-Office/DocumentServer](https://github.com/Euro-Office/DocumentServer) unless you are hacking sdkjs. The fetch tool downloads the official GitHub release `.deb`.

## Local demo (no FileBrowser)

Linux only. After setup:

```bash
make demo
```

Open **http://localhost:8080/** for the site home page, then **http://localhost:8080/office/demo/** to pick a sample document.

| URL | Purpose |
|-----|---------|
| `/` | Site home (about, links, next steps) |
| `/office/demo/` | Landing page with links to sample documents |
| `/office/demo/view?file=sample-files/sample.docx` | Editor viewer |
| `/api/office/demo/config?file=sample-files/sample.docx` | Editor init JSON (API) |
| `/api/office/demo/file/sample-files/sample.docx` | Serves a sample document (API) |
| `/api/office/demo/callback` | Save callback stub (API) |
| `/office/health` | Health check |

Flags: `-data .`, `-samples sample-files`, `-base /office`, `-api-base /api/office`, `-assets`, `-addr`, `-public`.

## Quick start (library only)

1. Fetch Euro-Office assets (see above), then run the example server:

```bash
go run ./cmd/go-office -assets ./assets -addr :8080
```

With a custom application subpath:

```bash
go run ./cmd/go-office -assets /path/to/assets -base /myapp/office
```

3. Verify static assets (FileBrowser OfficeDebug check):

```
http://localhost:8080/office/web-apps/apps/api/documents/api.js
```

4. Health:

```
http://localhost:8080/office/health
```

## Integration

```go
srv, err := office.New(myStorage, office.Options{
    AssetDir:        "/var/office-assets",
    BasePath:        office.JoinBasePath(appBaseURL, ""), // e.g. "/myapp/office"
    JWTSecret:       []byte("shared-secret"),
    ProtocolVersion: protocol, // office.ReadAssetVersion(assetDir) after fetch
})
http.Handle("/", srv.Handler())

cfg, err := srv.BuildEditorConfig(ctx, config.EditorRequest{...})
```

`appBaseURL` is the host application's configured subpath (FileBrowser `http.baseURL`, e.g. `"/myapp/"`).
Host API routes like `/api/office/config` remain on the application mux; this library serves editor assets and coauthoring under `BasePath` (default `/office`).

Set the Vue `documentServerUrl` to `srv.DocumentServerURL(publicOrigin)`.

## Layout

| Path | Purpose |
|------|---------|
| `storage.go` | Host `Storage` interface |
| `server.go` | HTTP routes and editor config |
| `config/` | ONLYOFFICE-compatible editor JSON |
| `session/` | In-memory document sessions |
| `static/` | Asset file server with cache headers |
| `internal/ws/` | Coauthoring Engine.IO polling (AGPL community handshake) |
| `cmd/go-office/` | Local demo server with embedded test UI |

## Roadmap

- **Phase 0:** ✅ static assets, coauthoring handshake, x2t open, demo, Playwright E2E (16 sample formats)
- **Phase 1:** single-user edit + save, reverse x2t, `Storage` save path
- **Phase 2:** packaging, cache hardening, WS golden fixtures
- **Phase 3:** multi-user co-editing

## Third-party assets

Editor JavaScript (`web-apps`, `sdkjs`) and converter binaries (`x2t`) come from Euro-Office / ONLYOFFICE builds and are **not** included in this repository. See `THIRD_PARTY_NOTICES`.
