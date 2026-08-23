#!/usr/bin/env bash
#
# ooblivion end-to-end test
#
# Starts an isolated instance on a dedicated port with a temporary database,
# exercises the full flow (login, capture of every HTTP method, headers, scopes,
# notifications, settings, flush, password change, logout, host gate, security
# headers) and prints a pass/fail report.
#
# Requires: go, curl, python3. Run from the repository root.
# Optional: E2E_PORT (default 8180), E2E_KEEP (set to keep temp files).

set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${E2E_PORT:-8180}"
BASE="http://127.0.0.1:${PORT}"
TMP="$(mktemp -d)"
DB="$TMP/e2e.db"
BIN="$TMP/ooblivion"
JAR="$TMP/cookies.txt"
LOG="$TMP/server.log"
ADMIN_PASSWORD='e2e-password-2026-strong!'
NEW_PASSWORD='e2e-new-password-2026-strong!'

PASS=0
FAIL=0
FAILED=()

cleanup() {
  [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null
  [ -z "${E2E_KEEP:-}" ] && rm -rf "$TMP"
}
trap cleanup EXIT

say()  { echo ""; echo "== $1 =="; }
check() {
  local name="$1"; shift
  if "$@" >/dev/null 2>&1; then
    PASS=$((PASS + 1)); echo "  [PASS] $name"
  else
    FAIL=$((FAIL + 1)); FAILED+=("$name"); echo "  [FAIL] $name"
  fi
}

# --- setup ---------------------------------------------------------------
say "setup"
command -v go >/dev/null || { echo "go not found"; exit 1; }
command -v curl >/dev/null || { echo "curl not found"; exit 1; }
command -v python3 >/dev/null || { echo "python3 not found"; exit 1; }

(cd "$ROOT" && go build -o "$BIN" ./cmd/ooblivion) || { echo "build failed"; exit 1; }

JWT_SECRET="$(python3 -c "import secrets; print(secrets.token_hex(72))")"
export OOB_LISTEN_ADDR="127.0.0.1:${PORT}"
export DATABASE_PATH="$DB"
export ADMIN_HOST="127.0.0.1"
export JWT_SECRET
export ADMIN_PASSWORD
export LOG_LEVEL="error"
"$BIN" >"$LOG" 2>&1 &
SERVER_PID=$!

ready=0
for _ in $(seq 1 60); do
  if curl -s -o /dev/null "$BASE/admin/login"; then ready=1; break; fi
  sleep 0.5
done
check "server starts on port ${PORT}" test "$ready" -eq 1

api() { curl -s -b "$JAR" "$BASE$1"; }
row_count() { # row_count <exact-path>
  api "/admin/api/requests?q=$1" | python3 -c "import sys,json; d=json.load(sys.stdin); print(sum(1 for i in d['items'] if i.get('Path')=='$1'))"
}

# --- unauthenticated access ----------------------------------------------
say "auth gate"
title=$(curl -s "$BASE/admin/login" | grep -o '<title>[^<]*' | head -1)
check "login page loads" test "$title" = "<title>Login - ooblivion"

code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/admin/settings")
check "unauthenticated /admin redirects to login" test "$code" = "302"

# --- login ---------------------------------------------------------------
say "login"
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/admin/api/login" \
  -H "Content-Type: application/json" -d "{\"password\":\"wrong-password\"}")
check "wrong password rejected" test "$code" = "401"

code=$(curl -s -o /dev/null -w "%{http_code}" -c "$JAR" -X POST "$BASE/admin/api/login" \
  -H "Content-Type: application/json" -d "{\"password\":\"$ADMIN_PASSWORD\"}")
check "correct password logs in" test "$code" = "200"

CSRF="$(python3 -c "import re; print(re.search(r'oob_csrf\s+(\S+)', open('$JAR').read()).group(1))")"

code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" "$BASE/admin/api/stats")
check "session cookie authenticated" test "$code" = "200"

# --- capture: every HTTP method ------------------------------------------
say "capture: HTTP methods"
for m in GET POST PUT PATCH DELETE HEAD OPTIONS TRACE CONNECT; do
  if [ "$m" = "HEAD" ]; then
    curl -s -I -o /dev/null "$BASE/e2e-$m"
  else
    curl -s -o /dev/null -X "$m" "$BASE/e2e-$m"
  fi
  n=$(row_count "/e2e-$m")
  check "captured $m request" test "$n" -ge 1
done

# --- capture: body and headers (verbatim) --------------------------------
say "capture: body and headers"
curl -s -o /dev/null -X POST "$BASE/e2e-body" \
  -H "Content-Type: application/json" -H "X-Custom: e2e-header-value" \
  -H "Cookie: e2e=abc123" -H "Authorization: Bearer e2etok" \
  -d '{"k":"v","n":1}'
api "/admin/api/requests?q=/e2e-body" > "$TMP/body.json"
check "request body captured" python3 -c "
import json,sys
d=json.load(open('$TMP/body.json'))
item=[i for i in d['items'] if i.get('Path')=='/e2e-body'][0]
sys.exit(0 if item.get('Body')=='{\"k\":\"v\",\"n\":1}' else 1)"
check "cookie header stored verbatim" python3 -c "
import json,sys
d=json.load(open('$TMP/body.json'))
item=[i for i in d['items'] if i.get('Path')=='/e2e-body'][0]
sys.exit(0 if 'e2e=abc123' in item.get('RequestHeaders','') else 1)"
check "authorization header stored verbatim" python3 -c "
import json,sys
d=json.load(open('$TMP/body.json'))
item=[i for i in d['items'] if i.get('Path')=='/e2e-body'][0]
sys.exit(0 if 'Bearer e2etok' in item.get('RequestHeaders','') else 1)"
check "custom header stored" python3 -c "
import json,sys
d=json.load(open('$TMP/body.json'))
item=[i for i in d['items'] if i.get('Path')=='/e2e-body'][0]
sys.exit(0 if 'e2e-header-value' in item.get('RequestHeaders','') else 1)"

curl -s -o /dev/null "$BASE/e2e-xff" -H "X-Forwarded-For: 203.0.113.42"
api "/admin/api/requests?q=/e2e-xff" > "$TMP/xff.json"
check "X-Forwarded-For resolves source IP" python3 -c "
import json,sys
d=json.load(open('$TMP/xff.json'))
item=[i for i in d['items'] if i.get('Path')=='/e2e-xff'][0]
sys.exit(0 if item.get('SourceIP')=='203.0.113.42' else 1)"

# --- CSRF enforcement ----------------------------------------------------
say "CSRF"
ID=$(api "/admin/api/requests?q=/e2e-GET" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print([i for i in d['items'] if i.get('Path')=='/e2e-GET'][0]['ID'])")
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -X POST "$BASE/admin/api/requests/$ID/save")
check "mutation without CSRF token rejected" test "$code" = "403"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" -X POST "$BASE/admin/api/requests/$ID/save")
check "mutation with CSRF token accepted" test "$code" = "200"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" -X POST "$BASE/admin/api/requests/$ID/unsave")
check "unsave works" test "$code" = "200"

# --- scopes + auto-save --------------------------------------------------
say "scopes"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -H "Content-Type: application/json" -X POST "$BASE/admin/api/scopes" \
  -d '{"name":"e2e-saved","match_on":"query","match_type":"contains","pattern":"e2e-save-me","enabled":true,"priority":10}')
