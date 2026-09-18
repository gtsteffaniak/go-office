# go-office API compatibility reference

Audit of go-office against the official [ONLYOFFICE Docs API](https://api.onlyoffice.com/docs/docs-api/) and the HTTP surface of ONLYOFFICE Document Server. Last reviewed against go-office v0.2.0 (`OFFICE_VERSION` / bundled Euro-Office assets).

**Browsable summary:** `GET /docs/api#compatibility` on a running server.

**Legend**

| Symbol | Meaning |
| ------ | ------- |
| ✅ | Compatible — same path/contract; covered by tests or production use |
| ⚠️ | Partial — works with documented limits or reduced payload |
| ❌ | Not implemented — request fails or feature absent |
| — | N/A — not part of Docs API embed path |

---

## Audit scope

This document compares:

1. **Document Server HTTP routes** — what the browser and integrator HTTP clients call on `documentServerUrl`.
2. **Coauthoring wire protocol** — Socket.IO / Engine.IO packets after `api.js` loads.
3. **Integrator contract** — editor config, callback handler, JWT.
4. **Additional APIs** — conversion, command service, WOPI, spell checker, admin.
5. **Legacy paths** — pre-8.2 / pre-5.5 URLs still seen in the wild.

**In scope for “compatible”:** single-user **open → edit → autosave → callback → persist** (FileBrowser/Nextcloud-style Docs API integration).

**Out of scope:** features that require full Document Server clustering (Postgres, Redis, RabbitMQ, admin UI on port 9000).

---

## Executive summary

| Category | ✅ | ⚠️ | ❌ |
| -------- | - | - | - |
| Core editor embed (api.js, assets, coauthoring polling, cache, save) | 14 | 3 | 2 |
| Integrator callback (inbound to your app) | 4 | 4 | 0 |
| Outbound callbacks (go-office → your `callbackUrl`) | 2 | 1 | 4 |
| Conversion API | 6 | 1 | 2 |
| Command service | 0 | 1 | 9 |
| WOPI | 0 | 0 | 4 |
| Ancillary services | 2 | 2 | 4 |

**Bottom line:** Docs API **editing** and **sync conversion/thumbnails** (`POST /converter`, JPG output) are compatible. **Command service**, **WOPI**, **async conversion**, and **rich callback telemetry** are not.

For **legacy binary formats** (`.xls`, `.doc`, `.ppt`) the save path follows upstream's
`assemblyFormatAsOrigin` behaviour: assemble to native OOXML, attempt conversion back to the
original format, and on failure persist the OOXML bytes at the original path. The bundled x2t
cannot write any of these three formats (`xlsx→xls` exit 88, `docx→doc` exit 80, `pptx→ppt`
exit 88), so the rollback is the normal outcome for them. Rollbacks are **reported**, not
silent: `saveResult` carries `rolledBack` and `bridge`, and the server logs a warning.

---

## 1. Document Server HTTP routes

Paths are relative to `documentServerUrl` (default site root). `OFFICE_BASE_PATH` prefixes all rows when mounted under a subpath.

### 1.1 Core routes (required for editor embed)

| Endpoint | Method | ONLYOFFICE | go-office | Notes |
| -------- | ------ | :--------: | :---------: | ----- |
| `/web-apps/apps/api/documents/api.js` | GET | ✅ | ✅ | [Basic concepts](https://api.onlyoffice.com/docs/docs-api/get-started/basic-concepts/) |
| `/web-apps/**` | GET | ✅ | ✅ | Editor shells, `plugins.json` if present in asset bundle |
| `/sdkjs/**` | GET | ✅ | ✅ | Editor runtime |
| `/fonts/**` | GET | ✅ | ✅ | Font files (`AllFonts.js` lives under converter bin, loaded via sdkjs) |
| `/document_editor_service_worker.js` | GET | ✅ | ✅ | sdkjs service worker |
| `/{version}/doc/{key}/c/` | GET, POST | ✅ | ✅ | Primary coauthoring URL (7.3+). Query: `EIO=4`, `transport=polling`, optional `sid`, `t` |
| `/doc/{key}/c/` | GET, POST | ✅ | ✅ | Unversioned coauthoring path (also accepted) |
| `?shardkey={key}` on coauthoring | — | ✅ | ⚠️ | ONLYOFFICE 8.1+ load-balancing hint; **ignored** by go-office (harmless) |
| Coauthoring `transport=websocket` | GET | ✅ | ❌ | HTTP **501**; sdkjs falls back to polling |
| `/cache/files/{key}/**` | GET | ✅ | ✅ | `Editor.bin`, `origin.pdf`, `media/`, `changes/`, `saved.{ext}` |
| `/downloadfile/{key}` | GET, POST | ✅ | ⚠️ | [Server config](https://api.onlyoffice.com/docs/docs-api/get-started/configuration/server-config/) `downloadFileAllowExt`. go-office: Range requests ✅; POST body `{"url":"…"}` ✅; `token` in body **not parsed** |
| `/healthcheck` | GET | ✅ | ⚠️ | Body `true`. ONLYOFFICE also probes DB/Redis/broker; go-office returns `true` without those dependencies |
| `/health` | GET | ⚠️ | ✅ | go-office JSON: `status`, `version`, `sessions`, `cacheDirs`, `cacheBytes` |
| `/healthz` | GET | — | ✅ | Alias of `/health` (k8s convention; not ONLYOFFICE-specific) |
| `/session/reset?key={documentKey}` | POST | — | ✅ | Clears in-memory coauthoring session for a document key before a new editor page load. Integrators that build config outside `BuildEditorConfig` (e.g. FileBrowser) should call this when minting editor config. Returns `204 No Content`. |
| `/info/info.json` | GET | ✅ | ✅ | `{"version":"…"}` |
| `/api/office/demo/savestate?file={path}` | GET | — | ✅ | **go-office extension.** Last settled flush outcome for a document, for save verification without polling storage. Returns `{key, known, success, force, rolledBack, bridge, bytes, sequence, time, error}`; `known:false` means no flush has settled yet. Used by the demo viewer. |

### 1.2 Conversion API (FileBrowser previews, print/export pipelines)

| Endpoint | Method | ONLYOFFICE | go-office | Notes |
| -------- | ------ | :--------: | :---------: | ----- |
| **`/converter`** | **POST** | **✅** | **✅** | [Conversion API](https://api.onlyoffice.com/docs/docs-api/additional-api/conversion-api/). JSON body: `filetype`, `key`, `outputtype`, `url`, optional `thumbnail`, `async`, … |
| `/converter?shardkey={key}` | POST | ✅ | ⚠️ | Load-balancing query param (8.1+); **ignored** (harmless) |
| `Accept: application/json` on `/converter` | — | ✅ | ✅ | JSON response when `Accept: application/json` |
| **`/ConvertService.ashx`** | POST | ✅ (legacy) | ✅ | Pre-5.5 path; same handler as `/converter` |
| JWT: `Authorization: Bearer` on converter | — | ✅ | ✅ | `VerifyConverterJWT` when `OFFICE_JWT_SECRET` set |
| JWT: `{"token":"…"}` in converter body | — | ✅ | ✅ | Alternative ONLYOFFICE style |
| `async: true` on `/converter` | — | ✅ | ❌ | Synchronous conversion only |
| Internal x2t on editor open/save | — | ✅ | ✅ | Same binary |

**FileBrowser `GenerateOfficePreview`:** `POST {onlyOfficeUrl}/converter` with `outputtype: "jpg"` → **supported** (sync JPG thumbnail).

### 1.3 Command service (admin / operational)

| Endpoint | Method | ONLYOFFICE | go-office | Notes |
| -------- | ------ | :--------: | :---------: | ----- |
| **`/command`** | POST | ✅ | ❌ | [Command service](https://api.onlyoffice.com/docs/docs-api/additional-api/command-service/) |
| `/command?shardkey={key}` | POST | ✅ | ❌ | 8.1+ |
| **`/coauthoring/CommandService.ashx`** | POST | ✅ (legacy) | ❌ | Pre-8.2 path |
| Command `info` | — | ✅ | ❌ | Who is editing |
| Command `drop` | — | ✅ | ❌ | Disconnect users |
| Command `forcesave` | — | ✅ | ⚠️ | Editor `customization.forcesave` + coauthoring save works; **HTTP command** does not |
| Command `meta` | — | ✅ | ❌ | Update doc meta for all editors |
| Command `version` | — | ✅ | ❌ | Server version |
| Command `license` | — | ✅ | ❌ | License/quota info |
| Command `getForgotten` / `getForgottenList` / `deleteForgotten` | — | ✅ | ❌ | Forgotten-file recovery |

### 1.4 WOPI (alternative integration protocol)

| Endpoint | Method | ONLYOFFICE | go-office | Notes |
| -------- | ------ | :--------: | :---------: | ----- |
| **`/hosting/discovery`** | GET | ✅ | ❌ | [WOPI discovery](https://api.onlyoffice.com/docs/docs-api/using-wopi/wopi-discovery/) XML |
| WOPI file operations (`/wopi/files/…`) | * | ✅ | ❌ | [WOPI overview](https://api.onlyoffice.com/docs/docs-api/using-wopi/overview/) |
| WOPI proof keys / PostMessage | — | ✅ | ❌ | |
| Docs API vs WOPI | — | Both | **Docs API only** | Use `document.url` + `callbackUrl`, not `wopisrc` |

### 1.5 Ancillary / deployment-specific routes

| Endpoint | Method | ONLYOFFICE | go-office | Notes |
| -------- | ------ | :--------: | :---------: | ----- |
| **`/spellchecker/`** | * | ✅ | ❌ | Separate Node service; nginx proxies to port 8080. Editor works; spell-check calls fail silently or error in console |
| **`/example/`** (bundled test apps) | GET | ✅ | ✅ | Upstream ships a bundled test example — "a simple doc management system" for trying the editors before integration, disabled by default. go-office serves its demo app at this path as an **alias of `/demo/`**: same landing page, viewer, and warm endpoint. `/demo/` stays canonical; both render identically and generated links keep the prefix of the path you used. This is not upstream's Node.js example app. |
| Admin panel | GET | ✅ | ❌ | Port 9000 in full install |
| `/plugins.json` (root) | GET | ✅ | ✅ | Empty plugin list (`[]`) for mobile/desktop editors |
| `document-formats/onlyoffice-docs-formats.json` | GET | ⚠️ | ⚠️ | Referenced in server config; served only if present in asset tree |
| Welcome / example nginx default page | GET | ✅ | — | go-office serves AGPL **home page** at `/` instead |

---

## 2. Coauthoring protocol (sdkjs ↔ Document Server)

Transport: Engine.IO v4 / Socket.IO. Reference: [Co-editing](https://api.onlyoffice.com/docs/docs-api/get-started/how-it-works/co-editing/).

| Packet / message | ONLYOFFICE | go-office | Notes |
| ---------------- | :--------: | :---------: | ----- |
| `0{"sid":…}` open | ✅ | ✅ | Session start |
| `40{"sid":…}` namespace | ✅ | ✅ | Golden fixture |
| `42["message",{"type":"license",…}]` | ✅ | ✅ | Handshake type `3`, `buildVersion` from assets |
| `auth` + `authChanges` | ✅ | ✅ | `openCmd` → download + x2t or PDF path |
| `documentOpen` ok/error | ✅ | ✅ | Cache file list |
| `isSaveLock` → `saveLock` | ✅ | ✅ | `saveLock:true` = blocked (flush running or lock held); `false` = proceed |
| `saveChanges` → `unSaveLock` | ✅ | ✅ | Changes appended; debounced flush |
| `forceSaveStart` → `forceSave` | ✅ | ✅ | `messages.inProgress:true` when a flush is already running; `forceSave` after x2t completes |
| `saveResult` (server → client) | — | ✅ | **go-office extension.** Broadcast to every session for a key when a flush settles: `{key, success, force, rolledBack, bridge, bytes, sequence, time, error}`. `sequence` increases per key so clients can discard out-of-order events. |
| Other coauthoring messages (cursor, chat, presence, …) | ✅ | ❌ | Ignored (POST returns `ok`, no reply) |
| Multi-user on same `key` | ✅ | ❌ | Single session per document key |

### Why `saveResult` exists

sdkjs clears its 60-second save-retry timer (`errorTimeOutSave`) when it receives `unSaveLock`. The server sends that reply as soon as changes are journalled — *before* the debounced x2t flush runs. Upstream clients therefore cannot tell a successful save from a failed or stalled one, and a failed save is silent: the editor shows no error and never retransmits.

`saveResult` closes that gap. It is emitted on **every** flush outcome, success or failure, and carries `rolledBack`/`bridge` so an integrator can distinguish a native save from an `assemblyFormatAsOrigin` rollback (OOXML bytes persisted at a legacy path such as `.xls`, `.doc`, `.ppt`). Clients that ignore unknown message types are unaffected.

---

## 3. Integrator contract (Docs API config + callback)

### 3.1 Editor configuration

| Field / API | ONLYOFFICE | go-office | Notes |
| ----------- | :--------: | :---------: | ----- |
| `DocsAPI.DocEditor(id, config)` | ✅ | ✅ | [Config](https://api.onlyoffice.com/docs/docs-api/usage-api/config/) |
| `document.key`, `.url`, `.fileType`, `.title` | ✅ | ✅ | `pkg/config.Build` |
| `document.permissions` (edit/download/print) | ✅ | ✅ | |
| `editorConfig.callbackUrl` | ✅ | ✅ | |
| `editorConfig.mode` (edit/view) | ✅ | ✅ | |
| `editorConfig.lang`, `customization`, `user` | ✅ | ✅ | Passed through when set by host |
| `editorConfig.coEditing` | ✅ | ⚠️ | Accepted; no multi-user semantics |
| `editorConfig.plugins`, `templates`, `embedded`, … | ✅ | ⚠️ | Passed if host supplies; not validated server-side |
| Config JWT (`token` top-level field) | ✅ | ✅ | Signed when `OFFICE_JWT_SECRET` or `JWT_SECRET` is set |
| Server-side JWT verify on coauthoring `auth` | ✅ | ✅ | When `JWTSecret` is set, `auth` packets must include a valid HS256 JWT |

### 3.2 Callback — integrator receives POSTs (Document Server → your app)

[Callback handler](https://api.onlyoffice.com/docs/docs-api/usage-api/callback-handler/) reference.

| Callback aspect | ONLYOFFICE Document Server | go-office |
| --------------- | :------------------------: | :-------: |
| POST to `editorConfig.callbackUrl` | ✅ | ✅ | 
| Response must be `{"error":0}` | ✅ | ✅ | `callback.WriteOK` |
| Status **2** (must save) | ✅ | ✅ | Outbound via `NotifyCallback`; inbound persist in `HandleCallback` |
| Status **6** (force saved) | ✅ | ✅ | Same |
| Status **1** (editing / user join) | ✅ | ❌ | **Not emitted** by go-office |
| Status **3** (save error) | ✅ | ⚠️ | Not sent to `callbackUrl`. Save failures are reported to the **editor** instead, via a `saveResult` coauthoring event (see §2) and `GET /api/office/demo/savestate`. Outbound callback failures do return an error to `HandleCallback`, which answers `{"error":1}`. |
| Status **4** (closed, no changes) | ✅ | ❌ | Not emitted |
| Status **7** (force save error) | ✅ | ⚠️ | Not sent to `callbackUrl`; surfaced to the editor as `saveResult` with `success:false` plus a `forceSave` packet with `success:false` |
| Payload fields: `users`, `actions` | ✅ | ⚠️ | Outbound includes `users` (document opener user id); `actions` not emitted |
| Payload fields: `changesurl`, `history`, `filetype` | ✅ | ⚠️ | Outbound includes `filetype`; `changesurl` / `history` not emitted |
| Payload fields: `forcesavetype`, `userdata` | ✅ | ⚠️ | Outbound includes `forcesavetype` on force save; `userdata` not emitted |
| Inbound: accept status 1/4 and return `error:0` | ✅ | ✅ | `HandleCallback` — no persist, OK response |
| Callback JWT `{"token":"…"}` (JWT_IN_BODY) | ✅ | ✅ | Sign outbound; verify inbound when secret set |
| Callback JWT in `Authorization` header only | ✅ | ✅ | `HandleCallback` uses `callback.ReadRequest` (Bearer or body token) |

**Important:** Integrators that only implement status **2** and **6** (typical save path) work. Integrators that track **status 1** “who is editing” or **status 4** “closed without save” from the document server will not receive those events from go-office.

### 3.3 JWT summary

| JWT usage | ONLYOFFICE | go-office |
| --------- | :--------: | :-------: |
| `OFFICE_JWT_SECRET` (preferred) / `JWT_SECRET` (fallback) | ✅ | ✅ |
| `OFFICE_JWT_ENABLED` / `JWT_ENABLED=false` | ✅ | ✅ |
| Sign editor config `token` | ✅ | ✅ |
| Verify config token on server | ✅ | ❌ |
| Callback body `{"token":"…"}` | ✅ | ✅ |
| Converter `Authorization: Bearer` | ✅ | ✅ | When JWT secret is set (`OFFICE_JWT_SECRET` or `JWT_SECRET`) |
| Command service `{"token":"…"}` | ✅ | ❌ (no `/command`) |

### 3.4 Environment variables

See [migration.md](migration.md) for the full matrix. Summary:

| Category | ONLYOFFICE | go-office |
| -------- | :--------: | :-------: |
| JWT secret | `JWT_SECRET` | `OFFICE_JWT_SECRET` (preferred) or `JWT_SECRET` |
| JWT disable | `JWT_ENABLED=false` | `OFFICE_JWT_ENABLED=false` or `JWT_ENABLED=false` |
| JWT header name | `JWT_HEADER` | ❌ not configurable (`Authorization` only) |
| JWT in callback body | `JWT_IN_BODY` | ⚠️ always signs body when secret set |
| Database / Redis / RabbitMQ | `DB_*`, `REDIS_*`, `AMQP_*` | ❌ not used |
| WOPI | `WOPI_ENABLED` | ❌ not supported |
| Listen port | Docker `-p host:80` | `OFFICE_ADDR` (default `:80` in image) |
| Public URL for cache links | nginx / proxy config | `OFFICE_PUBLIC_ORIGIN` |
| Debug logging | nginx / service logs | `OFFICE_DEBUG_LOGGING`, `OFFICE_LOG_JSON` |
| x2t concurrency | internal DS tuning | `OFFICE_CONVERT_LIMIT` (default `2`) |

---

## 4. FileBrowser integration checklist

| FileBrowser feature | ONLYOFFICE URL / API | go-office |
| ------------------- | -------------------- | --------- |
| In-browser editor | `{url}/web-apps/…/api.js` + config | ✅ |
| Config JWT (`integrations.onlyOffice.secret`) | Same as `OFFICE_JWT_SECRET` / `JWT_SECRET` | ✅ |
| **Grid preview thumbnails** | `POST {url}/converter` | **✅** |
| Document download URL in config | Your app’s download route | ✅ (host responsibility) |
| Callback save | Your app’s callback route | ✅ |

---

## 5. What “compatible” means (verdict)

### Compatible without changes

- Nextcloud / FileBrowser / custom app using **Docs API only**: load `api.js`, pass `document.key` / `document.url` / `callbackUrl`, handle callback status **2** and **6**, respond `{"error":0}`.
- Coauthoring over **polling** (production default when WebSocket unavailable).
- PDF open in editor via `downloadfile` + cache.
- FileBrowser **grid previews** via `POST /converter` with `outputtype: "jpg"`.

### Not compatible without workarounds

| Gap | Workaround |
| --- | ---------- |
| `async: true` on `/converter` | Use synchronous conversion only |
| `POST /command` | Use editor forcesave; accept no `info`/`drop` |
| WOPI integrators | Use Docs API instead |
| Spell checker service | Disable server spell-check; or proxy `/spellchecker/` elsewhere |
| Upstream Node.js test example app | `/example/` serves go-office's demo app (same as `/demo/`), not upstream's example |
| Multi-user co-editing | Single editor per `key` only |
| Callback status 1 / 4 telemetry | Track sessions in host app, not document server |
| WebSocket coauthoring | Polling only (usually automatic) |

---

## 6. Evidence

| Area | Tests / proof |
| ---- | ------------- |
| Coauthoring handshake, auth, save | `go test ./internal/ws/...`, `fixtures/coauthoring.json` |
| `saveResult` event (success, failure, sequencing) | `go test ./internal/ws/...` (`TestSaveOutcome*`) |
| Callback JWT | `go test ./pkg/callback/...`, `./pkg/office/save_test.go` |
| Editor open + content + save E2E | Playwright `open-formats`, `content`, `save`, `post-save-stability` |
| Conversion API | `go test ./pkg/office/...` (`TestHandleConverterJSON`), Playwright `thumbnails` |
| assemblyFormatAsOrigin rollback (xls/doc/ppt) | `go test ./internal/convert/...` (`TestSaveChangesXlsRollsBackToOOXML`) |
| ODS genuine conversion (no rollback) | `go test ./internal/convert/...` (`TestSaveChangesODSWithPendingChanges`) |
| No lost / duplicated saves under load | `go test ./pkg/office/...` (`TestPersistDoesNotLoseEditsUnderConcurrentAppends`, `TestPersistTwiceDoesNotDropSecondEdit`) |
| `/example/` alias | `go test ./internal/demo/...` (`TestExampleAliasServesDemoUI`, `TestExampleViewerUsesAliasBase`) |
| Fixture format integrity | `make check-sample-matrix` (OLE2 vs ZIP magic checks) |
| Command service | **No tests** — endpoint absent |
| WOPI / spellchecker | **No tests** — endpoints absent |

---

## 7. Related documentation

- [ONLYOFFICE Docs API](https://api.onlyoffice.com/docs/docs-api/)
- [How it works](https://api.onlyoffice.com/docs/docs-api/get-started/how-it-works/)
- [Conversion API](https://api.onlyoffice.com/docs/docs-api/additional-api/conversion-api/)
- [Command service](https://api.onlyoffice.com/docs/docs-api/additional-api/command-service/)
- [Callback handler](https://api.onlyoffice.com/docs/docs-api/usage-api/callback-handler/)
- [migration.md](migration.md) — Docker env mapping (`OFFICE_*` + ONLYOFFICE fallbacks)
- Live HTML matrix: `/docs/api#compatibility`

---

## 8. Audit changelog

| Date | Change |
| ---- | ------ |
| Phase 2 | Initial matrix; FileBrowser `/converter` gap |
| Follow-up audit | Added: legacy `.ashx` paths, WOPI, spellchecker, command subcommands, callback outbound field gaps, JWT matrix, `shardkey`, `downloadfile` partial, healthcheck semantics, coauthoring message gaps, integrator vs document-server callback direction |
| v0.2.0 | `/converter` + `/ConvertService.ashx` implemented (sync JPG); JWT on converter; demo thumbnails; library `DiscoverAssets` / `FetchAssets` / `EnsureAssets` |
| 2026-09 | `POST /session/reset`; `BuildEditorConfig` clears coauthoring session; demo warm; `JWT_SECRET` env fallback; reload/session tests |
| 2026-09 | `/example/` alias of the demo app; `saveResult` coauthoring event + `/api/office/demo/savestate` for server-driven save verification (replaces client disk polling); unified `assemblyFormatAsOrigin` rollback incl. reporting; save-error rows moved ❌ → ⚠️; fixture format-integrity checks |
