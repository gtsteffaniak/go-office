# Release schedule and version matrix

go-office uses [semantic versioning](https://semver.org/) for the Go module and Docker images. Each release embeds exactly one Euro-Office asset pin (`scripts/euro-office.version` → `pkg/office.ExpectedAssetsVersion`).

## Version matrix

| go-office tag | Euro-Office pin | Docker tags |
| ------------- | --------------- | ------------- |
| v0.1.0 | 9.3.4-hotfix.1 | `latest`, `9.3.4-hotfix.1` |
| v0.2.0 | 9.3.4-hotfix.1 | `latest`, `9.3.4-hotfix.1`, `v0.2.0` |

Pin by editor version: `docker pull ghcr.io/quantumx-apps/office-server:9.3.4-hotfix.1`

Pin by go-office release: `docker pull ghcr.io/quantumx-apps/office-server:v0.2.0`

Go module: `go get github.com/quantumx-apps/go-office@v0.2.0`

## Release schedule

- **Patch** (`v0.2.x`): Euro-Office hotfix pin bumps, bug fixes, documentation fixes — as needed
- **Minor** (`v0.3.0`): new integrator-facing features — quarterly at most while stabilizing
- **Docker `latest`**: tracks the most recent build from `main`
- **Docker Euro-Office tag** (`9.3.4-hotfix.1`): tracks the bundled editor/converter version

## Bumping Euro-Office

1. Edit `EURO_OFFICE_VERSION` in `scripts/euro-office.version`
2. Run `make build` (re-fetches assets)
3. Run `go run ./cmd/gen-assets-version` (updates `pkg/office/assets_version.go`)
4. Run tests (`make test`, `make test-integration`, `make test-playwright`)
5. Release a new go-office tag (patch if only the pin changed)

## Out of scope (deferred)

These are documented in [api.md](api.md) but not planned for near-term releases:

- Multi-user co-editing on the same document key
- WebSocket coauthoring transport (polling works today)
- `POST /command` HTTP service
- WOPI (`/hosting/discovery`, file operations)
- Spell checker service (`/spellchecker/`)
- Webhooks
- Async `/converter`

## AGPL note

Runtime `FetchAssets` / `EnsureAssets` download Euro-Office assets under AGPL-3.0. Each asset tree includes `assets/PROVENANCE` with the source release and fetch timestamp.
