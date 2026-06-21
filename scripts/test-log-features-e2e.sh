#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:3000}"
ES_MOCK_PORT="${ES_MOCK_PORT:-19200}"
CH_MOCK_PORT="${CH_MOCK_PORT:-18123}"
COOKIE_JAR="$(mktemp)"
TMP_DIR="$(mktemp -d)"
ADMIN_USER_ID=""
MOCK_PID=""
trap 'kill "$MOCK_PID" 2>/dev/null || true; rm -f "$COOKIE_JAR"; rm -rf "$TMP_DIR"' EXIT

log() { echo "[e2e] $*"; }
fail() { echo "[e2e] FAIL: $*" >&2; exit 1; }

auth_curl() {
  curl -sS -b "$COOKIE_JAR" -H "New-Api-User: $ADMIN_USER_ID" "$@"
}

update_option() {
  local key="$1"
  local value="$2"
  local payload
  if [[ "$value" == "true" || "$value" == "false" ]]; then
    payload="{\"key\":\"$key\",\"value\":$value}"
  else
    payload="{\"key\":\"$key\",\"value\":\"$value\"}"
  fi
  local resp
  resp="$(auth_curl -X PUT "$BASE_URL/api/option/" -H 'Content-Type: application/json' -d "$payload")"
  echo "$resp" | grep -q '"success":true' || fail "option update failed for $key: $resp"
}

start_mock_backends() {
  python3 - <<PY >"$TMP_DIR/mock-server.log" 2>&1 &
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

ES_PORT = int("${ES_MOCK_PORT}")
CH_PORT = int("${CH_MOCK_PORT}")
EXTERNAL_TRACE = "22222222-2222-2222-2222-222222222222"

class ESHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        if self.path in ("/", ""):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"ok":true}')
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b""
        auth = self.headers.get("Authorization", "")
        if self.path == "/_bulk":
            if auth != "ApiKey test-es-api-key":
                self.send_response(401)
                self.end_headers()
                return
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"errors":false,"items":[{"index":{"status":201}}]}')
            return
        if self.path.endswith("/_search"):
            payload = {
                "hits": {
                    "hits": [{
                        "_source": {
                            "event_type": "session_turn",
                            "trace_id": EXTERNAL_TRACE,
                            "turn_index": 1,
                            "request_id": "req-ext-1",
                            "user_id": 1,
                            "token_id": 1,
                            "model_name": "gpt-4",
                            "channel_id": 1,
                            "status": "success",
                            "prompt_tokens": 12,
                            "completion_tokens": 6,
                            "is_stream": False,
                            "created_at": 1700000000,
                            "client_request": "{\"messages\":[{\"role\":\"user\",\"content\":\"external-hello\"}]}",
                            "assistant_response": "{\"choices\":[{\"message\":{\"content\":\"external-hi\"}}]}"
                        }
                    }]
                }
            }
            self.send_response(200)
            self.end_headers()
            self.wfile.write(json.dumps(payload).encode())
            return
        self.send_response(404)
        self.end_headers()

class CHHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query).get("query", [""])[0].lower()
        if "select 1" in query.replace("+", " "):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"1\n")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        _ = self.rfile.read(length) if length else b""
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query).get("query", [""])[0].lower()
        if "create table" in query:
            self.send_response(200)
            self.end_headers()
            return
        if "insert into" in query:
            self.send_response(200)
            self.end_headers()
            return
        if "where trace_id" in query:
            self.send_response(200)
            self.end_headers()
            return
        self.send_response(200)
        self.end_headers()

ThreadingHTTPServer(("127.0.0.1", ES_PORT), ESHandler).serve_forever()
PY
  MOCK_PID=$!
  python3 - <<PY >>"$TMP_DIR/mock-server.log" 2>&1 &
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

CH_PORT = int("${CH_MOCK_PORT}")

class CHHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_GET(self):
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query).get("query", [""])[0].lower()
        if "select 1" in query.replace("+", " "):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"1\n")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        _ = self.rfile.read(length) if length else b""
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query).get("query", [""])[0].lower()
        self.send_response(200)
        self.end_headers()

