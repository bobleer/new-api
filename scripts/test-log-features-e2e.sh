#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:3000}"
COOKIE_JAR="$(mktemp)"
TMP_DIR="$(mktemp -d)"
ADMIN_USER_ID=""
trap 'rm -f "$COOKIE_JAR"; rm -rf "$TMP_DIR"' EXIT

log() { echo "[e2e] $*"; }
fail() { echo "[e2e] FAIL: $*" >&2; exit 1; }

auth_curl() {
  curl -sS -b "$COOKIE_JAR" -H "New-Api-User: $ADMIN_USER_ID" "$@"
}

json_get() {
  local expr="$1"
  python3 -c 'import json,sys; data=json.load(sys.stdin); expr=sys.argv[1]; print(eval(expr, {"data": data}))' "$expr"
}

log "1. Setup initial admin (skip if already initialized)"
setup_resp="$(curl -sS -X POST "$BASE_URL/api/setup" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456","confirmPassword":"admin123456","SelfUseModeEnabled":false,"DemoSiteEnabled":false}')"
echo "$setup_resp" | grep -Eq '"success":true|"系统已经初始化完成"' || fail "setup failed: $setup_resp"

log "2. Login as admin"
login_resp="$(curl -sS -c "$COOKIE_JAR" -X POST "$BASE_URL/api/user/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456"}')"
echo "$login_resp" | grep -q '"success":true' || fail "login failed: $login_resp"
ADMIN_USER_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' <<<"$login_resp")"

now="$(date +%s)"
start=$((now - 3600))

log "3. Seed consume and error logs"
python3 - <<PY
import sqlite3, json, time
now = int(time.time())
conn = sqlite3.connect("${SQLITE_PATH:-/tmp/new-api-e2e.db}")
cur = conn.cursor()
cur.execute("SELECT id FROM users WHERE username='admin' LIMIT 1")
user_id = cur.fetchone()[0]
logs = [
  (user_id, 'admin', 2, now - 600, 1, 'gpt-4', 10, 5, 1, 'key-a', 'default', '{"trace_id":"11111111-1111-1111-1111-111111111111"}'),
  (user_id, 'admin', 2, now - 500, 1, 'gpt-4', 20, 10, 1, 'key-a', 'default', '{"trace_id":"11111111-1111-1111-1111-111111111111"}'),
  (user_id, 'admin', 5, now - 400, 1, 'gpt-4', 0, 0, 1, 'key-a', 'default', '{"trace_id":"11111111-1111-1111-1111-111111111111","error_detail_id":"test-detail"}'),
  (user_id, 'admin', 2, now - 300, 2, 'claude-3', 100, 50, 2, 'key-b', 'vip', '{}'),
]
cur.executemany(
  """INSERT INTO logs (user_id, username, type, created_at, channel_id, model_name, prompt_tokens, completion_tokens, token_id, token_name, "group", other)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
  logs,
)
conn.commit()
conn.close()
print('seeded logs')
PY

log "4. Test log analytics with insights"
analytics_resp="$(auth_curl "$BASE_URL/api/log/analytics?start_timestamp=$start&end_timestamp=$now&group_by=channel")"
echo "$analytics_resp" | grep -q '"success":true' || fail "analytics failed: $analytics_resp"
echo "$analytics_resp" | grep -q '"insights"' || fail "analytics missing insights"
echo "$analytics_resp" | grep -q '"time_series"' || fail "analytics missing time_series"
echo "$analytics_resp" | grep -q '"heatmap"' || fail "analytics missing heatmap"
echo "$analytics_resp" | grep -q '"errors"' || fail "analytics missing errors"
echo "$analytics_resp" | grep -q '"flow_links"' || fail "analytics missing flow_links"

log "5. Test log export status"
export_status="$(auth_curl "$BASE_URL/api/log/export/status")"
echo "$export_status" | grep -q '"success":true' || fail "export status failed: $export_status"

log "6. Seed local session trace"
trace_id="11111111-1111-1111-1111-111111111111"
trace_dir="${SESSION_TRACE_DIR:-./data/session-traces}/$trace_id"
mkdir -p "$trace_dir"
cat > "$trace_dir/1.json" <<'JSON'
{"trace_id":"11111111-1111-1111-1111-111111111111","turn_index":1,"request_id":"req-1","client_request":"{\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}","assistant_response":"{\"choices\":[{\"message\":{\"content\":\"hi\"}}]}","is_stream":false,"truncated":false}
JSON
python3 - <<PY
import sqlite3, time, os
now = int(time.time())
conn = sqlite3.connect("${SQLITE_PATH:-/tmp/new-api-e2e.db}")
cur = conn.cursor()
cur.execute("SELECT id FROM users WHERE username='admin' LIMIT 1")
user_id = cur.fetchone()[0]
cur.execute("INSERT OR REPLACE INTO session_traces (trace_id, user_id, token_id, model_name, turn_count, created_at, last_activity_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
  ("$trace_id", user_id, 1, "gpt-4", 1, now - 600, now - 400))
cur.execute("INSERT OR REPLACE INTO session_trace_turns (trace_id, turn_index, request_id, user_id, token_id, model_name, channel_id, status, prompt_tokens, completion_tokens, is_stream, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
  ("$trace_id", 1, "req-1", user_id, 1, "gpt-4", 1, "success", 10, 5, 0, now - 500))
conn.commit()
conn.close()
print('seeded session trace metadata')
PY

log "7. Test session trace lookup"
trace_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$trace_id")"
echo "$trace_resp" | grep -q '"success":true' || fail "session trace failed: $trace_resp"
echo "$trace_resp" | grep -q '"turns"' || fail "session trace missing turns"
echo "$trace_resp" | grep -q '"data_source":"local"' || fail "session trace missing local data_source"

log "8. Test session trace turn download"
turn_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$trace_id/turn/1")"
echo "$turn_resp" | grep -q 'hello' || fail "turn download missing request payload: $turn_resp"

log "9. Test analytics 90-day guard"
old_start=$((now - 91*86400))
bad_analytics="$(auth_curl "$BASE_URL/api/log/analytics?start_timestamp=$old_start&end_timestamp=$now&group_by=channel")"
echo "$bad_analytics" | grep -q '"success":false' || fail "expected 90-day guard failure: $bad_analytics"

log "All E2E checks passed"
