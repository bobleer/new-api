#!/bin/bash
set -e

# 配置变量
IMAGE_NAME="new-api-custom"
CONTAINER_NAME="new-api"
PORT="33292"
DATA_DIR="./data"
LOG_DIR="./logs"

echo "==> 检查 Docker 是否安装"
if ! command -v docker &> /dev/null; then
    echo "错误: Docker 未安装，请先安装 Docker"
    exit 1
fi

echo "==> 构建 Docker 镜像"
docker build --platform linux/amd64 -t $IMAGE_NAME .

echo "==> 创建数据目录"
mkdir -p $DATA_DIR $LOG_DIR

echo "==> 停止并删除旧容器（如果存在）"
docker stop $CONTAINER_NAME 2>/dev/null || true
docker rm $CONTAINER_NAME 2>/dev/null || true

echo "==> 启动容器"
docker run --name $CONTAINER_NAME -d --restart always \
  -p 127.0.0.1:$PORT:3000 \
  -e TZ=Asia/Shanghai \
  -e ERROR_LOG_ENABLED=true \
  -e BATCH_UPDATE_ENABLED=true \
  -v $(pwd)/$DATA_DIR:/data \
  -v $(pwd)/$LOG_DIR:/app/logs \
  $IMAGE_NAME --log-dir /app/logs

echo "✓ 容器已配置 --restart always，Docker 启动时会自动恢复容器"

echo ""
echo "==> 部署完成！"
echo "容器名称: $CONTAINER_NAME"
echo "访问地址: http://localhost:$PORT"
echo "数据目录: $(pwd)/$DATA_DIR"
echo "日志目录: $(pwd)/$LOG_DIR"
echo ""
echo "常用命令："
echo "  查看日志: docker logs -f $CONTAINER_NAME"
echo "  停止服务: docker stop $CONTAINER_NAME"
echo "  启动服务: docker start $CONTAINER_NAME"
echo "  重启服务: docker restart $CONTAINER_NAME"
