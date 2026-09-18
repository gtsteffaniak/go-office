# go-office

Embedded Go document server library compatible with ONLYOFFICE / Euro-Office browser clients.

`go-office` lets host applications (such as [FileBrowser](https://github.com/filebrowser/filebrowser)) serve the ONLYOFFICE editor UI and coauthoring protocol without running a separate Document Server container. Host apps implement a `Storage` interface for direct VFS access.

**License:** [GNU Affero General Public License v3.0](LICENSE) — rights are granted automatically; no click-through EULA is required to use, build, host, or run this software.

**Source:** https://github.com/quantumx-apps/go-office

**Platform:** The document server runs on Linux (`linux/amd64`, `linux/arm64`) with Linux Euro-Office assets and x2t. On macOS and Windows, use Docker (`make serve`, `make build-docker`) or the [dev container](.devcontainer/devcontainer.json).

**Go:** 1.27 or newer (`go.mod` requires Go 1.27). `make setup` verifies your toolchain.

**Compatibility:** See [api.md](api.md) for the full ONLYOFFICE compatibility audit and known gaps. See [RELEASE.md](RELEASE.md) for version tags and release schedule.

## What works today

- Embedded Go library (`pkg/office`) and `office-server` Docker image (`ghcr.io/quantumx-apps/office-server`)
- Single-user open → edit → autosave → callback → persist
- `POST /converter` and `/ConvertService.ashx` (sync conversion, JPG thumbnails for FileBrowser previews)
- Coauthoring over Engine.IO **polling** (WebSocket upgrade returns 501; sdkjs falls back automatically)
- Demo UI, sample matrix, and Playwright E2E in CI
- Runtime asset discovery/fetch via `DiscoverAssets`, `FetchAssets`, and `EnsureAssets` (caller-controlled; `office.New` never downloads)
- Demo warm endpoint (`GET /demo/warm`) pre-converts samples; coauthoring session reset on config (`BuildEditorConfig`, `POST /session/reset`)

## Quick start (Makefile)

Requires [Docker](https://docs.docker.com/get-docker/) on macOS and Windows. Linux can run natively or via Docker.

Open in the **dev container** (VS Code / Cursor: *Reopen in Container*) for a full Linux toolchain with Go 1.27, Node, and Docker-in-Docker.

```bash
make setup    # once: Go module dependencies
make build    # fetch Euro-Office assets (~600MB) + compile bin/go-office
make serve    # build (if needed) and start server on :8080
make test     # unit tests (no assets)
```

| Target | What it does |
|--------|----------------|
| `make setup` | Verify Go 1.27+, `go mod download` |
| `make build` | Download Euro-Office assets into `./assets/`, compile `bin/go-office` |
| `make serve` | Runs `build` then starts the server on `:8080` |
| `make test` | `go test -race ./...` (no assets) |
| `make test-integration` | `build` then integration tests |
| `make test-playwright` | `build` then Playwright E2E in Docker |
| `make test-playwright-ui` | Demo server in Docker + Playwright UI on host |
| `make build-docker` | Build `office-server` Docker image |
| `make run-docker` | Run built image on host port 8080 → container 80 |
| `make clean` | Remove `bin/` and downloaded `assets/` |

Useful environment variables: `OFFICE_JWT_SECRET` (or ONLYOFFICE `JWT_SECRET`), `OFFICE_ASSETS`, `OFFICE_ADDR`, `OFFICE_DISABLE_SAMPLES`, `OFFICE_DEBUG_LOGGING`, `OFFICE_LOG_JSON`. See [migration.md](migration.md) for the full list and ONLYOFFICE Document Server mapping when replacing `onlyoffice/documentserver`.

## Euro-Office assets

Euro-Office editor files are **not** committed to this repo (AGPL, ~600MB). On **Linux**, fetch them into `./assets/`:

```bash
make build
# or: go run ./cmd/fetch-assets
```

The release is pinned in `scripts/euro-office.version` as `EURO_OFFICE_VERSION` (currently `v9.3.4-hotfix.1`). Bump that file when upgrading Euro-Office, then re-run `make build`.

**What gets extracted**

- `assets/web-apps/` — editor UI + `api.js`
- `assets/sdkjs/` — document engine
- `assets/converter/bin/` — Linux x2t binaries
- `assets/VERSION` — Euro-Office version string for `Options.ProtocolVersion`

**Non-Linux workstations:** run `make build` / `make serve` (uses Docker automatically), use the dev container, or pre-populate `AssetDir`.

## Docker image

```bash
make build-docker
make run-docker    # http://localhost:8080/
docker pull ghcr.io/quantumx-apps/office-server:latest
# or pin: ghcr.io/quantumx-apps/office-server:9.3.4-hotfix.1
# or pin: ghcr.io/quantumx-apps/office-server:v0.2.0
```

The image bundles `go-office`, Euro-Office assets, and `sample-files/`. Container listens on **port 80**. See [migration.md](migration.md).

## Local server

```bash
make serve
```

Open **http://localhost:8080/** and **http://localhost:8080/demo/**.

| URL | Purpose |
|-----|---------|
| `/` | Site home |
| `/demo/` | Sample document landing |
| `/demo/view?file=sample-files/sample.docx` | Editor viewer |
| `/health` | Health check (JSON) |
| `/healthcheck` | ONLYOFFICE-compatible health (`true`) |
| `/session/reset?key=` | Clear coauthoring session before editor reload (POST, `204`) |
| `/web-apps/apps/api/documents/api.js` | Integrator `api.js` |

## Library integration

`office.New()` never fetches assets. The host application decides **when** to download or discover them:

```go
import (
    "context"
    "log"

    office "github.com/quantumx-apps/go-office/pkg/office"
    "github.com/quantumx-apps/go-office/pkg/config"
)

// Option A: discover or fetch at startup (blocking)
bundle, err := office.EnsureAssets(ctx, office.EnsureAssetsOptions{
    AssetOptions: office.AssetOptions{Dir: os.Getenv("OFFICE_ASSETS")},
})
if err != nil {
    log.Fatal(err)
}

// Option B: pre-installed assets only
// bundle, ok, err := office.DiscoverAssets(office.AssetOptions{Dir: "/var/office-assets"})
// if !ok { log.Fatal("assets not found") }

srv, err := office.New(myStorage, office.Options{
    AssetDir:        bundle.Dir,
    BasePath:        office.DefaultBasePath,
    JWTSecret:       []byte("shared-secret"),
    ProtocolVersion: bundle.Version,
})
if err != nil {
    log.Fatal(err)
}
http.Handle("/", srv.Handler())

cfg, err := srv.BuildEditorConfig(ctx, config.EditorRequest{...})
```

Set the Vue `documentServerUrl` to `srv.DocumentServerURL(publicOrigin)`.

`go get github.com/quantumx-apps/go-office@v0.2.0`

## Playwright E2E

```bash
make build
make check-sample-matrix
make test-playwright
```

Playwright Docker defaults: `PLAYWRIGHT_WORKERS=10`, `OFFICE_CONVERT_LIMIT=4`. Override via environment when running locally or in CI.

## Layout

| Path | Purpose |
|------|---------|
| `pkg/office/` | Public library API (`Server`, `Storage`, asset helpers) |
| `pkg/config/` | ONLYOFFICE-compatible editor JSON |
| `internal/ws/` | Coauthoring Engine.IO polling |
| `internal/assetfetch/` | Euro-Office `.deb` download and extraction |
| `cmd/go-office/` | Standalone `office-server` binary |
| `api.md` | Full ONLYOFFICE compatibility matrix |
| `migration.md` | Replacing `onlyoffice/documentserver` |
| `RELEASE.md` | Version tags and Euro-Office pin matrix |

## License and AGPL compliance

go-office is licensed under [AGPL-3.0](LICENSE). To comply when you distribute go-office or a derived work:

- Include the full [LICENSE](LICENSE) text.
- Make corresponding source available (https://github.com/quantumx-apps/go-office).
- Preserve [NOTICE](NOTICE) and [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).

## Branding (AGPL Section 7)

The ONLYOFFICE / Euro-Office editor UI includes logos and attribution that **must not be removed, obscured, or replaced** in production deployments unless you have separate permission from the rights holder. See [NOTICE](NOTICE).

## Third-party assets

Editor JavaScript (`web-apps`, `sdkjs`) and converter binaries (`x2t`) come from Euro-Office / ONLYOFFICE builds and are fetched at build time. See [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES) and [CHANGELOG.md](CHANGELOG.md).
