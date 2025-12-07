#!/bin/bash

# AI模型下载脚本

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

MODEL_DIR="models"
MODEL_FILE="yolov8s.onnx"
MODEL_PATH="$MODEL_DIR/$MODEL_FILE"

# YOLOv8 模型下载地址（使用多个镜像源）
# 主源
YOLOV8N_URL="https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8n.onnx"
YOLOV8S_URL="https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8s.onnx"
YOLOV8M_URL="https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8m.onnx"

# 备用源（如果主源失败）
YOLOV8N_URL_BACKUP="https://raw.githubusercontent.com/ultralytics/assets/main/yolov8n.onnx"
YOLOV8S_URL_BACKUP="https://raw.githubusercontent.com/ultralytics/assets/main/yolov8s.onnx"
YOLOV8M_URL_BACKUP="https://raw.githubusercontent.com/ultralytics/assets/main/yolov8m.onnx"

echo -e "${BLUE}=========================================="
echo "   YOLOv8 模型下载工具"
echo -e "==========================================${NC}"
echo ""

# 创建目录
mkdir -p "$MODEL_DIR"

# 选择模型
echo "请选择要下载的模型:"
echo ""
echo "  1) YOLOv8n - 最小模型 (~6MB, 最快)"
echo "  2) YOLOv8s - 标准模型 (~22MB, 推荐) [默认]"
echo "  3) YOLOv8m - 中等模型 (~52MB, 更准确)"
echo ""
read -p "请输入选择 [1-3, 默认2]: " choice

case ${choice:-2} in
    1)
        MODEL_URL="$YOLOV8N_URL"
        MODEL_URL_BACKUP="$YOLOV8N_URL_BACKUP"
        MODEL_FILE="yolov8n.onnx"
        MODEL_SIZE="6MB"
        ;;
    2)
        MODEL_URL="$YOLOV8S_URL"
        MODEL_URL_BACKUP="$YOLOV8S_URL_BACKUP"
        MODEL_FILE="yolov8s.onnx"
        MODEL_SIZE="22MB"
        ;;
    3)
        MODEL_URL="$YOLOV8M_URL"
        MODEL_URL_BACKUP="$YOLOV8M_URL_BACKUP"
        MODEL_FILE="yolov8m.onnx"
        MODEL_SIZE="52MB"
        ;;
    *)
        echo -e "${YELLOW}无效选择，使用默认模型 YOLOv8s${NC}"
        MODEL_URL="$YOLOV8S_URL"
        MODEL_URL_BACKUP="$YOLOV8S_URL_BACKUP"
        MODEL_FILE="yolov8s.onnx"
        MODEL_SIZE="22MB"
        ;;
esac

MODEL_PATH="$MODEL_DIR/$MODEL_FILE"

echo ""
echo -e "${BLUE}下载信息:${NC}"
echo "  模型: $MODEL_FILE"
echo "  大小: ~$MODEL_SIZE"
echo "  路径: $MODEL_PATH"
echo ""

# 检查是否已存在
if [ -f "$MODEL_PATH" ] && [ -s "$MODEL_PATH" ]; then
    CURRENT_SIZE=$(du -h "$MODEL_PATH" | cut -f1)
    echo -e "${YELLOW}模型文件已存在 ($CURRENT_SIZE)${NC}"
    read -p "是否覆盖下载? [y/N]: " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "取消下载"
        exit 0
    fi
    rm -f "$MODEL_PATH"
fi

# 下载模型
echo -e "${BLUE}正在下载模型...${NC}"

download_success=false

# 尝试主源
if command -v wget &> /dev/null; then
    wget --progress=bar:force -O "$MODEL_PATH" "$MODEL_URL" && download_success=true || {
        echo "主源下载失败，尝试备用源..."
        wget --progress=bar:force -O "$MODEL_PATH" "$MODEL_URL_BACKUP" && download_success=true
    }
elif command -v curl &> /dev/null; then
    curl -L --progress-bar -o "$MODEL_PATH" "$MODEL_URL" && download_success=true || {
        echo "主源下载失败，尝试备用源..."
        curl -L --progress-bar -o "$MODEL_PATH" "$MODEL_URL_BACKUP" && download_success=true
    }
else
    echo "错误: 需要 wget 或 curl 来下载文件"
    echo "请安装: sudo apt-get install wget"
    exit 1
fi

if [ "$download_success" = false ]; then
    echo "下载失败，所有源都无法访问"
    echo ""
    echo "请手动下载模型文件:"
    echo "  URL: $MODEL_URL"
    echo "  保存到: $MODEL_PATH"
    rm -f "$MODEL_PATH"
    exit 1
fi

# 验证下载
if [ -f "$MODEL_PATH" ] && [ -s "$MODEL_PATH" ]; then
    FINAL_SIZE=$(du -h "$MODEL_PATH" | cut -f1)
    echo ""
    echo -e "${GREEN}=========================================="
    echo "   下载完成!"
    echo -e "==========================================${NC}"
    echo "  文件: $MODEL_PATH"
    echo "  大小: $FINAL_SIZE"
    echo ""
    echo "下一步:"
    echo "  1. 启动检测服务: ./start_ai_detector.sh start"
    echo "  2. 查看服务状态: ./start_ai_detector.sh status"
    echo ""
else
    echo "错误: 模型文件下载失败或文件为空"
    rm -f "$MODEL_PATH"
    exit 1
fi
