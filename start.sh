#!/bin/bash

# 服务名称或描述
SERVICE_NAME="DevOps MCP Server Proxy"

# 程序路径
PROGRAM="./mcp-link-linux-devops"
CMD="serve --port 8089 --host 0.0.0.0"

# log 文件路径
LOG_FILE="./log.log"

# 检查服务是否已运行
pid=$(pgrep -f "$PROGRAM")

if [ -n "$pid" ]; then
    echo "[$(date)] $SERVICE_NAME is already running (PID: $pid)" >> "$LOG_FILE"
    exit 1
fi

# 启动程序并将输出重定向到 log 文件
echo "[$(date)] Starting $SERVICE_NAME..." >> "$LOG_FILE"
nohup $PROGRAM $CMD >> "$LOG_FILE" 2>&1 &

echo "[$(date)] $SERVICE_NAME started with PID: $!" >> "$LOG_FILE"
