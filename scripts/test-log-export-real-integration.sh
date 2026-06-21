#!/usr/bin/env bash
# Real Elasticsearch + ClickHouse integration test with relay export write verification.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:3000}"
MOCK_UPSTREAM_PORT="${MOCK_UPSTREAM_PORT:-18080}"
ES_URL="${ES_URL:-http://127.0.0.1:9200}"
CH_URL="${CH_URL:-http://127.0.0.1:8123}"
ES_INDEX="${ES_INDEX:-new-api-logs}"
CH_DB="${CH_DB:-default}"
CH_TABLE="${CH_TABLE:-new_api_log_events}"
COOKIE_JAR="$(mktemp)"
TMP_DIR="$(mktemp -d)"
ADMIN_USER_ID=""
TRACE_ID="33333333-3333-3333-3333-333333333333"
MOCK_PID=""
trap 'kill $MOCK_PID 2>/dev/null || true; rm -f "$COOKIE_JAR"; rm -rf "$TMP_DIR"' EXIT

log() { echo "[real-e2e] $*"; }
fail() { echo "[real-e2e] FAIL: $*" >&2; exit 1; }

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

wait_for_export() {
  local attempt="$1"
  python3 - <<PY "$attempt"
import json, sys, urllib.request

attempt = sys.argv[1]
trace_id = "${TRACE_ID}"
es_url = "${ES_URL}/${ES_INDEX}/_search"
ch_url = "${CH_URL}/?query=SELECT+count()+FROM+${CH_DB}.${CH_TABLE}+WHERE+trace_id='${TRACE_ID}'+FORMAT+JSON"

es_query = json.dumps({"query": {"term": {"trace_id.keyword": trace_id}}, "size": 10}).encode()
es_req = urllib.request.Request(es_url, data=es_query, headers={"Content-Type": "application/json"})
es_hits = 0
try:
    with urllib.request.urlopen(es_req, timeout=5) as resp:
        es_hits = len(json.load(resp).get("hits", {}).get("hits", []))
except Exception as exc:
    print(f"es error: {exc}", file=sys.stderr)

ch_count = 0
try:
    with urllib.request.urlopen(ch_url, timeout=5) as resp:
        payload = json.load(resp)
        ch_count = int(payload["data"][0]["count()"])
except Exception as exc:
    print(f"ch error: {exc}", file=sys.stderr)

print(f"attempt={attempt} es_hits={es_hits} ch_count={ch_count}")
if es_hits >= 1 and ch_count >= 1:
    sys.exit(0)
sys.exit(1)
PY
}

log "0. Ensure real ES/CH are running"
/workspace/scripts/start-real-es-ch.sh >/dev/null

log "1. Start mock OpenAI upstream"
python3 - <<PY >"$TMP_DIR/mock-upstream.log" 2>&1 &
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int("${MOCK_UPSTREAM_PORT}")

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b""
        payload = {
            "id": "chatcmpl-test",
            "object": "chat.completion",
            "choices": [{
                "index": 0,
                "message": {"role": "assistant", "content": "integration-ok"},
                "finish_reason": "stop",
            }],
            "usage": {"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10},
        }
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(payload).encode())

ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
PY
MOCK_PID=$!
for _ in $(seq 1 20); do curl -sf -X POST "http://127.0.0.1:${MOCK_UPSTREAM_PORT}/v1/chat/completions" -H 'Content-Type: application/json' -d '{}' >/dev/null 2>&1 && break; sleep 0.2; done

log "2. Setup admin"
setup_resp="$(curl -sS -X POST "$BASE_URL/api/setup" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456","confirmPassword":"admin123456","SelfUseModeEnabled":false,"DemoSiteEnabled":false}')"
echo "$setup_resp" | grep -Eq '"success":true|"系统已经初始化完成"' || fail "setup failed: $setup_resp"

login_resp="$(curl -sS -c "$COOKIE_JAR" -X POST "$BASE_URL/api/user/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456"}')"
echo "$login_resp" | grep -q '"success":true' || fail "login failed: $login_resp"
ADMIN_USER_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' <<<"$login_resp")"

