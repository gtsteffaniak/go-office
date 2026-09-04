# office-server image: go-office + Euro-Office assets + sample-files.
#
#   make build-docker-image   # build only
#   make build-docker         # same as build-docker-image
#   make run-docker           # run after build
#
# Published as ghcr.io/quantumx-apps/office-server (see .github/workflows/main.yaml and tag.yaml).
#
# Stages:
#   gobuild  — Go: static go-office + fetch-assets binaries (built per target platform)
#   assets   — Debian slim: fetch Euro-Office .deb for the build platform (x2t needs glibc + dpkg)
#   runtime  — Debian slim: x2t is a prebuilt glibc binary; Alpine/musl is not supported
#
# Euro-Office assets are fetched in the assets stage when missing (fetch-assets skips
# re-download when .extracted matches the pinned version). Docker layer cache keeps
# this fast across rebuilds. For local dev, use make serve (host/volume-backed assets).
#
# Build args:
#   SKIP_ASSET_FETCH=false  Set true for CI smoke builds (stub assets, no .deb download).

ARG SKIP_ASSET_FETCH=false

FROM golang:1.27-bookworm AS gobuild
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
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
ARG SKIP_ASSET_FETCH
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates dpkg \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod ./go.mod
COPY scripts/euro-office.version ./scripts/euro-office.version
COPY --from=gobuild /out/fetch-assets /usr/local/bin/fetch-assets

RUN set -eux; \
    if [ "$SKIP_ASSET_FETCH" = "true" ]; then \
      echo "Skipping Euro-Office asset fetch (SKIP_ASSET_FETCH=true)"; \
      mkdir -p /assets/converter/bin /assets/sdkjs/common /assets/web-apps/apps/api/documents; \
      printf '%s\n' 'var _fonts = {};' > /assets/sdkjs/common/AllFonts.js; \
      printf '\0' > /assets/converter/bin/font_selection.bin; \
    else \
      fetch-assets -out /assets; \
      chmod +x /assets/converter/bin/x2t /assets/converter/bin/docbuilder 2>/dev/null || true; \
      test -x /assets/converter/bin/x2t; \
    fi

FROM debian:bookworm-slim
ARG SKIP_ASSET_FETCH
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
    chmod +x ./go-office; \
    if [ "$SKIP_ASSET_FETCH" != "true" ]; then \
      chmod +x ./assets/converter/bin/x2t ./assets/converter/bin/docbuilder; \
      cd ./assets/converter/bin; \
      out=$(/bin/sh -c 'LD_LIBRARY_PATH=. ./x2t 2>&1' || true); \
      printf '%s\n' "$out" | head -3 || true; \
      ! printf '%s' "$out" | grep -qiE 'error (loading|relocating)|symbol not found'; \
    fi

ENV OFFICE_ASSETS=/app/assets \
    OFFICE_DATA_DIR=/app \
    OFFICE_ADDR=:80

ARG OFFICE_DEBUG_LOGGING=
ENV OFFICE_DEBUG_LOGGING=${OFFICE_DEBUG_LOGGING}

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
  CMD curl -sf http://127.0.0.1:80/healthcheck || exit 1

ENTRYPOINT ["./go-office"]
CMD ["-assets", "/app/assets", "-addr", ":80", "-data", "/app", "-samples", "sample-files", "-public", "http://localhost"]
