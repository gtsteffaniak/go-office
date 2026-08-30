#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$root/assets/converter/bin"
if [[ ! -f "$bin/x2t" ]]; then
  echo "error: $bin/x2t not found (run: make build)" >&2
  exit 1
fi
chmod +x "$bin"/*
ls -la "$bin/x2t"
LD_LIBRARY_PATH="$bin" "$bin/x2t" | head -3
echo "ok: x2t is executable"
