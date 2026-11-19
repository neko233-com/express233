#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [[ ! -x "$ROOT/frontend/node_modules/.bin/vp" || "$ROOT/frontend/package-lock.json" -nt "$ROOT/frontend/node_modules/.package-lock.json" ]]; then
  npm --prefix "$ROOT/frontend" ci
fi
npm --prefix "$ROOT/frontend" run build
