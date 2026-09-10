#!/bin/sh
set -eu

LOG=/tmp/go-office.log
PID=""

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

dump_log() {
  echo "=== go-office server log (errors and editor clientLog) ===" >&2
  if [ -f "$LOG" ]; then
    bytes=$(wc -c <"$LOG" | tr -d ' ')
    echo "(log file size: ${bytes} bytes)" >&2
    if [ "$bytes" -gt 0 ]; then
      grep -E 'clientLog|severity=error|severity=warn|level=ERROR|level=WARN|"level":"ERROR"|"level":"WARN"|document open|warm failed|warm convert|flush|save failed|saveChanges|forceSave|x2t|Editor\.bin' "$LOG" \
        | tail -300 >&2 || true
      echo "=== go-office server log (full tail) ===" >&2
      tail -500 "$LOG" >&2 || true
    else
      echo "(log file is empty — check stderr capture)" >&2
    fi
  else
    echo "(no log file at $LOG)" >&2
  fi
}

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

: >"$LOG"
# Line-buffer stderr so CI log dumps include recent server output.
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
  >>"$LOG" 2>&1 &
PID=$!

deadline=$(( $(date +%s) + 120 ))
attempt=0
until healthcheck; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "go-office exited before health check (attempt $attempt)" >&2
    dump_log
    exit 1
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "health check timed out after ${attempt} attempts" >&2
    dump_log
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
    dump_log
    exit 1
  }
fi

cd /app/frontend
status=0
if [ -n "${PLAYWRIGHT_PROJECT:-}" ]; then
  if ! npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps; then
    status=1
  fi
else
  if ! npx playwright test; then
    status=1
  fi
fi

if [ "$status" -ne 0 ]; then
  dump_log
fi
exit "$status"
