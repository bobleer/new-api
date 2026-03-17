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

echo "==> 停止并删除旧容器（如果存在）"
docker stop $CONTAINER_NAME 2>/dev/null || true
docker rm $CONTAINER_NAME 2>/dev/null || true

echo "==> 构建 Docker 镜像"
docker build --platform linux/amd64 -t $IMAGE_NAME .

echo "==> 创建数据目录"
mkdir -p $DATA_DIR $LOG_DIR

echo "==> 启动容器"
docker run --name $CONTAINER_NAME -d --restart always \
  -p 127.0.0.1:$PORT:3000 \
  -e TZ=Asia/Shanghai \
  -e ERROR_LOG_ENABLED=true \
  -e BATCH_UPDATE_ENABLED=true \
  -v $(pwd)/$DATA_DIR:/data \
  -v $(pwd)/$LOG_DIR:/app/logs \
  $IMAGE_NAME --log-dir /app/logs

echo "==> 配置开机自启（systemd）"
SERVICE_FILE="/etc/systemd/system/new-api-docker.service"

if [ ! -f "$SERVICE_FILE" ]; then
    echo "创建 systemd 服务文件..."
    sudo tee $SERVICE_FILE > /dev/null <<EOF
[Unit]
Description=New API Docker Container
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$(pwd)
ExecStart=/usr/bin/docker start $CONTAINER_NAME
ExecStop=/usr/bin/docker stop $CONTAINER_NAME
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable new-api-docker.service
    echo "✓ 开机自启已配置"
else
    echo "✓ systemd 服务已存在"
fi

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
