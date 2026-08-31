#!/usr/bin/env bash
# Record coauthoring polling packets from a running Euro-Office / go-office instance.
# Usage: ./scripts/record-ws-fixtures.sh http://127.0.0.1:8080 doc-key
set -euo pipefail
BASE="${1:-http://127.0.0.1:8080}"
KEY="${2:-fixture-key}"
URL="${BASE%/}/doc/${KEY}/c/?EIO=4&transport=polling"

echo "Open: GET ${URL}"
curl -sf "${URL}" | tee /tmp/ws-open.txt
echo

SID=$(grep -o '"sid":"[^"]*"' /tmp/ws-open.txt | head -1 | cut -d'"' -f4)
echo "Connect: POST sid=${SID}"
curl -sf -X POST "${URL}&sid=${SID}" -d '40' | tee /tmp/ws-connect.txt
echo

echo "Poll:"
curl -sf "${URL}&sid=${SID}&t=1" | tee /tmp/ws-poll.txt
echo
echo "Saved snippets under /tmp/ws-*.txt — merge into internal/ws/fixtures/coauthoring.json"
