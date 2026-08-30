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

**After** (go-office):

```yaml
services:
  onlyoffice:
    image: ghcr.io/quantumx-apps/office-server:latest
    environment:
      OFFICE_JWT_SECRET: your-shared-secret
    ports:
      - "9052:80"
```

Your integrator (FileBrowser, Nextcloud ONLYOFFICE app, custom app) should keep the same `documentServerUrl` — only the image and JWT environment variable name change. The editor loads `api.js` from the site root:

```text
http://your-host:9052/web-apps/apps/api/documents/api.js
```

## Environment variables

All go-office configuration uses the **`OFFICE_` prefix**. This is intentional: one namespace for the service, distinct from integrator-specific settings.

| go-office variable | Purpose | Default (Docker image) |
| ------------------ | ------- | ---------------------- |
| `OFFICE_ASSETS` | Path to Euro-Office assets (`web-apps/`, `sdkjs/`, `converter/`) | `/app/assets` |
| `OFFICE_ADDR` | Listen address | `:80` |
| `OFFICE_DATA_DIR` | Data root (demo samples, cache parent) | `/app` |
| `OFFICE_SAMPLES_DIR` | Sample files directory relative to data dir | `sample-files` |
| `OFFICE_DISABLE_SAMPLES` | Set to `1` / `true` to hide demo UI and skip sample dir check | unset (samples **on**) |
| `OFFICE_JWT_SECRET` | HMAC secret for signing/verifying editor JWT tokens | unset (no JWT) |
| `OFFICE_PUBLIC_ORIGIN` | Public URL used in generated document links | inferred from `OFFICE_ADDR` |
| `OFFICE_BASE_PATH` | Mount prefix for editor routes | `/` (site root) |
| `OFFICE_API_BASE` | Demo API prefix (`/demo/config`, etc.) | `/api/office` |
| `OFFICE_VERSION` | Protocol version string (coauthoring) | read from `assets/VERSION` |
| `OFFICE_DEBUG` | Verbose logging (`1` / `true`) | unset |

CLI flags (`-assets`, `-addr`, `-jwt`, `-disable-samples`, …) override environment when passed explicitly.

### Mapping from ONLYOFFICE Document Server

Official Document Server uses many variables for PostgreSQL, Redis, RabbitMQ, and internal services. go-office is a **single process** and ignores infrastructure the full stack requires.

| ONLYOFFICE / Docker-DocumentServer | go-office | Notes |
| ---------------------------------- | --------- | ----- |
| `JWT_SECRET` | **`OFFICE_JWT_SECRET`** | **Rename required.** Same secret value; must match integrator. |
| `JWT_ENABLED=true` | *(not used)* | JWT is enabled when `OFFICE_JWT_SECRET` is non-empty. |
| `JWT_HEADER` | *(not used)* | Standard `Authorization` header; same as typical ONLYOFFICE setups. |
| `JWT_IN_BODY` | *(not used)* | Callback body JWT parsing not implemented yet (Phase 1). |
| `DB_*`, `REDIS_*`, `AMQP_*` | *(none)* | Not used — no Postgres/Redis/RabbitMQ. |
| `WOPI_*` | *(none)* | WOPI not supported. |

### Deprecated aliases (remove when convenient)

These were used during early development. Prefer `OFFICE_*` in new deployments.

| Deprecated | Use instead |
| ---------- | ----------- |
| `GO_OFFICE_ASSETS` | `OFFICE_ASSETS` |
| `GO_OFFICE_DEBUG` | `OFFICE_DEBUG` |

## URL and port compatibility

| Endpoint | ONLYOFFICE Document Server | go-office |
| -------- | -------------------------- | --------- |
| Editor `api.js` | `/web-apps/apps/api/documents/api.js` | Same |
| Coauthoring | `/doc/{key}/c/` or `/{version}/doc/{key}/c/` | Same |
| Document cache | `/cache/files/{key}/…` | Same |
| Health (JSON) | varies | `/health` |
| Health (compat) | `/healthcheck` → `true` | Same |
| Info | `/info/info.json` | Same (version field) |
| Site home | welcome page / nginx default | `/` (go-office about page + AGPL attribution) |
| Demo samples | not included | `/demo/` (disable with `OFFICE_DISABLE_SAMPLES`) |

