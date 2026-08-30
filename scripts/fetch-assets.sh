#!/usr/bin/env bash
# Linux only: download Euro-Office assets into ./assets/
set -euo pipefail
if [[ "$(uname -s)" != "Linux" ]]; then
  echo "fetch-assets.sh requires Linux (use WSL or CI on other hosts)" >&2
  exit 1
fi
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
exec go run ./cmd/fetch-assets "$@"
