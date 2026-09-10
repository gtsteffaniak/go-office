#!/bin/sh
set -eu

PID=""

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

healthcheck() {
  # playwright-base has Node but not curl/wget.
  node -e "
    fetch('http://127.0.0.1:8080/healthcheck')
      .then((res) => res.text().then((body) => {
        process.exit(res.ok && body.trim() === 'true' ? 0 : 1);
      }))
      .catch(() => process.exit(1));
  "
}

cd /app
: "${PLAYWRIGHT_WORKERS:=10}"
: "${OFFICE_CONVERT_LIMIT:=4}"
export PLAYWRIGHT_WORKERS OFFICE_CONVERT_LIMIT

# Stream go-office logs to CI output as they happen (line-buffered when stdbuf exists).
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
  PLAYWRIGHT_BASE_URL="http://127.0.0.1:8080" \
    PLAYWRIGHT_SAMPLES_DIR="/app/sample-files" \
    node /app/_docker/warm-playwright-samples.mjs >&2 || {
    echo "sample pre-warm failed" >&2
    exit 1
  }
fi

cd /app/frontend
if [ -n "${PLAYWRIGHT_PROJECT:-}" ]; then
  exec npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps
else
  exec npx playwright test
fi
