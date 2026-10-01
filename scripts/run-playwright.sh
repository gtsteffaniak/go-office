#!/usr/bin/env bash
# Start go-office, pre-warm samples, run Playwright (CI and local).
set -euo pipefail

ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$ROOT"

SERVER_LOG="${PLAYWRIGHT_SERVER_LOG:-$ROOT/server.log}"
PID=""

stop_server() {
  if [[ -z "${PID:-}" ]]; then
    return 0
  fi
  if kill -0 "$PID" 2>/dev/null; then
    kill -TERM "$PID" 2>/dev/null || true
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$PID" 2>/dev/null || break
      sleep 0.2
    done
    kill -KILL "$PID" 2>/dev/null || true
  fi
  wait "$PID" 2>/dev/null || true
  PID=""
}

cleanup() {
  stop_server
}
trap cleanup EXIT INT TERM

healthcheck() {
  node -e "
    fetch('http://127.0.0.1:8080/healthcheck')
      .then((res) => res.text().then((body) => {
        process.exit(res.ok && body.trim() === 'true' ? 0 : 1);
      }))
      .catch(() => process.exit(1));
  "
}

cpu_count() {
  if command -v nproc >/dev/null 2>&1; then
    nproc
  elif [[ "$(uname -s)" == "Darwin" ]]; then
    sysctl -n hw.ncpu 2>/dev/null || echo 4
  else
    echo 4
  fi
}

NCORES="$(cpu_count)"
PARALLEL=$((NCORES / 2))
if [[ "$PARALLEL" -lt 2 ]]; then
  PARALLEL=2
fi
: "${PLAYWRIGHT_WORKERS:=${PARALLEL}}"
: "${OFFICE_CONVERT_LIMIT:=${PARALLEL}}"
: "${PLAYWRIGHT_PREWARM_DEADLINE_SEC:=60}"
: "${OFFICE_ASSETS:=${ROOT}/assets}"
: "${PLAYWRIGHT_BASE_URL:=http://127.0.0.1:8080}"
export PLAYWRIGHT_WORKERS OFFICE_CONVERT_LIMIT OFFICE_ASSETS PLAYWRIGHT_BASE_URL
export OFFICE_DEBUG_LOGGING="${OFFICE_DEBUG_LOGGING:-1}"
export OFFICE_POLL_HOLD="${OFFICE_POLL_HOLD:-0}"
export OFFICE_SAVE_DELAY="${OFFICE_SAVE_DELAY:-500ms}"
export PLAYWRIGHT_SAVE_DONE_TIMEOUT="${PLAYWRIGHT_SAVE_DONE_TIMEOUT:-30000}"
export PLAYWRIGHT_SAVE_TEST_TIMEOUT="${PLAYWRIGHT_SAVE_TEST_TIMEOUT:-150000}"
export PLAYWRIGHT_RTF_TEST_TIMEOUT="${PLAYWRIGHT_RTF_TEST_TIMEOUT:-240000}"
export PLAYWRIGHT_WARM_TIMEOUT="${PLAYWRIGHT_WARM_TIMEOUT:-60000}"
export PLAYWRIGHT_WARM_REQUEST_MS="${PLAYWRIGHT_WARM_REQUEST_MS:-30000}"
export PLAYWRIGHT_PREWARM_DEADLINE_MS="${PLAYWRIGHT_PREWARM_DEADLINE_MS:-60000}"
export PLAYWRIGHT_PREWARM_REQUEST_MS="${PLAYWRIGHT_PREWARM_REQUEST_MS:-20000}"
export PLAYWRIGHT_ACTION_TIMEOUT="${PLAYWRIGHT_ACTION_TIMEOUT:-45000}"
export PLAYWRIGHT_EDITOR_TIMEOUT="${PLAYWRIGHT_EDITOR_TIMEOUT:-45000}"
export PLAYWRIGHT_STRICT="${PLAYWRIGHT_STRICT:-1}"
export PLAYWRIGHT_FAIL_FAST_SAVE="${PLAYWRIGHT_FAIL_FAST_SAVE:-1}"

