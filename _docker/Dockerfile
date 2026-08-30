# office-server image: go-office + Euro-Office assets + sample-files.
#
#   make build-docker-image   # build only
#   make build-docker         # build image and run (default)
#
# Published as ghcr.io/quantumx-apps/office-server (see .github/workflows/publish.yml).
#
# Stages:
#   gobuild  — Alpine + Go: static go-office + fetch-assets binaries
#   assets   — Debian slim: unpack Euro-Office .deb (x2t needs glibc + dpkg)
#   runtime  — Debian slim: x2t is a prebuilt glibc binary; Alpine/musl is not supported
#
# Reuses ./assets from the host when present (after make build); otherwise
# fetches Euro-Office assets during the image build.

FROM golang:1.25-alpine AS gobuild
RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY internal ./internal
COPY scripts/euro-office.version ./scripts/euro-office.version

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/go-office ./cmd/go-office \
 && go build -trimpath -ldflags="-s -w" -o /out/fetch-assets ./cmd/fetch-assets

FROM debian:bookworm-slim AS assets
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates dpkg \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod ./go.mod
COPY scripts/euro-office.version ./scripts/euro-office.version
COPY --from=gobuild /out/fetch-assets /usr/local/bin/fetch-assets

# Optional host-built assets (populated by make build or CI cache; .gitkeep when empty).
COPY assets /assets-cache/

RUN set -eux; \
    if [ -f /assets-cache/sdkjs/common/AllFonts.js ] || [ -f /assets-cache/web-apps/apps/api/documents/api.js.tpl ]; then \
      echo "Using host assets from build context"; \
      cp -a /assets-cache /assets; \
    else \
      echo "Fetching Euro-Office assets inside Docker (first build may take several minutes)"; \
      fetch-assets -out /assets; \
    fi; \
    chmod +x /assets/converter/bin/x2t /assets/converter/bin/docbuilder 2>/dev/null || true; \
    test -x /assets/converter/bin/x2t

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*

LABEL org.opencontainers.image.source="https://github.com/quantumx-apps/go-office" \
      org.opencontainers.image.licenses="AGPL-3.0" \
      org.opencontainers.image.title="office-server"

WORKDIR /app

COPY --from=gobuild /out/go-office ./go-office
COPY --from=assets /assets ./assets
COPY sample-files ./sample-files

RUN set -eux; \
    chmod +x ./go-office ./assets/converter/bin/x2t ./assets/converter/bin/docbuilder; \
    cd ./assets/converter/bin; \
    out=$(/bin/sh -c 'LD_LIBRARY_PATH=. ./x2t 2>&1' || true); \
    printf '%s\n' "$out" | head -3 || true; \
    ! printf '%s' "$out" | grep -qiE 'error (loading|relocating)|symbol not found'

ENV OFFICE_ASSETS=/app/assets \
    OFFICE_DATA_DIR=/app \
    OFFICE_ADDR=:80

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
  CMD curl -sf http://127.0.0.1:80/healthcheck || exit 1

ENTRYPOINT ["./go-office"]
CMD ["-assets", "/app/assets", "-addr", ":80", "-data", "/app", "-samples", "sample-files", "-public", "http://localhost"]
