# Changelog

All notable changes to **go-office** (the Go library and demo server in this repository) are documented here.

Euro-Office / ONLYOFFICE editor assets (`web-apps/`, `sdkjs/`, `x2t`) are **not** modified in this repository. They are fetched at build time from upstream releases (see `scripts/euro-office.version` and `THIRD_PARTY_NOTICES`). This project integrates and serves those components; it does not ship patched copies of them in git.

## [Unreleased]

### Phase 2 — Hardening + robust Playwright

#### Server hardening
- **Cache lifecycle** (`pkg/office/cache.go`): configurable TTL (default 24h) and max cache dirs (default 256); periodic janitor; cache eviction on sweep.
- **`Server.Close()`** stops coauthoring save timers and the cache janitor (no longer a no-op).
- **Graceful shutdown** in `cmd/go-office/main.go`: SIGINT/SIGTERM → `http.Server.Shutdown` → `srv.Close()`.
- **`/health`** extended with `sessions`, `cacheDirs`, and `cacheBytes`.
- **Callback JWT** (`pkg/callback/jwt.go`): sign outbound callback bodies as `{"token":"…"}` when `OFFICE_JWT_SECRET` is set; verify inbound JWT-wrapped callbacks in `HandleCallback`.
- **WS golden fixtures** (`internal/ws/fixtures/coauthoring.json`) and tests; `ResetSessionsForTest()` for isolated handler tests; `scripts/record-ws-fixtures.sh`.
- **Coauthoring `PollHold` fix**: `PollHold=0` disables long-poll wait in tests (was incorrectly falling back to 20s).

#### Playwright (content-aware E2E)
- **Sample manifest** (`scripts/extract-sample-expectations.go`, `frontend/tests/playwright/fixtures/sample-manifest.json|.ts`) — expected cell/text per sample file.
- **Editor helpers** (`frontend/tests/playwright/editor.ts`): `waitForDocumentReady`, `assertCellContent` (formula bar), `assertDocumentContains`, `setCellContent`, `waitForSaveDone`.
- **`content.spec.ts`** — tier-1 content assertions; `sample.csv` B2 via formula bar (not canvas `getByText`).
- **`save.spec.ts`** — CSV + DOCX save round-trip verified via `/api/office/demo/file/…`.
- **Demo viewer hooks** (`internal/demo/viewer.html`): `data-document-ready` and `data-save-done` via DocsAPI `onDocumentReady` / `onRequestSaveResult`.
- **`open-formats.spec.ts`** uses `waitForDocumentReady` instead of fixed sleeps.
- **CI**: Docker Playwright uses `PLAYWRIGHT_SAMPLE_TIER=1`, `PLAYWRIGHT_WORKERS=6`, `OFFICE_POLL_HOLD=0`, and `OFFICE_CONVERT_LIMIT=4` for parallel E2E without coauthoring long-poll pile-ups.
- **`make extract-sample-manifest`** target.

#### Docs, demo UX, API reference
- **`migration.md`**: JWT_IN_BODY mapping; save support accurate; FileBrowser `/converter` preview gap; expanded limitations.
- **`api.md`**: full ONLYOFFICE ↔ go-office compatibility audit (conversion, command, WOPI, spellchecker, callback payload gaps, legacy `.ashx` paths).
- **`GET /docs/api#compatibility`**: expanded side-by-side matrix on the live API docs page.
- **Demo landing links** use relative paths (`/demo/view?file=…`) instead of `http://localhost/…` URLs.
- **Homepage / API docs**: coauthoring paths document both `/{version}/doc/{key}/c/` and `/doc/{key}/c/`.
- **README** roadmap marks Phase 2 done.

### Licensing and compliance

- Clarified that users do **not** need to click or sign a EULA to download, host, or run go-office or its Docker images. Rights under [AGPL-3.0](LICENSE) are granted automatically.
- Documented AGPL compliance expectations (source availability, license text, modification notices) in README and NOTICE.
- Documented AGPL Section 7 branding obligations for ONLYOFFICE / Euro-Office editor UI (attribution must not be removed or replaced).
- Added OCI image labels (`org.opencontainers.image.source`, `org.opencontainers.image.licenses`) to published Dockerfiles.

## [0.1.0] - 2026-08-30

Initial public release: embedded Go document server, coauthoring polling handshake, x2t document open, demo UI, Playwright E2E, Docker demo image.

[Unreleased]: https://github.com/quantumx-apps/go-office/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/quantumx-apps/go-office/releases/tag/v0.1.0
