#!/bin/bash
set -e

echo "==> 停止现有服务"
pkill -f "./new-api" || echo "没有运行中的服务"

echo "==> 重新构建前端"
cd web
bun run build
cd ..

echo "==> 重新构建后端"
go build -o new-api

echo "==> 启动服务"
mkdir -p logs
nohup ./new-api --log-dir ./logs --port 49831 > ./logs/server.log 2>&1 &
echo "PID: $!"
echo "==> 服务已重启，访问 http://localhost:49831"
echo "==> 日志: ./logs/server.log"
