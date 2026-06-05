#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
"$ROOT/deploy-github/deploy-github.sh" "$@"
if [[ "${1:-}" == "--push" ]]; then
  echo "== git push =="
  git push -u origin HEAD
fi
