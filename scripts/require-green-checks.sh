#!/usr/bin/env bash
# 发布前校验：指定 commit 上 CI + Lint 的 GitHub Check 均为 success。
set -euo pipefail

SHA="${1:-${GITHUB_SHA:-}}"
REPO="${GITHUB_REPOSITORY:-}"

if [[ -z "$SHA" ]]; then
  echo "usage: require-green-checks.sh <commit-sha>" >&2
  exit 2
fi

if [[ -z "$REPO" ]]; then
  origin="$(git remote get-url origin 2>/dev/null || true)"
  if [[ "$origin" =~ github\.com[:/]([^/]+/[^/.]+) ]]; then
    REPO="${BASH_REMATCH[1]%.git}"
  else
    echo "GITHUB_REPOSITORY or git origin required" >&2
    exit 2
  fi
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh CLI not found; install: https://cli.github.com/ or run install-gh.cmd" >&2
  exit 2
fi

REQUIRED=(
  "golangci-lint"
  "test (ubuntu-latest)"
  "test (windows-latest)"
  "test (macos-latest)"
  "build binaries"
  "validate scripts"
)

mapfile -t rows < <(
  gh api "/repos/${REPO}/commits/${SHA}/check-runs?per_page=100" \
    --jq '.check_runs[] | [.name, .status, .conclusion] | @tsv'
)

missing=0
failed=0

for want in "${REQUIRED[@]}"; do
  ok=0
  for row in "${rows[@]}"; do
    IFS=$'\t' read -r name status conclusion <<<"$row"
    [[ "$name" == "$want" ]] || continue
    ok=1
    if [[ "$status" != "completed" || "$conclusion" != "success" ]]; then
      echo "FAIL: $name (status=$status conclusion=${conclusion:-none})"
      failed=1
    else
      echo "OK:   $name"
    fi
    break
  done
  if [[ "$ok" -eq 0 ]]; then
    echo "MISSING: $want (no check run on ${SHA:0:7})"
    missing=1
  fi
done

if [[ "$missing" -eq 1 || "$failed" -eq 1 ]]; then
  echo ""
  echo "Release blocked: push to main and wait for CI + Lint on this commit before tagging."
  echo "See: https://github.com/${REPO}/commit/${SHA}/checks"
  exit 1
fi

echo "All required checks passed for ${SHA:0:7}."
