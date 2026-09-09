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

cd /app
./go-office \
  -assets /app/assets \
  -addr :8080 \
  -data /app \
  -samples sample-files \
  -public http://127.0.0.1:8080 \
  >"$LOG" 2>&1 &
PID=$!

deadline=$(( $(date +%s) + 60 ))
until curl -sf http://127.0.0.1:8080/healthcheck >/dev/null 2>&1; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "go-office exited before health check" >&2
    cat "$LOG" >&2 || true
    exit 1
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "health check timed out" >&2
    cat "$LOG" >&2 || true
    exit 1
  fi
  sleep 1
done

cd /app/frontend
if [ -n "${PLAYWRIGHT_PROJECT:-}" ]; then
  npx playwright test --project="$PLAYWRIGHT_PROJECT" --no-deps
else
  npx playwright test
fi
status=$?

if [ "$status" -ne 0 ]; then
  echo "=== go-office server log ===" >&2
  cat "$LOG" >&2 || true
fi
exit "$status"
