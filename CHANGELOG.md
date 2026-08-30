# Changelog

All notable changes to **go-office** (the Go library and demo server in this repository) are documented here.

Euro-Office / ONLYOFFICE editor assets (`web-apps/`, `sdkjs/`, `x2t`) are **not** modified in this repository. They are fetched at build time from upstream releases (see `scripts/euro-office.version` and `THIRD_PARTY_NOTICES`). This project integrates and serves those components; it does not ship patched copies of them in git.

## [Unreleased]

### Licensing and compliance

- Clarified that users do **not** need to click or sign a EULA to download, host, or run go-office or its Docker images. Rights under [AGPL-3.0](LICENSE) are granted automatically.
- Documented AGPL compliance expectations (source availability, license text, modification notices) in README and NOTICE.
- Documented AGPL Section 7 branding obligations for ONLYOFFICE / Euro-Office editor UI (attribution must not be removed or replaced).
- Added OCI image labels (`org.opencontainers.image.source`, `org.opencontainers.image.licenses`) to published Dockerfiles.

## [0.1.0] - 2026-08-30

Initial public release: embedded Go document server, coauthoring polling handshake, x2t document open, demo UI, Playwright E2E, Docker demo image.

[Unreleased]: https://github.com/quantumx-apps/go-office/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/quantumx-apps/go-office/releases/tag/v0.1.0
