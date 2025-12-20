#!/bin/bash
# YOLOv8 AI 检测服务启动脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$SCRIPT_DIR/ai_detector.pid"
LOG_FILE="$SCRIPT_DIR/logs/ai_detector.log"
PYTHON_SCRIPT="$SCRIPT_DIR/ai_detector_service.py"
PORT=${AI_PORT:-8001}
MODEL_PATH=${MODEL_PATH:-"$SCRIPT_DIR/third-party/zlm/bin/models/yolov8s.onnx"}

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 创建日志目录
mkdir -p "$SCRIPT_DIR/logs"

# 生成 Python 检测服务脚本
generate_python_script() {
    cat > "$PYTHON_SCRIPT" << 'PYTHON_EOF'
#!/usr/bin/env python3
"""
YOLOv8 AI 检测服务
基于 Flask + ONNX Runtime 的目标检测 HTTP API
"""

import os
import sys
import json
import base64
import logging
import numpy as np
from io import BytesIO
from flask import Flask, request, jsonify

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# 全局变量
detector = None
MODEL_PATH = os.environ.get('MODEL_PATH', './third-party/zlm/bin/models/yolov8s.onnx')
CONFIDENCE_THRESHOLD = float(os.environ.get('CONFIDENCE', '0.5'))
IOU_THRESHOLD = float(os.environ.get('IOU_THRESHOLD', '0.45'))
INPUT_SIZE = int(os.environ.get('INPUT_SIZE', '640'))

# COCO 类别名称
COCO_CLASSES = [
    'person', 'bicycle', 'car', 'motorcycle', 'airplane', 'bus', 'train', 'truck',
    'boat', 'traffic light', 'fire hydrant', 'stop sign', 'parking meter', 'bench',
    'bird', 'cat', 'dog', 'horse', 'sheep', 'cow', 'elephant', 'bear', 'zebra',
    'giraffe', 'backpack', 'umbrella', 'handbag', 'tie', 'suitcase', 'frisbee',
    'skis', 'snowboard', 'sports ball', 'kite', 'baseball bat', 'baseball glove',
    'skateboard', 'surfboard', 'tennis racket', 'bottle', 'wine glass', 'cup',
    'fork', 'knife', 'spoon', 'bowl', 'banana', 'apple', 'sandwich', 'orange',
    'broccoli', 'carrot', 'hot dog', 'pizza', 'donut', 'cake', 'chair', 'couch',
    'potted plant', 'bed', 'dining table', 'toilet', 'tv', 'laptop', 'mouse',
    'remote', 'keyboard', 'cell phone', 'microwave', 'oven', 'toaster', 'sink',
    'refrigerator', 'book', 'clock', 'vase', 'scissors', 'teddy bear', 'hair drier',
    'toothbrush'
]


class YOLOv8Detector:
    """YOLOv8 ONNX 检测器"""
    
    def __init__(self, model_path, confidence=0.5, iou_threshold=0.45, input_size=640):
        import onnxruntime as ort
        
        self.confidence = confidence
        self.iou_threshold = iou_threshold
        self.input_size = input_size
        
        # 创建 ONNX Runtime 会话
        providers = ['CPUExecutionProvider']
        if 'CUDAExecutionProvider' in ort.get_available_providers():
            providers.insert(0, 'CUDAExecutionProvider')
            logger.info("使用 CUDA 加速")
        
        self.session = ort.InferenceSession(model_path, providers=providers)
        self.input_name = self.session.get_inputs()[0].name
        
        logger.info(f"YOLOv8 模型加载成功: {model_path}")
        logger.info(f"输入尺寸: {input_size}, 置信度阈值: {confidence}")
    
    def preprocess(self, image):
        """预处理图像"""
        from PIL import Image
        
        # 调整大小并保持比例
        orig_h, orig_w = image.shape[:2] if len(image.shape) == 3 else (image.height, image.width)
        
        if isinstance(image, np.ndarray):
            image = Image.fromarray(image)
        
        # 等比例缩放
        scale = min(self.input_size / orig_w, self.input_size / orig_h)
        new_w, new_h = int(orig_w * scale), int(orig_h * scale)
        image = image.resize((new_w, new_h), Image.BILINEAR)
        
        # 创建填充后的图像
        padded = Image.new('RGB', (self.input_size, self.input_size), (114, 114, 114))
        padded.paste(image, ((self.input_size - new_w) // 2, (self.input_size - new_h) // 2))
        
        # 转换为 numpy 数组并归一化
        img_array = np.array(padded).astype(np.float32) / 255.0
        img_array = img_array.transpose(2, 0, 1)  # HWC -> CHW
        img_array = np.expand_dims(img_array, 0)  # 添加 batch 维度
        
        return img_array, (orig_w, orig_h), scale, ((self.input_size - new_w) // 2, (self.input_size - new_h) // 2)
    
    def postprocess(self, outputs, orig_size, scale, padding):
        """后处理检测结果"""
        predictions = outputs[0]  # [1, 84, 8400] for YOLOv8
        
        if predictions.shape[1] == 84:  # [1, 84, 8400]
            predictions = predictions.transpose(0, 2, 1)  # -> [1, 8400, 84]
        
        predictions = predictions[0]  # [8400, 84]
        
        # 分离坐标和类别分数
        boxes = predictions[:, :4]  # x_center, y_center, width, height
        scores = predictions[:, 4:]  # 80 类别分数
        
        # 获取每个框的最高类别分数
        class_ids = np.argmax(scores, axis=1)
        confidences = scores[np.arange(len(scores)), class_ids]
        
        # 过滤低置信度
        mask = confidences > self.confidence
        boxes = boxes[mask]
        confidences = confidences[mask]
        class_ids = class_ids[mask]
        
        if len(boxes) == 0:
            return []
        
        # 转换为 x1, y1, x2, y2 格式
        x_center, y_center, width, height = boxes[:, 0], boxes[:, 1], boxes[:, 2], boxes[:, 3]
        x1 = x_center - width / 2
        y1 = y_center - height / 2
        x2 = x_center + width / 2
        y2 = y_center + height / 2
        
        # 调整坐标回原始图像
        pad_x, pad_y = padding
        x1 = (x1 - pad_x) / scale
        y1 = (y1 - pad_y) / scale
        x2 = (x2 - pad_x) / scale
        y2 = (y2 - pad_y) / scale
        
        # 裁剪到图像边界
        orig_w, orig_h = orig_size
        x1 = np.clip(x1, 0, orig_w)
        y1 = np.clip(y1, 0, orig_h)
        x2 = np.clip(x2, 0, orig_w)
        y2 = np.clip(y2, 0, orig_h)
        
        # NMS
        boxes_xyxy = np.stack([x1, y1, x2, y2], axis=1)
        indices = self.nms(boxes_xyxy, confidences, self.iou_threshold)
        
        results = []
        for i in indices:
            results.append({
                'class': COCO_CLASSES[class_ids[i]] if class_ids[i] < len(COCO_CLASSES) else f'class_{class_ids[i]}',
                'class_id': int(class_ids[i]),
                'confidence': float(confidences[i]),
                'bbox': {
                    'x1': float(x1[i]),
                    'y1': float(y1[i]),
                    'x2': float(x2[i]),
                    'y2': float(y2[i])
                }
            })
        
        return results
    
    def nms(self, boxes, scores, iou_threshold):
        """非极大值抑制"""
        x1, y1, x2, y2 = boxes[:, 0], boxes[:, 1], boxes[:, 2], boxes[:, 3]
        areas = (x2 - x1) * (y2 - y1)
        
        order = scores.argsort()[::-1]
        keep = []
        
        while len(order) > 0:
            i = order[0]
            keep.append(i)
            
            if len(order) == 1:
                break
            
            xx1 = np.maximum(x1[i], x1[order[1:]])
            yy1 = np.maximum(y1[i], y1[order[1:]])
            xx2 = np.minimum(x2[i], x2[order[1:]])
            yy2 = np.minimum(y2[i], y2[order[1:]])
            
            w = np.maximum(0, xx2 - xx1)
            h = np.maximum(0, yy2 - yy1)
            
            inter = w * h
            iou = inter / (areas[i] + areas[order[1:]] - inter)
            
            inds = np.where(iou <= iou_threshold)[0]
            order = order[inds + 1]
        
        return keep
    
    def detect(self, image):
        """执行检测"""
        # 预处理
        input_tensor, orig_size, scale, padding = self.preprocess(image)
        
        # 推理
        outputs = self.session.run(None, {self.input_name: input_tensor})
        
        # 后处理
        results = self.postprocess(outputs, orig_size, scale, padding)
        
        return results


def load_image_from_request():
    """从请求中加载图像"""
    from PIL import Image
    
    if 'image' in request.files:
        # 从上传的文件加载
        file = request.files['image']
        image = Image.open(file.stream).convert('RGB')
        return np.array(image)
    
    elif request.is_json:
        data = request.get_json()
        if 'image' in data:
            # Base64 编码的图像
            image_data = base64.b64decode(data['image'])
            image = Image.open(BytesIO(image_data)).convert('RGB')
            return np.array(image)
        elif 'image_path' in data:
            # 从文件路径加载
            image = Image.open(data['image_path']).convert('RGB')
            return np.array(image)
    
    return None


@app.route('/detect', methods=['POST'])
def detect():
    """检测 API 端点"""
    global detector
    
    if detector is None:
        return jsonify({'error': '检测器未初始化'}), 500
    
    try:
        image = load_image_from_request()
        if image is None:
            return jsonify({'error': '无法加载图像'}), 400
        
        results = detector.detect(image)
        
        return jsonify({
            'success': True,
            'detections': results,
            'count': len(results)
        })
    
    except Exception as e:
        logger.error(f"检测错误: {e}")
        return jsonify({'error': str(e)}), 500


@app.route('/health', methods=['GET'])
def health():
    """健康检查端点"""
    return jsonify({
        'status': 'ok',
        'model': MODEL_PATH,
        'detector': 'YOLOv8' if detector else None
    })


@app.route('/info', methods=['GET'])
def info():
    """模型信息端点"""
    return jsonify({
        'model': 'YOLOv8',
        'model_path': MODEL_PATH,
        'confidence': CONFIDENCE_THRESHOLD,
        'iou_threshold': IOU_THRESHOLD,
        'input_size': INPUT_SIZE,
        'classes': COCO_CLASSES
    })


def main():
    global detector
    
    # 检查模型文件
    if not os.path.exists(MODEL_PATH):
        logger.error(f"模型文件不存在: {MODEL_PATH}")
        sys.exit(1)
    
    # 初始化检测器
    try:
        detector = YOLOv8Detector(
            MODEL_PATH,
            confidence=CONFIDENCE_THRESHOLD,
            iou_threshold=IOU_THRESHOLD,
            input_size=INPUT_SIZE
        )
    except Exception as e:
        logger.error(f"初始化检测器失败: {e}")
        sys.exit(1)
    
    # 启动服务
    port = int(os.environ.get('PORT', 8001))
    logger.info(f"AI 检测服务启动在端口 {port}")
    app.run(host='0.0.0.0', port=port, threaded=True)


if __name__ == '__main__':
    main()
PYTHON_EOF

    chmod +x "$PYTHON_SCRIPT"
    echo -e "${GREEN}✓ Python 检测脚本已生成${NC}"
}

# 检查依赖
check_dependencies() {
    echo "检查 Python 依赖..."
    
    if ! command -v python3 &> /dev/null; then
        echo -e "${RED}✗ Python3 未安装${NC}"
        return 1
    fi
    
    # 检查必要的包
    local missing=()
    python3 -c "import flask" 2>/dev/null || missing+=("flask")
    python3 -c "import numpy" 2>/dev/null || missing+=("numpy")
    python3 -c "import PIL" 2>/dev/null || missing+=("Pillow")
    python3 -c "import onnxruntime" 2>/dev/null || missing+=("onnxruntime")
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${YELLOW}缺少 Python 包: ${missing[*]}${NC}"
        echo "正在安装..."
        pip3 install "${missing[@]}" --quiet
    fi
    
    echo -e "${GREEN}✓ 依赖检查完成${NC}"
    return 0
}

# 启动服务
start() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${YELLOW}AI 检测服务已在运行 (PID: $pid)${NC}"
            return 0
        fi
        rm -f "$PID_FILE"
    fi
    
    # 检查模型文件
    if [ ! -f "$MODEL_PATH" ]; then
        echo -e "${RED}✗ 模型文件不存在: $MODEL_PATH${NC}"
        echo "请下载 YOLOv8 模型或指定正确的路径"
        return 1
    fi
    
    check_dependencies || return 1
    generate_python_script
    
    echo "启动 AI 检测服务..."
    
    # 设置环境变量并启动
    MODEL_PATH="$MODEL_PATH" \
    PORT="$PORT" \
    CONFIDENCE="${CONFIDENCE:-0.5}" \
    IOU_THRESHOLD="${IOU_THRESHOLD:-0.45}" \
    INPUT_SIZE="${INPUT_SIZE:-640}" \
    nohup python3 "$PYTHON_SCRIPT" > "$LOG_FILE" 2>&1 &
    
    echo $! > "$PID_FILE"
    sleep 2
    
    if kill -0 $(cat "$PID_FILE") 2>/dev/null; then
        echo -e "${GREEN}✓ AI 检测服务启动成功 (PID: $(cat $PID_FILE))${NC}"
        echo "  端口: $PORT"
        echo "  模型: $MODEL_PATH"
        echo "  日志: $LOG_FILE"
        echo ""
        echo "测试命令:"
        echo "  curl http://localhost:$PORT/health"
    else
        echo -e "${RED}✗ AI 检测服务启动失败${NC}"
        echo "查看日志: tail -f $LOG_FILE"
        return 1
    fi
}

# 停止服务
stop() {
    if [ ! -f "$PID_FILE" ]; then
        echo "AI 检测服务未运行"
        return 0
    fi
    
    pid=$(cat "$PID_FILE")
    if kill -0 "$pid" 2>/dev/null; then
        echo "停止 AI 检测服务 (PID: $pid)..."
        kill "$pid"
        sleep 2
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid"
        fi
    fi
    rm -f "$PID_FILE"
    echo -e "${GREEN}✓ AI 检测服务已停止${NC}"
}

# 查看状态
status() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${GREEN}AI 检测服务运行中 (PID: $pid)${NC}"
            # 尝试获取健康状态
            if command -v curl &> /dev/null; then
                echo ""
                curl -s "http://localhost:$PORT/health" 2>/dev/null | python3 -m json.tool 2>/dev/null || true
            fi
            return 0
        fi
    fi
    echo -e "${YELLOW}AI 检测服务未运行${NC}"
    return 1
}

# 查看日志
logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo "日志文件不存在: $LOG_FILE"
    fi
}

# 主入口
case "${1:-}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        sleep 1
        start
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    *)
        echo "YOLOv8 AI 检测服务管理脚本"
        echo ""
        echo "用法: $0 {start|stop|restart|status|logs}"
        echo ""
        echo "环境变量:"
        echo "  MODEL_PATH    - 模型文件路径 (默认: $MODEL_PATH)"
        echo "  AI_PORT       - 服务端口 (默认: 8001)"
        echo "  CONFIDENCE    - 置信度阈值 (默认: 0.5)"
        echo "  IOU_THRESHOLD - NMS IOU阈值 (默认: 0.45)"
        echo "  INPUT_SIZE    - 输入尺寸 (默认: 640)"
        echo ""
        echo "示例:"
        echo "  $0 start                          # 启动服务"
        echo "  MODEL_PATH=./yolov8n.onnx $0 start  # 使用自定义模型"
        echo "  AI_PORT=8002 $0 start             # 使用自定义端口"
        ;;
esac
