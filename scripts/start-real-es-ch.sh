#!/usr/bin/env bash
# Start real Elasticsearch 7.x and ClickHouse for integration testing (no Docker required).
set -euo pipefail

ROOT="/workspace/.integration"
ES_DIR="$ROOT/es/elasticsearch-7.17.24"
CH_BIN="$ROOT/ch/clickhouse"
ES_PORT="${ES_PORT:-9200}"
CH_PORT="${CH_PORT:-8123}"

mkdir -p "$ROOT/data/clickhouse/tmp" "$ROOT/data/elasticsearch" "$ROOT/logs"

start_clickhouse() {
  if curl -sf "http://127.0.0.1:${CH_PORT}/?query=SELECT+1" >/dev/null 2>&1; then
    echo "[real-es-ch] ClickHouse already running on ${CH_PORT}"
    return
  fi
  "$CH_BIN" server \
    --config-file="$ROOT/ch/config.xml" \
    --pid-file="$ROOT/logs/clickhouse.pid" \
    --daemon
  for _ in $(seq 1 30); do
    curl -sf "http://127.0.0.1:${CH_PORT}/?query=SELECT+1" >/dev/null 2>&1 && break
    sleep 0.5
  done
  curl -sf "http://127.0.0.1:${CH_PORT}/?query=SELECT+1" >/dev/null || {
    echo "[real-es-ch] ClickHouse failed to start" >&2
    exit 1
  }
  echo "[real-es-ch] ClickHouse ready on ${CH_PORT}"
}

start_elasticsearch() {
  if curl -sf "http://127.0.0.1:${ES_PORT}/" >/dev/null 2>&1; then
    echo "[real-es-ch] Elasticsearch already running on ${ES_PORT}"
    return
  fi
  cd "$ES_DIR"
  ES_JAVA_OPTS="${ES_JAVA_OPTS:--Xms512m -Xmx512m}" ./bin/elasticsearch \
    -E discovery.type=single-node \
    -E network.host=127.0.0.1 \
    -E http.port="$ES_PORT" \
    -E path.data="$ROOT/data/elasticsearch" \
    -E path.logs="$ROOT/logs/elasticsearch" \
    >>"$ROOT/logs/elasticsearch.stdout.log" 2>&1 &
  echo $! >"$ROOT/logs/elasticsearch.pid"
  for _ in $(seq 1 60); do
    curl -sf "http://127.0.0.1:${ES_PORT}/" >/dev/null 2>&1 && break
    sleep 1
  done
  curl -sf "http://127.0.0.1:${ES_PORT}/" >/dev/null || {
    echo "[real-es-ch] Elasticsearch failed to start" >&2
    tail -20 "$ROOT/logs/elasticsearch.stdout.log" >&2 || true
    exit 1
  }
  echo "[real-es-ch] Elasticsearch ready on ${ES_PORT}"
}

start_clickhouse
start_elasticsearch
