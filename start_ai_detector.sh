#!/bin/bash

# AI录像检测器启动脚本
# 用于启动AI检测服务，配合智能录像功能使用

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置参数
AI_DETECTOR_PORT=${AI_DETECTOR_PORT:-8001}
AI_MODEL_TYPE=${AI_MODEL_TYPE:-"yolov8"}
AI_MODEL_PATH=${AI_MODEL_PATH:-"models/yolov8s.onnx"}
AI_CONFIDENCE=${AI_CONFIDENCE:-0.5}
AI_INPUT_SIZE=${AI_INPUT_SIZE:-320}
AI_DEVICE=${AI_DEVICE:-"cpu"}  # cpu 或 cuda
AI_WORKERS=${AI_WORKERS:-2}
LOG_DIR="logs"
PID_FILE="ai_detector.pid"

# 函数：打印信息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 函数：检查依赖
check_dependencies() {
    print_info "检查依赖..."
    
    # 检查Python
    if ! command -v python3 &> /dev/null; then
        print_error "未找到 python3，请先安装 Python 3.7+"
        exit 1
    fi
    
    PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
    print_info "Python 版本: $PYTHON_VERSION"
    
    # 检查pip
    if ! command -v pip3 &> /dev/null; then
        print_error "未找到 pip3，请先安装 pip"
        exit 1
    fi
    
    # 检查必需的Python包
    print_info "检查Python依赖包..."
    REQUIRED_PACKAGES=("flask" "opencv-python" "numpy" "onnxruntime")
    MISSING_PACKAGES=()
    
    for package in "${REQUIRED_PACKAGES[@]}"; do
        if ! python3 -c "import ${package//-/_}" &> /dev/null; then
            MISSING_PACKAGES+=("$package")
        fi
    done
    
    if [ ${#MISSING_PACKAGES[@]} -gt 0 ]; then
        print_warning "缺少以下Python包: ${MISSING_PACKAGES[*]}"
        print_info "正在安装缺失的包..."
        pip3 install "${MISSING_PACKAGES[@]}" || {
            print_error "安装依赖失败，请手动执行: pip3 install ${MISSING_PACKAGES[*]}"
            exit 1
        }
    fi
    
    print_success "所有依赖检查通过"
}

# 函数：检查模型文件
check_model() {
    print_info "检查AI模型文件..."
    
    if [ ! -f "$AI_MODEL_PATH" ]; then
        print_error "模型文件不存在: $AI_MODEL_PATH"
        print_info "请下载YOLO模型文件并放置到: $AI_MODEL_PATH"
        print_info "推荐模型: YOLOv8s (https://github.com/ultralytics/ultralytics)"
        exit 1
    fi
    
    MODEL_SIZE=$(du -h "$AI_MODEL_PATH" | cut -f1)
    print_success "模型文件已找到: $AI_MODEL_PATH ($MODEL_SIZE)"
}

# 函数：创建Python检测服务
create_detector_service() {
    cat > ai_detector_service.py << 'PYTHON_SCRIPT'
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AI检测服务 - HTTP API
用于接收图像并返回检测结果（人员检测）
"""

import os
import sys
import logging
import base64
import io
from flask import Flask, request, jsonify
from PIL import Image
import numpy as np
import cv2
import onnxruntime as ort

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.FileHandler('logs/ai_detector.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Flask应用
app = Flask(__name__)

# 全局检测器
detector = None

class YOLOv8Detector:
    """YOLOv8 ONNX检测器"""
    
    def __init__(self, model_path, confidence=0.5, input_size=320):
        self.model_path = model_path
        self.confidence = confidence
        self.input_size = input_size
        
        logger.info(f"加载模型: {model_path}")
        self.session = ort.InferenceSession(
            model_path,
            providers=['CPUExecutionProvider']
        )
        
        # 获取输入输出信息
        self.input_name = self.session.get_inputs()[0].name
        self.output_names = [out.name for out in self.session.get_outputs()]
        
        logger.info(f"模型加载成功 - 输入: {self.input_name}, 输出: {self.output_names}")
        logger.info(f"置信度阈值: {self.confidence}, 输入尺寸: {self.input_size}")
    
    def preprocess(self, image):
        """预处理图像"""
        # 调整大小
        img_resized = cv2.resize(image, (self.input_size, self.input_size))
        
        # 归一化到 [0, 1]
        img_normalized = img_resized.astype(np.float32) / 255.0
        
        # 转换为 CHW 格式
        img_transposed = np.transpose(img_normalized, (2, 0, 1))
        
        # 添加批次维度
        img_batch = np.expand_dims(img_transposed, axis=0)
        
        return img_batch
    
    def postprocess(self, outputs, original_shape):
        """后处理检测结果"""
        # YOLOv8输出格式: [batch, 84, 8400] (80个类别 + 4个框坐标)
        output = outputs[0][0]  # 移除批次维度
        
        # 转置为 [8400, 84]
        output = output.transpose()
        
        # 提取框和置信度
        boxes = []
        confidences = []
        class_ids = []
        
        for detection in output:
            # 前4个值是框坐标 (x_center, y_center, width, height)
            box = detection[:4]
            # 后80个值是类别置信度
            scores = detection[4:]
            
            # 找到最高置信度的类别
            class_id = np.argmax(scores)
            confidence = scores[class_id]
            
            # 只保留"人"类别 (COCO数据集中person是类别0)
            if class_id == 0 and confidence >= self.confidence:
                boxes.append(box)
                confidences.append(float(confidence))
                class_ids.append(int(class_id))
        
        # 转换框坐标
        detections = []
        h, w = original_shape[:2]
        scale_x = w / self.input_size
        scale_y = h / self.input_size
        
        for box, conf, cls_id in zip(boxes, confidences, class_ids):
            x_center, y_center, width, height = box
            
            # 转换到原始图像坐标
            x1 = int((x_center - width / 2) * scale_x)
            y1 = int((y_center - height / 2) * scale_y)
            x2 = int((x_center + width / 2) * scale_x)
            y2 = int((y_center + height / 2) * scale_y)
            
            detections.append({
                'label': 'person',
                'confidence': conf,
                'bbox': [x1, y1, x2, y2]
            })
        
        return detections
    
    def detect(self, image):
        """执行检测"""
        try:
            # 预处理
            input_tensor = self.preprocess(image)
            
            # 推理
            outputs = self.session.run(self.output_names, {self.input_name: input_tensor})
            
            # 后处理
            detections = self.postprocess(outputs, image.shape)
            
            return detections
        except Exception as e:
            logger.error(f"检测失败: {e}")
            return []

@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({
        'status': 'ok',
        'model': detector.model_path if detector else 'not loaded',
        'confidence': detector.confidence if detector else 0
    })

@app.route('/detect', methods=['POST'])
def detect():
    """检测接口"""
    try:
        # 获取图像数据
        if 'image' not in request.files and 'image_base64' not in request.json:
            return jsonify({
                'success': False,
                'error': 'No image provided'
            }), 400
        
        # 读取图像
        if 'image' in request.files:
            # 文件上传
            file = request.files['image']
            image_bytes = file.read()
        else:
            # Base64编码
            image_base64 = request.json['image_base64']
            image_bytes = base64.b64decode(image_base64)
        
        # 转换为numpy数组
        image = Image.open(io.BytesIO(image_bytes))
        image_np = np.array(image)
        
        # BGR转换
        if len(image_np.shape) == 2:
            image_np = cv2.cvtColor(image_np, cv2.COLOR_GRAY2BGR)
        elif image_np.shape[2] == 4:
            image_np = cv2.cvtColor(image_np, cv2.COLOR_RGBA2BGR)
        elif image_np.shape[2] == 3:
            image_np = cv2.cvtColor(image_np, cv2.COLOR_RGB2BGR)
        
        # 执行检测
        detections = detector.detect(image_np)
        
        logger.info(f"检测完成: 发现 {len(detections)} 个目标")
        
        return jsonify({
            'success': True,
            'detections': detections,
            'count': len(detections)
        })
        
    except Exception as e:
        logger.error(f"检测请求处理失败: {e}", exc_info=True)
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

def main():
    global detector
    
    # 从环境变量读取配置
    model_path = os.getenv('AI_MODEL_PATH', 'third-party/zlm/models/yolov8s.onnx')
    confidence = float(os.getenv('AI_CONFIDENCE', '0.5'))
    input_size = int(os.getenv('AI_INPUT_SIZE', '320'))
    port = int(os.getenv('AI_DETECTOR_PORT', '8001'))
    workers = int(os.getenv('AI_WORKERS', '2'))
    
    # 创建日志目录
    os.makedirs('logs', exist_ok=True)
    
    logger.info("=" * 60)
    logger.info("AI检测服务启动中...")
    logger.info(f"模型路径: {model_path}")
    logger.info(f"置信度: {confidence}")
    logger.info(f"输入尺寸: {input_size}")
    logger.info(f"监听端口: {port}")
    logger.info("=" * 60)
    
    # 初始化检测器
    detector = YOLOv8Detector(model_path, confidence, input_size)
    
    logger.info("检测器初始化完成，启动HTTP服务...")
    
    # 启动Flask应用
    app.run(
        host='0.0.0.0',
        port=port,
        debug=False,
        threaded=True
    )

if __name__ == '__main__':
    main()
PYTHON_SCRIPT
    
    chmod +x ai_detector_service.py
    print_success "检测服务脚本已创建"
}

# 函数：启动服务
start_service() {
    print_info "启动AI检测服务..."
    
    # 检查是否已在运行
    if [ -f "$PID_FILE" ]; then
        OLD_PID=$(cat "$PID_FILE")
        if kill -0 "$OLD_PID" 2>/dev/null; then
            print_warning "服务已在运行 (PID: $OLD_PID)"
            return 0
        fi
        rm -f "$PID_FILE"
    fi
    
    # 创建日志目录
    mkdir -p "$LOG_DIR"
    
    # 设置环境变量并启动
    export AI_MODEL_PATH
    export AI_CONFIDENCE
    export AI_INPUT_SIZE
    export AI_DETECTOR_PORT
    export AI_WORKERS
    
    nohup python3 ai_detector_service.py > "$LOG_DIR/ai_detector.log" 2>&1 &
    PID=$!
    echo $PID > "$PID_FILE"
    
    # 等待服务启动
    sleep 3
    
    # 检查服务是否成功启动
    if kill -0 "$PID" 2>/dev/null; then
        print_success "AI检测服务已启动 (PID: $PID)"
        print_info "监听端口: $AI_DETECTOR_PORT"
        print_info "日志文件: $LOG_DIR/ai_detector.log"
        
        # 测试健康检查
        sleep 2
        if curl -s "http://localhost:$AI_DETECTOR_PORT/health" > /dev/null 2>&1; then
            print_success "服务健康检查通过"
            curl -s "http://localhost:$AI_DETECTOR_PORT/health" | python3 -m json.tool
        else
            print_warning "服务健康检查失败，请查看日志"
        fi
    else
        print_error "服务启动失败，请查看日志: $LOG_DIR/ai_detector.log"
        rm -f "$PID_FILE"
        exit 1
    fi
}

# 函数：停止服务
stop_service() {
    print_info "停止AI检测服务..."
    
    if [ ! -f "$PID_FILE" ]; then
        print_warning "服务未运行"
        return 0
    fi
    
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        kill "$PID"
        rm -f "$PID_FILE"
        print_success "服务已停止 (PID: $PID)"
    else
        print_warning "进程不存在 (PID: $PID)"
        rm -f "$PID_FILE"
    fi
}

# 函数：查看状态
show_status() {
    print_info "检查服务状态..."
    
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            print_success "服务运行中 (PID: $PID)"
            echo ""
            echo "配置信息:"
            echo "  端口: $AI_DETECTOR_PORT"
            echo "  模型: $AI_MODEL_PATH"
            echo "  置信度: $AI_CONFIDENCE"
            echo "  输入尺寸: ${AI_INPUT_SIZE}x${AI_INPUT_SIZE}"
            echo ""
            
            # 尝试健康检查
            if curl -s "http://localhost:$AI_DETECTOR_PORT/health" > /dev/null 2>&1; then
                echo "健康状态:"
                curl -s "http://localhost:$AI_DETECTOR_PORT/health" | python3 -m json.tool
            fi
        else
            print_warning "PID文件存在但进程不存在"
            rm -f "$PID_FILE"
        fi
    else
        print_info "服务未运行"
    fi
}

# 函数：查看日志
show_logs() {
    if [ -f "$LOG_DIR/ai_detector.log" ]; then
        print_info "显示最近的日志 (Ctrl+C 退出)..."
        tail -f "$LOG_DIR/ai_detector.log"
    else
        print_warning "日志文件不存在"
    fi
}

# 函数：测试检测
test_detection() {
    print_info "测试检测功能..."
    
    # 创建测试图片（纯色图片）
    python3 << 'PYTHON_TEST'
import cv2
import numpy as np
import base64
import requests
import json

# 创建测试图片
img = np.zeros((480, 640, 3), dtype=np.uint8)
cv2.putText(img, 'Test Image', (200, 240), cv2.FONT_HERSHEY_SIMPLEX, 1, (255, 255, 255), 2)

# 编码为JPEG
_, buffer = cv2.imencode('.jpg', img)
img_base64 = base64.b64encode(buffer).decode('utf-8')

# 发送检测请求
try:
    response = requests.post(
        'http://localhost:8001/detect',
        json={'image_base64': img_base64},
        timeout=10
    )
    print(json.dumps(response.json(), indent=2, ensure_ascii=False))
except Exception as e:
    print(f"测试失败: {e}")
PYTHON_TEST
}

# 主函数
main() {
    echo ""
    echo "=========================================="
    echo "   AI录像检测器管理脚本"
    echo "=========================================="
    echo ""
    
    case "${1:-}" in
        start)
            check_dependencies
            check_model
            create_detector_service
            start_service
            ;;
        stop)
            stop_service
            ;;
        restart)
            stop_service
            sleep 2
            check_dependencies
            check_model
            start_service
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        test)
            test_detection
            ;;
        *)
            echo "用法: $0 {start|stop|restart|status|logs|test}"
            echo ""
            echo "命令说明:"
            echo "  start   - 启动AI检测服务"
            echo "  stop    - 停止AI检测服务"
            echo "  restart - 重启AI检测服务"
            echo "  status  - 查看服务状态"
            echo "  logs    - 查看实时日志"
            echo "  test    - 测试检测功能"
            echo ""
            echo "环境变量:"
            echo "  AI_DETECTOR_PORT  - 服务端口 (默认: 8001)"
            echo "  AI_MODEL_PATH     - 模型路径 (默认: models/yolov8s.onnx)"
            echo "  AI_CONFIDENCE     - 置信度阈值 (默认: 0.5)"
            echo "  AI_INPUT_SIZE     - 输入尺寸 (默认: 320)"
            echo "  AI_DEVICE         - 设备类型 (默认: cpu)"
            echo ""
            echo "示例:"
            echo "  $0 start                          # 使用默认配置启动"
            echo "  AI_CONFIDENCE=0.6 $0 start        # 使用自定义置信度启动"
            echo "  AI_DETECTOR_PORT=8001 $0 start    # 使用自定义端口启动"
            echo ""
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"
