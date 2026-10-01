#!/bin/sh
# Deprecated: use scripts/run-playwright.sh (kept for older docs).
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export REPO_ROOT="$ROOT"
exec "$ROOT/scripts/run-playwright.sh"