check "create scope" test "$code" = "200"

curl -s -o /dev/null "$BASE/e2e-scoped?e2e-save-me=1"
saved=$(api "/admin/api/requests?saved=1" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(sum(1 for i in d['items'] if i.get('Path')=='/e2e-scoped'))")
check "scope auto-saves matching request" test "$saved" -ge 1

# --- notifications -------------------------------------------------------
say "notifications"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -H "Content-Type: application/json" -X POST "$BASE/admin/api/notifications" \
  -d '{"name":"e2e-alert","match_on":"path","match_type":"contains","pattern":"e2e-alert","enabled":true}')
check "create notification rule" test "$code" = "200"

RID=$(api "/admin/api/notifications" | python3 -c "
import sys,json
print(json.load(sys.stdin)[0]['ID'])")
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -X POST "$BASE/admin/api/notifications/$RID/test")
check "test notification without chat id rejected" test "$code" = "400"

# --- settings ------------------------------------------------------------
say "settings"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -H "Content-Type: application/json" -X PUT "$BASE/admin/api/settings" \
  -d '{"public_url":"https://e2e.example","retention_days":45,"auto_flush_enabled":true}')
check "update settings" test "$code" = "200"

ret=$(api "/admin/api/settings" | python3 -c "import sys,json; print(json.load(sys.stdin)['retention_days'])")
check "settings persisted" test "$ret" = "45"

