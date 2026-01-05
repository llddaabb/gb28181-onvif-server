#!/bin/bash

# GB28181/ONVIF 服务器快速启动脚本
# 用于快速启动服务器，跳过检查

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 创建必要的目录
mkdir -p logs recordings

# 配置库路径
if [ -d "./lib" ]; then
    export LD_LIBRARY_PATH="./lib:$LD_LIBRARY_PATH"
fi

# 查找可执行文件
if [ -f "./gb28181-server" ]; then
    EXECUTABLE="./gb28181-server"
elif [ -f "./dist/gb28181-server" ]; then
    EXECUTABLE="./dist/gb28181-server"
elif [ -f "./server" ]; then
    EXECUTABLE="./server"
else
    echo "错误: 未找到可执行文件"
    exit 1
fi

echo "启动服务器: $EXECUTABLE"
echo "Web 管理界面: http://localhost:9080"
echo ""

exec "$EXECUTABLE"
