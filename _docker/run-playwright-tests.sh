#!/bin/sh
# Self-contained Playwright runner for the Dockerfile.playwright test image (/app layout).
set -eu

PID=""

stop_server() {
  if [ -z "${PID:-}" ]; then
    return 0
  fi
  if kill -0 "$PID" 2>/dev/null; then
    kill -TERM "$PID" 2>/dev/null || true
    sleep 1
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

cd /app

if command -v nproc >/dev/null 2>&1; then
  NCORES="$(nproc)"
else
  NCORES=4
fi
DEFAULT_PARALLEL=6
: "${PLAYWRIGHT_WORKERS:=${DEFAULT_PARALLEL}}"
: "${OFFICE_CONVERT_LIMIT:=${DEFAULT_PARALLEL}}"
: "${PLAYWRIGHT_PREWARM_DEADLINE_SEC:=60}"
export PLAYWRIGHT_WORKERS OFFICE_CONVERT_LIMIT

echo "playwright env: nproc=${NCORES} workers=${PLAYWRIGHT_WORKERS} convert_limit=${OFFICE_CONVERT_LIMIT}" >&2

if command -v stdbuf >/dev/null 2>&1; then
  GO_STDERR=stdbuf
  GO_STDERR_ARGS="-oL -eL"
else
  GO_STDERR=""
  GO_STDERR_ARGS=""
fi
$GO_STDERR $GO_STDERR_ARGS ./go-office \
  -debug \
  -assets /app/assets \
  -addr :8080 \
  -data /app \
  -samples sample-files \
  -public http://127.0.0.1:8080 \
  >&2 &
PID=$!

deadline=$(( $(date +%s) + 120 ))
attempt=0
until healthcheck; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "go-office exited before health check (attempt $attempt)" >&2
    exit 1
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "health check timed out after ${attempt} attempts" >&2
    exit 1
  fi
  attempt=$((attempt + 1))
  if [ "$attempt" = 1 ] || [ $((attempt % 10)) -eq 0 ]; then
    echo "waiting for healthcheck (attempt $attempt)…" >&2
  fi
  sleep 1
done
echo "go-office healthy after ${attempt} attempt(s)" >&2

if [ -f /app/_docker/warm-playwright-samples.mjs ]; then
  PREWARM_THUMBS=1
  case "${PLAYWRIGHT_PROJECT:-}" in
    chromium-save|chromium-post-save) PREWARM_THUMBS=0 ;;
  esac
  PLAYWRIGHT_BASE_URL="http://127.0.0.1:8080" \
    PLAYWRIGHT_SAMPLES_DIR="/app/sample-files" \
    PLAYWRIGHT_PREWARM_THUMBNAILS="$PREWARM_THUMBS" \
    PLAYWRIGHT_PREWARM_DEADLINE_MS="${PLAYWRIGHT_PREWARM_DEADLINE_MS:-60000}" \
    timeout "$PLAYWRIGHT_PREWARM_DEADLINE_SEC" node /app/_docker/warm-playwright-samples.mjs >&2 || {
    echo "sample pre-warm failed or exceeded ${PLAYWRIGHT_PREWARM_DEADLINE_SEC}s deadline" >&2
    exit 1
  }
fi

cd /app/frontend
if [ -n "${PLAYWRIGHT_PROJECT:-}" ]; then
  echo "starting playwright project=${PLAYWRIGHT_PROJECT} workers=${PLAYWRIGHT_WORKERS}" >&2
  exec npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps --workers="$PLAYWRIGHT_WORKERS"
fi
echo "starting playwright all projects workers=${PLAYWRIGHT_WORKERS}" >&2
exec npx playwright test --workers="$PLAYWRIGHT_WORKERS"
