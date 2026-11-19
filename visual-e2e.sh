#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"
bash scripts/build-console.sh
if [[ ! -x test/browser/node_modules/.bin/agent-browser || test/browser/package-lock.json -nt test/browser/node_modules/.package-lock.json ]]; then
  npm --prefix test/browser ci --ignore-scripts
fi
exec npm --prefix test/browser run verify