ThreadingHTTPServer(("127.0.0.1", CH_PORT), CHHandler).serve_forever()
PY
  MOCK_PID="$MOCK_PID $!"
  for _ in $(seq 1 20); do
    curl -sf "http://127.0.0.1:${ES_MOCK_PORT}/" >/dev/null 2>&1 && \
    curl -sf "http://127.0.0.1:${CH_MOCK_PORT}/?query=SELECT+1" >/dev/null 2>&1 && break
    sleep 0.2
  done
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

log "3. Seed consume, error, and clusterable error logs"
python3 - <<PY
import sqlite3, json, time
now = int(time.time())
conn = sqlite3.connect("${SQLITE_PATH:-/tmp/new-api-e2e.db}")
cur = conn.cursor()
cur.execute("SELECT id FROM users WHERE username='admin' LIMIT 1")
user_id = cur.fetchone()[0]
logs = [
  (user_id, 'admin', 2, now - 600, 1, 'gpt-4', 10, 5, 1, 'key-a', 'default', '', '{"trace_id":"11111111-1111-1111-1111-111111111111"}'),
  (user_id, 'admin', 2, now - 500, 1, 'gpt-4', 20, 10, 1, 'key-a', 'default', '', '{"trace_id":"11111111-1111-1111-1111-111111111111"}'),
  (user_id, 'admin', 5, now - 400, 1, 'gpt-4', 0, 0, 1, 'key-a', 'default', 'upstream error with trace', '{"trace_id":"11111111-1111-1111-1111-111111111111","error_detail_id":"test-detail"}'),
  (user_id, 'admin', 2, now - 300, 2, 'claude-3', 100, 50, 2, 'key-b', 'vip', '', '{}'),
  (user_id, 'admin', 5, now - 280, 1, 'gpt-4', 0, 0, 1, 'key-a', 'default', 'upstream timeout after 120 seconds', '{}'),
  (user_id, 'admin', 5, now - 270, 1, 'gpt-4', 0, 0, 1, 'key-a', 'default', 'upstream timeout after 240 seconds', '{}'),
  (user_id, 'admin', 5, now - 260, 1, 'gpt-4', 0, 0, 1, 'key-a', 'default', 'upstream timeout after 360 seconds', '{}'),
]
cur.executemany(
  """INSERT INTO logs (user_id, username, type, created_at, channel_id, model_name, prompt_tokens, completion_tokens, token_id, token_name, "group", content, other)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
  logs,
)
conn.commit()
conn.close()
print('seeded logs')
PY

log "4. Test log analytics with insights and error clustering"
analytics_resp="$(auth_curl "$BASE_URL/api/log/analytics?start_timestamp=$start&end_timestamp=$now&group_by=channel")"
echo "$analytics_resp" | grep -q '"success":true' || fail "analytics failed: $analytics_resp"
echo "$analytics_resp" | grep -q '"insights"' || fail "analytics missing insights"
python3 - <<'PY' "$analytics_resp"
import json, sys
data = json.loads(sys.argv[1])["data"]
insights = data.get("insights") or {}
for key in ("time_series", "heatmap", "errors", "flow_links"):
    if key not in insights:
        raise SystemExit(f"missing insights.{key}")
errors = insights["errors"]
if not errors:
    raise SystemExit("expected error clusters")
cluster = next((item for item in errors if "upstream timeout" in item.get("message", "")), None)
if cluster is None:
    raise SystemExit(f"expected upstream timeout cluster, got: {errors}")
if cluster.get("count", 0) < 3:
    raise SystemExit(f"expected clustered count >= 3, got {cluster}")
print("error clustering ok")
PY

log "5. Test log export status (disabled by default)"
export_status="$(auth_curl "$BASE_URL/api/log/export/status")"
echo "$export_status" | grep -q '"success":true' || fail "export status failed: $export_status"

log "6. Start mock Elasticsearch and ClickHouse backends"
start_mock_backends

log "7. Configure log export integration"
update_option "log_export_setting.enabled" "true"
update_option "log_export_setting.elasticsearch_enabled" "true"
update_option "log_export_setting.elasticsearch_url" "http://127.0.0.1:${ES_MOCK_PORT}"
update_option "log_export_setting.elasticsearch_index" "new-api-logs"
update_option "log_export_setting.elasticsearch_api_key" "test-es-api-key"
update_option "log_export_setting.clickhouse_enabled" "true"
update_option "log_export_setting.clickhouse_url" "http://127.0.0.1:${CH_MOCK_PORT}"
update_option "log_export_setting.clickhouse_database" "default"
update_option "log_export_setting.clickhouse_table" "new_api_log_events"

log "8. Verify elasticsearch_api_key persisted"
python3 - <<PY
import sqlite3
conn = sqlite3.connect("${SQLITE_PATH:-/tmp/new-api-e2e.db}")
cur = conn.cursor()
cur.execute("SELECT value FROM options WHERE key='log_export_setting.elasticsearch_api_key'")
row = cur.fetchone()
if not row or row[0] != "test-es-api-key":
    raise SystemExit(f"elasticsearch_api_key not persisted: {row}")
print("elasticsearch_api_key persisted")
PY

log "9. Test log export connections"
export_test="$(auth_curl -X POST "$BASE_URL/api/log/export/test")"
echo "$export_test" | grep -q '"success":true' || fail "export test failed: $export_test"
python3 - <<'PY' "$export_test"
import json, sys
data = json.loads(sys.argv[1])["data"]
for backend in ("elasticsearch", "clickhouse"):
    info = data.get(backend) or {}
    if not info.get("configured"):
        raise SystemExit(f"{backend} not configured")
    if not info.get("healthy"):
        raise SystemExit(f"{backend} not healthy: {info}")
print("export connections ok")
PY

export_status="$(auth_curl "$BASE_URL/api/log/export/status")"
echo "$export_status" | grep -q '"enabled":true' || fail "export status not enabled: $export_status"

log "10. Seed local session trace"
trace_id="11111111-1111-1111-1111-111111111111"
trace_dir="${SESSION_TRACE_DIR:-./data/session-traces}/$trace_id"
mkdir -p "$trace_dir"
cat > "$trace_dir/1.json" <<'JSON'
{"trace_id":"11111111-1111-1111-1111-111111111111","turn_index":1,"request_id":"req-1","client_request":"{\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}","assistant_response":"{\"choices\":[{\"message\":{\"content\":\"hi\"}}]}","is_stream":false,"truncated":false}
JSON
python3 - <<PY
import sqlite3, time
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

log "11. Test local session trace query"
trace_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$trace_id")"
echo "$trace_resp" | grep -q '"success":true' || fail "session trace failed: $trace_resp"
echo "$trace_resp" | grep -q '"turns"' || fail "session trace missing turns"
echo "$trace_resp" | grep -q '"data_source":"local"' || fail "session trace missing local data_source"

log "12. Test session trace turn download (local)"
turn_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$trace_id/turn/1")"
echo "$turn_resp" | grep -q 'hello' || fail "turn download missing request payload: $turn_resp"

log "13. Test external session trace lookup via Elasticsearch"
update_option "log_export_setting.prefer_external_for_trace_query" "true"
external_trace_id="22222222-2222-2222-2222-222222222222"
external_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$external_trace_id")"
echo "$external_resp" | grep -q '"success":true' || fail "external session trace failed: $external_resp"
echo "$external_resp" | grep -q '"data_source":"elasticsearch"' || fail "external session trace missing elasticsearch data_source: $external_resp"
echo "$external_resp" | grep -q 'external-hello' || fail "external session trace missing exported payload: $external_resp"

log "14. Test external session trace turn download fallback"
external_turn="$(auth_curl "$BASE_URL/api/log/session-trace/$external_trace_id/turn/1")"
echo "$external_turn" | grep -q 'external-hello' || fail "external turn download failed: $external_turn"

log "15. Test analytics 90-day guard"
old_start=$((now - 91*86400))
bad_analytics="$(auth_curl "$BASE_URL/api/log/analytics?start_timestamp=$old_start&end_timestamp=$now&group_by=channel")"
echo "$bad_analytics" | grep -q '"success":false' || fail "expected 90-day guard failure: $bad_analytics"

log "All E2E checks passed"