if [[ ! -x "${ROOT}/bin/go-office" ]]; then
  echo "error: missing ${ROOT}/bin/go-office (run: GOOS=linux go build -o bin/go-office ./cmd/go-office)" >&2
  exit 1
fi

MEM_KB=""
if [[ -r /proc/meminfo ]]; then
  MEM_KB="$(awk '/MemTotal:/ {print $2}' /proc/meminfo)"
fi

{
  echo "playwright env: arch=$(uname -m) nproc=${NCORES} workers=${PLAYWRIGHT_WORKERS} convert_limit=${OFFICE_CONVERT_LIMIT} mem_kb=${MEM_KB:-unknown}"
  echo "playwright env: node=$(node -v) playwright=$(cd frontend && npx playwright --version 2>/dev/null | head -1)"
  echo "playwright env: base_image=${PLAYWRIGHT_BASE_IMAGE:-not-set} repo=${ROOT}"
} >&2

: >"$SERVER_LOG"
# Do not pipe through tee in the background: $! would be tee, go-office stays a job, and bash
# can hang on exit waiting for the server after Playwright has already finished.
if command -v stdbuf >/dev/null 2>&1; then
  stdbuf -oL -eL "${ROOT}/bin/go-office" \
    -debug \
    -assets "$OFFICE_ASSETS" \
    -addr :8080 \
    -data "$ROOT" \
    -samples sample-files \
    -public "$PLAYWRIGHT_BASE_URL" \
    >>"$SERVER_LOG" 2>&1 &
else
  "${ROOT}/bin/go-office" \
    -debug \
    -assets "$OFFICE_ASSETS" \
    -addr :8080 \
    -data "$ROOT" \
    -samples sample-files \
    -public "$PLAYWRIGHT_BASE_URL" \
    >>"$SERVER_LOG" 2>&1 &
fi
PID=$!

deadline=$(( $(date +%s) + 120 ))
attempt=0
until healthcheck; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "go-office exited before health check (attempt $attempt)" >&2
    tail -80 "$SERVER_LOG" >&2 || true
    exit 1
  fi
  if [[ "$(date +%s)" -ge "$deadline" ]]; then
    echo "health check timed out after ${attempt} attempts" >&2
    tail -80 "$SERVER_LOG" >&2 || true
    exit 1
  fi
  attempt=$((attempt + 1))
  if [[ "$attempt" = 1 ]] || [[ $((attempt % 10)) -eq 0 ]]; then
    echo "waiting for healthcheck (attempt $attempt)…" >&2
  fi
  sleep 1
done
echo "go-office healthy after ${attempt} attempt(s)" >&2

WARM_SCRIPT="${ROOT}/_docker/warm-playwright-samples.mjs"
if [[ -f "$WARM_SCRIPT" ]]; then
  PREWARM_THUMBS=1
  case "${PLAYWRIGHT_PROJECT:-}" in
    chromium-save|chromium-post-save) PREWARM_THUMBS=0 ;;
  esac
  PLAYWRIGHT_SAMPLES_DIR="${ROOT}/sample-files" \
    PLAYWRIGHT_PREWARM_THUMBNAILS="$PREWARM_THUMBS" \
    PLAYWRIGHT_PREWARM_DEADLINE_MS="${PLAYWRIGHT_PREWARM_DEADLINE_MS:-60000}" \
    timeout "$PLAYWRIGHT_PREWARM_DEADLINE_SEC" node "$WARM_SCRIPT" >&2 || {
    echo "sample pre-warm failed or exceeded ${PLAYWRIGHT_PREWARM_DEADLINE_SEC}s deadline" >&2
    exit 1
  }
fi

cd "${ROOT}/frontend"
echo "starting playwright project=${PLAYWRIGHT_PROJECT:-all} workers=${PLAYWRIGHT_WORKERS}" >&2

set +e
if [[ -n "${PLAYWRIGHT_PROJECT:-}" ]]; then
  npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps
else
  npx playwright test
fi
rc=$?
set -e

if [[ "$rc" -ne 0 ]]; then
  echo "=== tail of server.log (last 200 lines) ===" >&2
  tail -200 "$SERVER_LOG" >&2 || true
fi
echo "playwright exited rc=$rc, stopping go-office" >&2
stop_server
exit "$rc"
