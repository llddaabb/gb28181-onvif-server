#!/bin/bash
# YOLOv8 检测服务快速启动脚本

cd "$(dirname "$0")/.."

echo "==================================================="
echo "YOLOv8 检测服务快速启动"
echo "==================================================="

# 检查 Python
if ! command -v python3 &> /dev/null; then
    echo "错误: 未找到 python3"
    exit 1
fi

# 检查虚拟环境
if [ ! -d "venv" ]; then
    echo "首次运行，创建虚拟环境..."
    python3 -m venv venv
    echo "✓ 虚拟环境创建完成"
fi

# 激活虚拟环境
echo "激活虚拟环境..."
source venv/bin/activate

# 检查依赖
if ! python -c "import ultralytics" 2>/dev/null; then
    echo "安装依赖..."
    pip install -q -r tools/requirements.txt
    echo "✓ 依赖安装完成"
fi

# 检查模型
MODEL=""
if [ -f "models/yolov8n.pt" ]; then
    MODEL="models/yolov8n.pt"
elif [ -f "models/yolov8s.pt" ]; then
    MODEL="models/yolov8s.pt"
elif [ -f "models/yolov8m.pt" ]; then
    MODEL="models/yolov8m.pt"
fi

# 启动服务
echo ""
echo "启动 YOLOv8 检测服务..."
echo ""

if [ -z "$MODEL" ]; then
    echo "未找到本地模型，将自动下载 yolov8n.pt..."
    python tools/yolo_server.py --download "$@"
else
    echo "使用模型: $MODEL"
    python tools/yolo_server.py --model "$MODEL" "$@"
fi