code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" "$BASE/admin/api/stats")
check "dashboard stats endpoint" test "$code" = "200"

# --- flush ---------------------------------------------------------------
say "flush"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -X POST "$BASE/admin/api/flush")
check "manual flush accepted" test "$code" = "200"
after=$(api "/admin/api/requests?saved=1" | python3 -c "
import sys,json
print(sum(1 for i in json.load(sys.stdin)['items'] if i.get('Path')=='/e2e-scoped'))")
check "saved data survives flush" test "$after" -ge 1
unsaved=$(api "/admin/api/requests?q=/e2e-GET" | python3 -c "
import sys,json
print(sum(1 for i in json.load(sys.stdin)['items'] if i.get('Path')=='/e2e-GET' and not i.get('Saved')))")
check "unsaved data removed by flush" test "$unsaved" = "0"

# --- password change + revocation ----------------------------------------
say "password change"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -H "Content-Type: application/json" -X POST "$BASE/admin/api/password" \
  -d "{\"current\":\"$ADMIN_PASSWORD\",\"new\":\"$NEW_PASSWORD\"}")
check "change password" test "$code" = "200"

code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" "$BASE/admin/api/stats")
check "old token revoked after password change" test "$code" = "401"

curl -s -o /dev/null -c "$JAR" -X POST "$BASE/admin/api/login" \
  -H "Content-Type: application/json" -d "{\"password\":\"$NEW_PASSWORD\"}"
CSRF="$(python3 -c "import re; print(re.search(r'oob_csrf\s+(\S+)', open('$JAR').read()).group(1))")"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" "$BASE/admin/api/stats")
check "new password logs in" test "$code" = "200"

# --- host gate -----------------------------------------------------------
say "host gate"
code=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: other.acme.corp" "$BASE/admin/login")
check "non-admin host /admin returns 200 empty" test "$code" = "200"
captured=$(api "/admin/api/requests?q=/admin/login" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(sum(1 for i in d['items'] if i.get('Path')=='/admin/login' and i.get('Host')=='other.acme.corp'))")
check "non-admin host /admin captured in log" test "$captured" -ge 1

# --- security headers ----------------------------------------------------
say "security headers"
hdr=$(curl -s -D - -o /dev/null "$BASE/admin/login")
check "X-Content-Type-Options nosniff" bash -c "echo '$hdr' | grep -qi '^X-Content-Type-Options: nosniff'"
check "X-Frame-Options DENY" bash -c "echo '$hdr' | grep -qi '^X-Frame-Options: DENY'"
check "Content-Security-Policy present" bash -c "echo '$hdr' | grep -qi '^Content-Security-Policy:'"

# --- logout --------------------------------------------------------------
say "logout"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" -H "X-CSRF-Token: $CSRF" \
  -X POST "$BASE/admin/api/logout")
check "logout succeeds" test "$code" = "200"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$JAR" "$BASE/admin/api/stats")
check "token destroyed after logout" test "$code" = "401"

# --- report --------------------------------------------------------------
total=$((PASS + FAIL))
pct=0
if [ "$total" -gt 0 ]; then pct=$((PASS * 100 / total)); fi
dur=$SECONDS

echo ""
echo "=================================================="
echo "  ooblivion E2E TEST REPORT"
echo "=================================================="
echo "  Base URL    : $BASE"
echo "  Duration    : ${dur}s"
echo "  Total tests : $total"
echo "  Passed      : $PASS"
echo "  Failed      : $FAIL"
echo "  Pass rate   : ${pct}%"
if [ "$FAIL" -gt 0 ]; then
  echo "  Failed tests:"
  for t in "${FAILED[@]}"; do echo "    - $t"; done
fi
echo "  Server log  : $LOG"
echo "=================================================="

[ "$FAIL" -eq 0 ] || exit 1
