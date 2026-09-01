#!/bin/bash
set -euo pipefail
cd /home/graham/git/go-office

git checkout -b stability-p0-p1-demo-fixes 2>/dev/null || git checkout stability-p0-p1-demo-fixes

git add -A

# Unstage any .env files if accidentally staged
if git diff --cached --name-only | grep -qE '\.env'; then
  git diff --cached --name-only | grep -E '\.env' | xargs -r git reset HEAD --
fi

git commit -m "$(cat <<'EOF'
Stabilize P0/P1 paths and fix demo viewer origin handling.

P0/P1: rewrite AllFonts at fetch-assets and remove symlink hack, add concurrent
AllFonts isolation test, converter Drain for in-flight x2t jobs, coauthoring open
drain on Stop, and hard-fail CSV cell-edit assertion.

Demo: same-origin relative URLs in viewer, service worker unregister, and
request-origin for config/callback URLs via netutil.RequestOrigin.
EOF
)"

git push -u origin HEAD

echo "===BRANCH==="
git branch --show-current
echo "===HASH==="
git rev-parse HEAD
echo "===STATUS==="
git status -sb
