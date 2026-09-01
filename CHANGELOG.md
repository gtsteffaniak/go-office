# Changelog

All notable changes to **go-office** (the Go library and demo server in this repository) are documented here.

Euro-Office / ONLYOFFICE editor assets (`web-apps/`, `sdkjs/`, `x2t`) are **not** modified in this repository. They are fetched at build time from upstream releases (see `scripts/euro-office.version` and `THIRD_PARTY_NOTICES`). This project integrates and serves those components; it does not ship patched copies of them in git.

## [Unreleased]

## [0.2.0] - 2026-09-01

**Euro-Office pin:** `9.3.4-hotfix.1`

- **Go module:** `go get github.com/quantumx-apps/go-office@v0.2.0`
- **Docker:** `docker pull ghcr.io/quantumx-apps/office-server:v0.2.0` or `docker pull ghcr.io/quantumx-apps/office-server:9.3.4-hotfix.1` (images are tagged with both go-office semver and Euro-Office version)

### Library and assets

- **`office.DiscoverAssets`**, **`office.FetchAssets`**, **`office.EnsureAssets`** — caller-controlled Euro-Office asset discovery and download; `office.New()` never fetches.
- **`office.ValidAssetDir`** and **`office.ExpectedAssetsVersion`** (embedded pin from `scripts/euro-office.version`).
- Single **`EURO_OFFICE_VERSION`** pin (replaces `EURO_OFFICE_RELEASE` / `EURO_OFFICE_PROTOCOL`).
- **`cmd/gen-assets-version`** generates `pkg/office/assets_version.go`.
- **`cmd/go-office`**: calls `EnsureAssets` at startup; `-skip-asset-fetch` / `OFFICE_SKIP_ASSET_FETCH` for pre-baked images.

### Conversion API

- **`POST /converter`** and **`/ConvertService.ashx`** — sync conversion with JWT (Bearer + body token).
- Demo landing thumbnails via **`GET /api/office/demo/thumbnail`**.

### Server hardening (from v0.1.0 development)

- Cache lifecycle, graceful shutdown, callback JWT, WS golden fixtures.
- Content-aware Playwright E2E (`content`, `save`, `thumbnails` specs).

### Documentation

- **[README.md](README.md)** rewritten as current-state snapshot (no phase roadmap).
- **[api.md](api.md)** updated: `/converter` and FileBrowser previews marked compatible.
- **[RELEASE.md](RELEASE.md)** — version matrix and release schedule.
- **[migration.md](migration.md)** — converter/previews and library asset APIs.

## [0.1.0] - 2026-08-30

Initial public release: embedded Go document server, coauthoring polling handshake, x2t document open, demo UI, Playwright E2E, Docker demo image.

[Unreleased]: https://github.com/quantumx-apps/go-office/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/quantumx-apps/go-office/releases/tag/v0.2.0
[0.1.0]: https://github.com/quantumx-apps/go-office/releases/tag/v0.1.0
