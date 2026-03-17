#!/bin/bash
set -e

IMAGE_NAME="new-api-custom"

echo "==> 构建 Docker 镜像: $IMAGE_NAME"
docker build -t $IMAGE_NAME .

echo "==> 启动服务"
docker compose -f docker-compose.sqlite.yml up -d

echo "==> 服务已启动，访问 http://localhost:49831"
