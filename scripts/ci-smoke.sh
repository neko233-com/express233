#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA="${RUNNER_TEMP:-/tmp}/express233-smoke-$$"
ADDR="127.0.0.1:39233"
BASE="http://${ADDR}"

mkdir -p "$DATA"
"$ROOT/bin/express233-server" -addr ":39233" -data "$DATA" &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT

for _ in $(seq 1 50); do
  curl -fsS "$BASE/" >/dev/null 2>&1 && break
  sleep 0.2
done

COOKIE_JAR="$(mktemp)"
trap 'kill $PID 2>/dev/null || true; rm -f "$COOKIE_JAR"' EXIT

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

printf 'port=1\n' | curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST \
  "$BASE/api/projects/${PID_NUM}/versions/1.0.0/files" \
  -F "file=@-;filename=game.properties" >/dev/null

cat > "$DATA/server.yaml" <<'EOF'
servers:
  s1:
    replacements:
      game.properties:
        port: "9001"
    post_hook: restart.sh
EOF

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

echo "ci-smoke OK"
