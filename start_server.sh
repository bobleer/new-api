#!/bin/bash
cd "$(dirname "$0")"
docker run --name new-api-lwb -d --restart always \
  -p 127.0.0.1:33292:3000 \
  -e TZ=Asia/Shanghai \
  -e ERROR_LOG_ENABLED=true \
  -e BATCH_UPDATE_ENABLED=true \
  -v ./data:/data \
  -v ./logs:/app/logs \
  new-api-lwb --log-dir /app/logs
