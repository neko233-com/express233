#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA="${RUNNER_TEMP:-/tmp}/express233-smoke-$$"
ADDR="127.0.0.1:39233"
BASE="${EXPRESS233_SMOKE_BASE_URL:-http://${ADDR}}"
OWN_SERVER=1
if [[ -n "${EXPRESS233_SMOKE_BASE_URL:-}" ]]; then
  OWN_SERVER=0
fi

if [[ "$OWN_SERVER" = 1 ]]; then
  mkdir -p "$DATA"
  "$ROOT/bin/express233-server" -addr ":39233" -data "$DATA" &
  PID=$!
  trap 'kill $PID 2>/dev/null || true' EXIT
fi

for _ in $(seq 1 50); do
  curl -fsS "$BASE/" >/dev/null 2>&1 && break
  sleep 0.2
done

COOKIE_JAR="$(mktemp)"
if [[ "$OWN_SERVER" = 1 ]]; then
  trap 'kill $PID 2>/dev/null || true; rm -f "$COOKIE_JAR"' EXIT
else
  trap 'rm -f "$COOKIE_JAR"' EXIT
fi

curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE/api/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"root","password":"root"}' >/dev/null

curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE/api/projects" \
  -H 'Content-Type: application/json' \
  -d '{"name":"smoke"}' >/dev/null

PID_NUM=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE/api/projects" | python3 -c "
import json,sys
for p in json.load(sys.stdin):
    if p.get('name')=='smoke':
        print(p['id']); break
")
[ -n "$PID_NUM" ]

curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE/api/projects/${PID_NUM}/versions" \
  -H 'Content-Type: application/json' \
  -d '{"name":"1.0.0"}' >/dev/null

BUNDLE_DIR="$(mktemp -d)"
mkdir -p "$BUNDLE_DIR/conf/app" "$BUNDLE_DIR/scripts" "$BUNDLE_DIR/bin"
printf 'port=1\n' > "$BUNDLE_DIR/game.properties"
printf 'service:\n  database:\n    host: db.internal\n' > "$BUNDLE_DIR/conf/app/application.yaml"
printf '#!/bin/sh\necho restarted\n' > "$BUNDLE_DIR/scripts/restart.sh"
printf 'Write-Output restarted\n' > "$BUNDLE_DIR/scripts/restart.ps1"
printf '\177ELFexpress233-smoke\n' > "$BUNDLE_DIR/bin/game-server"
tar -C "$BUNDLE_DIR" -czf "$BUNDLE_DIR/bundle.tar.gz" game.properties conf scripts bin
curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE/api/projects/${PID_NUM}/versions/1.0.0/files" \
  -F "file=@$BUNDLE_DIR/bundle.tar.gz;filename=bundle.tar.gz" >/dev/null

python3 - <<'PY' | curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X PUT "$BASE/api/server-yaml" \
  -H 'Content-Type: application/json' \
  -d @-
import json
print(json.dumps({"content": """servers:
  s1:
    replacements:
      game.properties:
        port: "9001"
    post_hook: restart.sh
"""}))
PY

curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  "$BASE/api/deploy/preview?project=smoke&version=1.0.0&server_id=s1" | grep -q '"after":"9001"'

curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE/api/projects/${PID_NUM}/versions/1.0.0/publish" >/dev/null

TOKEN=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE/api/users" | python3 -c "
import json,sys
print(json.load(sys.stdin)[0]['token'])
")
[ -n "$TOKEN" ]

curl -fsS -o /tmp/smoke.tgz "$BASE/api/pull?token=${TOKEN}&project=smoke&server_id=s1"
[ -s /tmp/smoke.tgz ]
tar -tzf /tmp/smoke.tgz | grep -qx 'conf/app/application.yaml'
tar -tzf /tmp/smoke.tgz | grep -qx 'scripts/restart.sh'
tar -tzf /tmp/smoke.tgz | grep -qx 'scripts/restart.ps1'
tar -tzf /tmp/smoke.tgz | grep -qx 'bin/game-server'
rm -rf "$BUNDLE_DIR"

echo "ci-smoke OK"
