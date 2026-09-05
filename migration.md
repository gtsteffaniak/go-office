# Migrating from ONLYOFFICE Document Server to go-office

This guide covers replacing the official `onlyoffice/documentserver` Docker image (or equivalent package install) with [go-office](https://github.com/quantumx-apps/go-office) published as **`ghcr.io/quantumx-apps/office-server`**.

go-office implements the **browser-facing Document Server API** (static editor assets, coauthoring handshake, document open). It does **not** yet replicate every administrative feature of the full ONLYOFFICE stack (see [Limitations](#limitations) below).

## Quick swap (Docker Compose)

**Before** (official Document Server):

```yaml
services:
  onlyoffice:
    image: onlyoffice/documentserver
    environment:
      JWT_SECRET: your-shared-secret
    ports:
      - "9052:80"
```

**After** (go-office — `OFFICE_JWT_SECRET` preferred; ONLYOFFICE `JWT_SECRET` still works):

```yaml
services:
  onlyoffice:
    image: ghcr.io/quantumx-apps/office-server:latest
    environment:
      JWT_SECRET: your-shared-secret   # ONLYOFFICE name; OFFICE_JWT_SECRET also accepted
    ports:
      - "9052:80"
```

Your integrator (FileBrowser, Nextcloud ONLYOFFICE app, custom app) should keep the same `documentServerUrl` — only the container image changes. JWT can keep the ONLYOFFICE variable name (`JWT_SECRET`) or use the go-office name (`OFFICE_JWT_SECRET`). The editor loads `api.js` from the site root:

```text
http://your-host:9052/web-apps/apps/api/documents/api.js
```

## Environment variables

go-office uses the **`OFFICE_` prefix** for its own settings. Where ONLYOFFICE Document Server already defines a variable for the same purpose, go-office accepts the **ONLYOFFICE name as a fallback** when the `OFFICE_` name is unset.

**Resolution order:** `OFFICE_*` → ONLYOFFICE fallback (if any) → default.

CLI flags (`-assets`, `-addr`, `-jwt`, `-disable-samples`, …) override environment when passed explicitly.

### go-office variables

| Variable | Purpose | Default (`office-server` image) |
| -------- | ------- | ------------------------------- |
| `OFFICE_ASSETS` | Path to Euro-Office assets (`web-apps/`, `sdkjs/`, `converter/`) | `/app/assets` |
| `OFFICE_ADDR` | Listen address | `:80` (image); `:8080` (`make serve`) |
| `OFFICE_DATA_DIR` | Data root (demo samples, cache parent) | `/app` |
| `OFFICE_SAMPLES_DIR` | Sample files directory relative to data dir | `sample-files` |
| `OFFICE_DISABLE_SAMPLES` | `1` / `true` — hide demo UI and skip sample dir check | unset (samples **on**) |
| `OFFICE_SKIP_ASSET_FETCH` | `1` / `true` — do not download assets at startup when missing | unset |
| `OFFICE_JWT_SECRET` | HMAC secret for editor config, converter, and callback JWT | unset (no JWT) |
| `OFFICE_JWT_ENABLED` | `false` / `0` — disable JWT even if a secret is set | unset (enabled when secret set) |
| `OFFICE_PUBLIC_ORIGIN` | Public URL used in generated document/cache links | inferred from `OFFICE_ADDR` / request |
| `OFFICE_BASE_PATH` | Mount prefix for editor routes | `/` (site root) |
| `OFFICE_API_BASE` | Demo API prefix (`/demo/config`, etc.) | `/api/office` |
| `OFFICE_VERSION` | Coauthoring protocol version string | read from `assets/VERSION` |
| `OFFICE_DEBUG_LOGGING` | Verbose logging (`1` / `true`) | unset |
| `OFFICE_DEBUG` | Legacy alias for `OFFICE_DEBUG_LOGGING` | unset |
| `OFFICE_LOG_JSON` | JSON log lines on stderr (`1` / `true`) | unset (text) |
| `OFFICE_POLL_HOLD` | Coauthoring long-poll hold (`0`, `2s`, `500ms`, …) | `2s` production default |
| `OFFICE_CONVERT_LIMIT` | Max concurrent x2t subprocesses | `2` |
| `OFFICE_SAVE_DELAY` | Coauthoring save debounce before flush | `5s` production default |

### ONLYOFFICE Document Server fallbacks

These ONLYOFFICE variables are read **only when** the matching `OFFICE_` variable is unset (except `JWT_ENABLED`, which pairs with either secret name).

| ONLYOFFICE variable | go-office equivalent | Supported | Notes |
| ------------------- | -------------------- | :-------: | ----- |
| `JWT_SECRET` | `OFFICE_JWT_SECRET` | ✅ | Same secret value; preferred for drop-in compose files |
| `JWT_ENABLED` | `OFFICE_JWT_ENABLED` | ✅ | `false` disables JWT even if `JWT_SECRET` is set |
| `JWT_HEADER` | — | ❌ | go-office uses `Authorization: Bearer` only; not configurable |
| `JWT_IN_BODY` | — | ⚠️ | When a secret is set, callbacks are signed as `{"token":"…"}` (ONLYOFFICE `JWT_IN_BODY=true` style). Header-only callback JWT is not wired. |
| `DB_*` (`DB_TYPE`, `DB_HOST`, …) | — | ❌ | No PostgreSQL — single process |
| `REDIS_*` | — | ❌ | No Redis |
| `AMQP_*` | — | ❌ | No RabbitMQ |
| `WOPI_ENABLED` | — | ❌ | WOPI not supported |
| `USE_UNAUTHORIZED_STORAGE` | — | ❌ | Not implemented |
| `ALLOW_PRIVATE_IP_ADDRESS` | — | ❌ | Document downloads are not restricted by IP class |
| `ALLOW_META_IP_ADDRESS` | — | ❌ | Not implemented |
| `GENERATE_FONTS` | — | ❌ | Fonts ship with Euro-Office assets |
| `ONLYOFFICE_HTTPS_HSTS_*` | — | ❌ | Use your reverse proxy for TLS/HSTS |

### Early development aliases (deprecated)

| Old name | Use instead |
| -------- | ----------- |
| `GO_OFFICE_ASSETS` | `OFFICE_ASSETS` |
| `GO_OFFICE_DEBUG` | `OFFICE_DEBUG_LOGGING` |

These are **not** read by current builds.

## URL and port compatibility

**Legend:** ✅ supported · ⚠️ partial · ❌ not supported

| Endpoint | ONLYOFFICE Document Server | go-office |
| -------- | -------------------------- | --------- |
| Editor `api.js` | `/web-apps/apps/api/documents/api.js` | ✅ |
| Coauthoring | `/{version}/doc/{key}/c/` (primary) or `/doc/{key}/c/` | ✅ polling; ❌ WebSocket (501) |
| Document cache | `/cache/files/{key}/…` | ✅ |
| Health (JSON) | varies | ✅ (`/health` extended JSON) |
| Health (compat) | `/healthcheck` → `true` | ✅ |
| Info | `/info/info.json` | ✅ |
| Site home | welcome page / nginx default | ✅ (`/` AGPL home page) |
| Demo samples | not included | ✅ `/demo/` (disable with `OFFICE_DISABLE_SAMPLES`) |

**Default listen port:** `80` inside the `office-server` image (map host port as you did before, e.g. `9052:80`).

**Base path:** Editor assets are served at the **site root** (`OFFICE_BASE_PATH=/`), matching typical `documentServerUrl` like `http://host:9052/`. If your integrator mounted ONLYOFFICE under a subpath, set `OFFICE_BASE_PATH` accordingly (FileBrowser embedded mode may use `/office` later).

## Integrator checklist

1. **Image** — `ghcr.io/quantumx-apps/office-server:latest`, a Euro-Office tag (e.g. `9.3.4-hotfix.1`), or a go-office tag (e.g. `v0.2.0`).
2. **JWT** — Keep `JWT_SECRET` from your ONLYOFFICE compose file, or rename to `OFFICE_JWT_SECRET`. Same value on document server and integrator. Set `JWT_ENABLED=false` / `OFFICE_JWT_ENABLED=false` to disable.
3. **documentServerUrl** — Unchanged URL shape; must end with `/` and serve `web-apps/…/api.js`.
4. **document.url / callbackUrl** — Still point at your integrator; go-office fetches documents from those URLs on open.
5. **Health checks** — Update probes to `GET /healthcheck` (expects body `true`) or `GET /health` (JSON).
6. **License / AGPL** — go-office is AGPL-3.0. Editor UI attribution (ONLYOFFICE logo) must remain visible; see [NOTICE](NOTICE).

## What works today

- Open documents (word, cell, slide, PDF) via coauthoring + x2t (or `origin.pdf` for PDF)
- Static assets, fonts, `api.js`
- Coauthoring polling transport (license + auth + `documentOpen`)
- JWT signing of editor config when `OFFICE_JWT_SECRET` or `JWT_SECRET` is set
- `POST /session/reset?key=` and automatic session clear in `BuildEditorConfig` (fixes editor page reload)
- Demo warm (`GET /demo/warm`) for faster sample opens
- Demo UI and bundled sample files (unless disabled)

## Limitations

Plan accordingly before migrating production **edit-and-save** workflows:

**Legend:** ✅ supported · ⚠️ partial · ❌ not supported

| Feature | ONLYOFFICE Document Server | go-office (current) |
| ------- | -------------------------- | ------------------- |
| Save / force-save to integrator | ✅ | ✅ (coauthoring save → reverse x2t → callback → `Storage.Save`) |
| Multi-user co-editing | ✅ | ❌ |
| WebSocket coauthoring | ✅ | ⚠️ Polling only (501 on WS upgrade) |
| PostgreSQL / Redis / clustering | ✅ | ❌ (single process) |
| WOPI | ✅ | ❌ |
| `POST /converter` (conversion API / thumbnails) | ✅ | ✅ (sync only; `async: true` not supported) — see [api.md](api.md#12-conversion-api-filebrowser-previews-printexport-pipelines) |
| Callback status 1 / 4 (editing telemetry) | ✅ | ❌ — not emitted outbound (see [api.md](api.md#32-callback--integrator-receives-posts-document-server--your-app)) |
| `POST /coauthoring/CommandService.ashx` | ✅ | ❌ |
| `GET /hosting/discovery` (WOPI) | ✅ | ❌ |
| `/spellchecker/` | ✅ | ❌ |
| Spell checker service | ⚠️ Optional | ❌ |
| Admin panel | ✅ | ❌ (port 9000 in full install) |

Opening and viewing documents in the editor works; **saving edits back to storage** is supported via coauthoring flush, reverse x2t, and the integrator callback. See [api.md](api.md) for the full compatibility matrix.

## FileBrowser

FileBrowser today expects an external `onlyOfficeUrl` and signs config with `integrations.office.secret`. When pointing at go-office:

- Set `onlyOfficeUrl` to this server’s public URL (e.g. `http://files.example.com:9052/`).
- Use the **same** secret as `OFFICE_JWT_SECRET` (or `JWT_SECRET`).
- On each editor config request, FileBrowser should call `POST {go-office}/session/reset?key={documentKey}` (or use go-office `BuildEditorConfig`, which clears the session automatically).
- **Office grid previews:** `POST {onlyOfficeUrl}/converter` with `outputtype: "jpg"` (sync thumbnail conversion).
- Run `office-server` as a sidecar or standalone container.

### Library integration (Go host app)

`office.New()` never downloads assets. Call asset helpers explicitly before creating the server:

```go
bundle, err := office.EnsureAssets(ctx, office.EnsureAssetsOptions{
    AssetOptions: office.AssetOptions{Dir: os.Getenv("OFFICE_ASSETS")},
})
srv, err := office.New(store, office.Options{
    AssetDir:        bundle.Dir,
    ProtocolVersion: bundle.Version,
    JWTSecret:       []byte(secret),
})
```

Use `office.DiscoverAssets` when assets are pre-installed and you must not download at runtime.

## Troubleshooting

| Symptom | Likely cause |
| ------- | ------------ |
| Page reload spins forever | Stale coauthoring session — call `POST /session/reset?key=` when minting config, or use `BuildEditorConfig`. |
| Editor loads but document never opens | x2t/conversion error — check server logs (`OFFICE_DEBUG_LOGGING=1` or `-debug`). go-office must **reach** the `document.url` from inside its container (not just the browser). |
| `document open failed` / download errors | FileBrowser `document.url` uses an internal hostname (e.g. `http://beta-large/...`) that the go-office container cannot resolve. Put both on the same Docker network or use a URL reachable from go-office. |
| Wrong `cache/files` URLs / mixed content | Set `OFFICE_PUBLIC_ORIGIN=https://your-public-host` (or ensure reverse proxy sends `X-Forwarded-Proto` / `X-Forwarded-Host`). |
| `plugins.json` 404 | Fixed in current go-office (`[]` stub). Harmless on older builds. |
| WebSocket `501` on `/doc/.../c/` | Expected — sdkjs falls back to polling automatically. Not an error. |
| “Token” / JWT errors | `OFFICE_JWT_SECRET` / `JWT_SECRET` mismatch with integrator, or `JWT_ENABLED=false` on one side only. |
| `api.js` 404 | Wrong `documentServerUrl` or `OFFICE_BASE_PATH` does not match how the URL is constructed. |
| Health check fails | Probe still targeting internal port `8000`; use port `80` on the container. |
| PDF stalls | Ensure go-office version includes PDF `origin.pdf` open path (not x2t). |

## Version tags

Docker images are tagged with:

- `latest` — current `main` build
- Euro-Office release tag (e.g. `9.3.4-hotfix.1`) — matches bundled `assets/VERSION`
- go-office release tag (e.g. `v0.2.0`) — on git tag releases; see [RELEASE.md](RELEASE.md) for the Euro-Office pin paired with each tag

Pin the Euro-Office tag when you care about editor/sdkjs protocol alignment. Pin the go-office tag when you care about the server/library release.

## Further reading

- [README.md](README.md) — build, demo, library integration
- [CHANGELOG.md](CHANGELOG.md) — release notes
- [NOTICE](NOTICE) — AGPL and branding obligations
