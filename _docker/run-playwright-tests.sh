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
  echo "=== go-office server log ===" >&2
  if [ -f "$LOG" ]; then
    cat "$LOG" >&2 || true
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
: >"$LOG"
./go-office \
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

cd /app/frontend
if [ -n "${PLAYWRIGHT_PROJECT:-}" ]; then
  npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps
else
  npx playwright test
fi
status=$?

if [ "$status" -ne 0 ]; then
  dump_log
fi
exit "$status"
