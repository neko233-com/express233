#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT/test/visual"
if [[ ! -d node_modules ]]; then
  echo "== npm install =="
  npm install
  npx playwright install chromium
fi
echo "== playwright visual e2e (not run on publish / git-deploy) =="
exec npx playwright test "$@"