**Default listen port:** `80` inside the `office-server` image (map host port as you did before, e.g. `9052:80`).

**Base path:** Editor assets are served at the **site root** (`OFFICE_BASE_PATH=/`), matching typical `documentServerUrl` like `http://host:9052/`. If your integrator mounted ONLYOFFICE under a subpath, set `OFFICE_BASE_PATH` accordingly (FileBrowser embedded mode may use `/office` later).

## Integrator checklist

1. **Image** — `ghcr.io/quantumx-apps/office-server:latest` or a version tag matching Euro-Office (e.g. `9.3.4-hotfix.1`).
2. **JWT** — Rename `JWT_SECRET` → `OFFICE_JWT_SECRET` with the **same value** on both document server and integrator.
3. **documentServerUrl** — Unchanged URL shape; must end with `/` and serve `web-apps/…/api.js`.
4. **document.url / callbackUrl** — Still point at your integrator; go-office fetches documents from those URLs on open.
5. **Health checks** — Update probes to `GET /healthcheck` (expects body `true`) or `GET /health` (JSON).
6. **License / AGPL** — go-office is AGPL-3.0. Editor UI attribution (ONLYOFFICE logo) must remain visible; see [NOTICE](NOTICE).

## What works today

- Open documents (word, cell, slide, PDF) via coauthoring + x2t (or `origin.pdf` for PDF)
- Static assets, fonts, `api.js`
- Coauthoring polling transport (license + auth + `documentOpen`)
- JWT signing of editor config when `OFFICE_JWT_SECRET` is set
- Demo UI and bundled sample files (unless disabled)

## Limitations

Plan accordingly before migrating production **edit-and-save** workflows:

| Feature | ONLYOFFICE Document Server | go-office (current) |
| ------- | -------------------------- | ------------------- |
| Save / force-save to integrator | Yes | **No** (callback stub only) — Phase 1 |
| Multi-user co-editing | Yes | **No** — Phase 3 |
| WebSocket coauthoring | Yes | Polling only (501 on WS upgrade) |
| PostgreSQL / Redis / clustering | Yes | **No** (single process) |
| WOPI | Yes | **No** |
| Spell checker service | Optional | **No** |
| Admin panel | Port 9000 | **No** |

Opening and viewing documents in the editor works; **persisting edits back to storage** is not complete yet. Validate your use case against [README.md](README.md) roadmap before cutover.

## FileBrowser

FileBrowser today expects an external `onlyOfficeUrl` and signs config with `integrations.office.secret`. When pointing at go-office:

- Set `onlyOfficeUrl` to this server’s public URL (e.g. `http://files.example.com:9052/`).
- Use the **same** secret as `OFFICE_JWT_SECRET`.
- Embedded mode (`//go:build office`) is planned; until then, run `office-server` as a sidecar or standalone container.

## Troubleshooting

| Symptom | Likely cause |
| ------- | ------------ |
| Editor loads but document never opens | x2t/conversion error — check server logs (`OFFICE_DEBUG=1`). |
| “Token” / JWT errors | `OFFICE_JWT_SECRET` mismatch with integrator, or secret still named `JWT_SECRET`. |
| `api.js` 404 | Wrong `documentServerUrl` or `OFFICE_BASE_PATH` does not match how the URL is constructed. |
| Health check fails | Probe still targeting internal port `8000`; use port `80` on the container. |
| PDF stalls | Ensure go-office version includes PDF `origin.pdf` open path (not x2t). |

## Version tags

Docker images are tagged with:

- `latest` — current `main` build
- Euro-Office release tag (e.g. `9.3.4-hotfix.1`) — matches bundled `assets/VERSION`

Pin the Euro-Office tag in production so `documentServerUrl`, sdkjs, and coauthoring protocol stay aligned.

## Further reading

- [README.md](README.md) — build, demo, library integration
- [CHANGELOG.md](CHANGELOG.md) — release notes
- [NOTICE](NOTICE) — AGPL and branding obligations
