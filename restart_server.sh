#!/bin/bash
cd "$(dirname "$0")"

# 构建新镜像
docker build --platform linux/amd64 -t new-api-lwb .

# 停止并删除旧容器
docker stop new-api-lwb 2>/dev/null || true
docker rm new-api-lwb 2>/dev/null || true

# 启动新容器
docker run --name new-api-lwb -d --restart always \
  -p 127.0.0.1:33292:3000 \
  -e TZ=Asia/Shanghai \
  -e ERROR_LOG_ENABLED=true \
  -e BATCH_UPDATE_ENABLED=true \
  -v ./data:/data \
  -v ./logs:/app/logs \
  new-api-lwb --log-dir /app/logs
