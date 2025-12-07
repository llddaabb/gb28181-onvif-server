#!/bin/bash

# AI检测器依赖安装脚本

set -e

echo "=========================================="
echo "   AI检测器依赖安装"
echo "=========================================="
echo ""

# 检查Python
if ! command -v python3 &> /dev/null; then
    echo "错误: 未找到 python3"
    echo "请安装 Python 3.7 或更高版本"
    exit 1
fi

PYTHON_VERSION=$(python3 --version)
echo "✓ 找到 $PYTHON_VERSION"

# 检查pip
if ! command -v pip3 &> /dev/null; then
    echo "错误: 未找到 pip3"
    echo "请先安装 pip"
    exit 1
fi

echo "✓ 找到 pip3"
echo ""

# 安装依赖
echo "正在安装Python依赖包..."
echo ""

pip3 install flask opencv-python numpy onnxruntime requests pillow || {
    echo ""
    echo "错误: 安装失败"
    echo "请尝试手动安装: pip3 install flask opencv-python numpy onnxruntime requests pillow"
    exit 1
}

echo ""
echo "=========================================="
echo "   依赖安装完成"
echo "=========================================="
echo ""
echo "接下来的步骤:"
echo "1. 下载 YOLOv8 模型文件到: third-party/zlm/models/yolov8s.onnx"
echo "   推荐: https://github.com/ultralytics/assets/releases/download/v0.0.0/yolov8s.onnx"
echo ""
echo "2. 启动检测服务:"
echo "   ./start_ai_detector.sh start"
echo ""
echo "3. 在 config.yaml 中配置 AI 部分:"
echo "   AI:"
echo "     Enable: true"
echo "     APIEndpoint: http://localhost:8000"
echo ""
