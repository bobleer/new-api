#!/bin/bash
set -e

echo "==> 检查依赖"
if ! command -v bun &> /dev/null; then
    echo "安装 bun..."
    brew install oven-sh/bun/bun
fi

if ! command -v go &> /dev/null; then
    echo "安装 go..."
    brew install go
fi

echo "==> 构建前端"
cd web
bun install
bun run build
cd ..

echo "==> 构建后端"
go build -o new-api

echo "==> 启动服务（使用 SQLite）"
mkdir -p data logs
./new-api --log-dir ./logs --port 49831

echo "==> 服务已启动，访问 http://localhost:49831"