log "3. Configure log export to real ES/CH"
update_option "log_export_setting.enabled" "true"
update_option "log_export_setting.export_consume_logs" "true"
update_option "log_export_setting.export_error_logs" "true"
update_option "log_export_setting.export_session_turns" "true"
update_option "log_export_setting.prefer_external_for_trace_query" "true"
update_option "log_export_setting.elasticsearch_enabled" "true"
update_option "log_export_setting.elasticsearch_url" "$ES_URL"
update_option "log_export_setting.elasticsearch_index" "$ES_INDEX"
update_option "log_export_setting.clickhouse_enabled" "true"
update_option "log_export_setting.clickhouse_url" "$CH_URL"
update_option "log_export_setting.clickhouse_database" "$CH_DB"
update_option "log_export_setting.clickhouse_table" "$CH_TABLE"

export_test="$(auth_curl -X POST "$BASE_URL/api/log/export/test")"
echo "$export_test" | grep -q '"success":true' || fail "export test failed: $export_test"
python3 - <<'PY' "$export_test"
import json, sys
data = json.loads(sys.argv[1])["data"]
for backend in ("elasticsearch", "clickhouse"):
    info = data.get(backend) or {}
    if not info.get("healthy"):
        raise SystemExit(f"{backend} unhealthy: {info}")
print("real export connections ok")
PY

log "4. Create mock OpenAI channel and API token"
channel_resp="$(auth_curl -X POST "$BASE_URL/api/channel/" -H 'Content-Type: application/json' -d "{
  \"mode\": \"single\",
  \"channel\": {
    \"type\": 1,
    \"key\": \"sk-integration-test\",
    \"name\": \"integration-mock-openai\",
    \"base_url\": \"http://127.0.0.1:${MOCK_UPSTREAM_PORT}\",
    \"models\": \"gpt-4o-mini\",
    \"group\": \"default\",
    \"status\": 1
  }
}")"
echo "$channel_resp" | grep -q '"success":true' || fail "create channel failed: $channel_resp"

auth_curl -X POST "$BASE_URL/api/channel/fix" >/dev/null || true

token_resp="$(auth_curl -X POST "$BASE_URL/api/token/" -H 'Content-Type: application/json' -d '{
  "name": "integration-export-token",
  "expired_time": -1,
  "remain_quota": 500000000,
  "unlimited_quota": true
}')"
echo "$token_resp" | grep -q '"success":true' || fail "create token failed: $token_resp"
token_search_resp="$(auth_curl "$BASE_URL/api/token/search?keyword=integration-export-token")"
echo "$token_search_resp" | grep -q '"success":true' || fail "search token failed: $token_search_resp"
TOKEN_ID="$(python3 - <<'PY' "$token_search_resp"
import json, sys
payload = json.loads(sys.argv[1])
items = payload.get("data", {}).get("items") or payload.get("data") or []
if isinstance(items, dict) and "items" in items:
    items = items["items"]
if not items:
    raise SystemExit("token not found after creation")
print(items[0]["id"])
PY
)"
token_key_resp="$(auth_curl -X POST "$BASE_URL/api/token/$TOKEN_ID/key")"
echo "$token_key_resp" | grep -q '"success":true' || fail "get token key failed: $token_key_resp"
API_KEY="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["key"])' <<<"$token_key_resp")"

log "5. Relay chat completion with trace id (triggers consume + session turn export)"
relay_resp="$(curl -sS -X POST "$BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Trace-Id: $TRACE_ID" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"integration relay export test"}]}')"
echo "$relay_resp" | grep -q 'integration-ok' || fail "relay failed: $relay_resp"

log "6. Wait for async export into real ES and ClickHouse"
exported=0
for attempt in $(seq 1 20); do
  if wait_for_export "$attempt"; then
    exported=1
    break
  fi
  sleep 1
done
[[ "$exported" -eq 1 ]] || fail "export not found in real ES/CH after relay request"

log "7. Verify session trace via external store query"
trace_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$TRACE_ID")"
echo "$trace_resp" | grep -q '"success":true' || fail "external session trace query failed: $trace_resp"
echo "$trace_resp" | grep -Eq '"data_source":"(elasticsearch|clickhouse)"' || fail "expected external data_source: $trace_resp"
echo "$trace_resp" | grep -q 'integration relay export test' || fail "external trace missing request payload: $trace_resp"

turn_resp="$(auth_curl "$BASE_URL/api/log/session-trace/$TRACE_ID/turn/1")"
echo "$turn_resp" | grep -q 'integration-ok' || fail "external turn download failed: $turn_resp"

log "All real ES/CH integration checks passed"
