#!/usr/bin/python3
"""
YOLOv8 检测服务
提供 HTTP API 接口供 GB28181 服务器调用
"""

import io
import logging
import argparse
from typing import List, Dict, Any
from pathlib import Path

from flask import Flask, request, jsonify
from PIL import Image
import numpy as np

try:
    from ultralytics import YOLO
except ImportError as e:
    print(f"错误: 未安装 ultralytics 库或缺少依赖")
    print(f"详细信息: {e}")
    print("请运行: pip install ultralytics")
    print("\n如果遇到 '_bz2' 错误，请安装系统依赖:")
    print("  Ubuntu/Debian: sudo apt-get install libbz2-dev python3-bz2")
    print("  然后重新安装 Python 或使用系统 Python: /usr/bin/python3")
    exit(1)

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y/%m/%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# 全局变量
model = None
model_name = None
confidence_threshold = 0.5


def load_model(model_path: str, device: str = 'cpu', auto_download: bool = False) -> YOLO:
    """加载 YOLOv8 模型"""
    logger.info(f"正在加载模型: {model_path}")
    
    # 如果是预训练模型名称（如 yolov8n.pt），YOLO 会自动下载
    # 只检查本地文件路径是否存在
    if not auto_download and not Path(model_path).exists():
        raise FileNotFoundError(f"模型文件不存在: {model_path}")
    
    try:
        # YOLO 会自动下载预训练模型（如 yolov8n.pt）
        model = YOLO(model_path)
        
        # 只有 .pt 模型支持 to(device)，ONNX 等导出格式不支持
        if model_path.endswith('.pt'):
            model.to(device)
            logger.info(f"✓ 模型加载成功: {model_path}")
            logger.info(f"✓ 使用设备: {device}")
        else:
            logger.info(f"✓ 模型加载成功: {model_path}")
            logger.info(f"✓ 导出格式模型，设备选择在推理时指定")
        
        return model
    except Exception as e:
        logger.error(f"模型加载失败: {e}")
        raise


@app.route('/health', methods=['GET'])
def health_check():
    """健康检查接口"""
    return jsonify({
        'status': 'ok',
        'model': model_name,
        'device': str(model.device) if model else 'unknown'
    })


@app.route('/detect', methods=['POST'])
def detect():
    """检测接口"""
    try:
        # 获取置信度阈值
        conf_threshold = float(request.headers.get('X-Confidence-Threshold', confidence_threshold))
        
        # 读取图像
        image_bytes = request.get_data()
        if not image_bytes:
            return jsonify({
                'success': False,
                'error': '未接收到图像数据'
            }), 400
        
        # 解码图像
        try:
            image = Image.open(io.BytesIO(image_bytes))
            if image.mode != 'RGB':
                image = image.convert('RGB')
        except Exception as e:
            return jsonify({
                'success': False,
                'error': f'图像解码失败: {str(e)}'
            }), 400
        
        # 执行检测
        results = model(image, conf=conf_threshold, verbose=False)
        
        # 解析结果
        boxes_data = []
        person_count = 0
        max_confidence = 0.0
        
        if len(results) > 0 and results[0].boxes is not None:
            boxes = results[0].boxes
            
            for box in boxes:
                # 获取类别
                cls = int(box.cls[0])
                class_name = model.names[cls]
                conf = float(box.conf[0])
                
                # 检查是否是人（COCO 数据集中 person 的类别 ID 是 0）
                if cls == 0:  # person
                    person_count += 1
                    max_confidence = max(max_confidence, conf)
                
                # 获取边界框坐标 (x1, y1, x2, y2)
                xyxy = box.xyxy[0].tolist()
                
                boxes_data.append({
                    'x1': float(xyxy[0]),
                    'y1': float(xyxy[1]),
                    'x2': float(xyxy[2]),
                    'y2': float(xyxy[3]),
                    'confidence': conf,
                    'class': class_name
                })
        
        # 构建响应
        response = {
            'success': True,
            'has_person': person_count > 0,
            'person_count': person_count,
            'confidence': max_confidence,
            'boxes': boxes_data
        }
        
        if person_count > 0:
            logger.info(f"✓ 检测到 {person_count} 个人，置信度: {max_confidence:.2f}")
        
        return jsonify(response)
    
    except Exception as e:
        logger.error(f"检测错误: {e}", exc_info=True)
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500


def find_model() -> str:
    """自动查找可用的模型文件"""
    model_paths = [
        # 优先查找 .pt 模型
        'models/yolov8n.pt',
        'models/yolov8s.pt',
        'models/yolov8m.pt',
        'models/yolov8l.pt',
        'models/yolov8x.pt',
        '../models/yolov8n.pt',
        '../models/yolov8s.pt',
        'yolov8n.pt',
        'yolov8s.pt',
        # 也支持 ONNX 模型
        'models/yolov8n.onnx',
        'models/yolov8s.onnx',
        '../models/yolov8n.onnx',
        '../models/yolov8s.onnx',
        'yolov8n.onnx',
        'yolov8s.onnx',
    ]
    
    for path in model_paths:
        if Path(path).exists() and Path(path).stat().st_size > 0:
            return path
    
    return None


def main():
    parser = argparse.ArgumentParser(description='YOLOv8 检测服务')
    parser.add_argument(
        '--model',
        type=str,
        default=None,
        help='模型文件路径 (默认: 自动查找)'
    )
    parser.add_argument(
        '--device',
        type=str,
        default='cpu',
        choices=['cpu', 'cuda', '0', '1', '2', '3'],
        help='运行设备 (默认: cpu)'
    )
    parser.add_argument(
        '--host',
        type=str,
        default='0.0.0.0',
        help='监听地址 (默认: 0.0.0.0)'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=8000,
        help='监听端口 (默认: 8000)'
    )
    parser.add_argument(
        '--confidence',
        type=float,
        default=0.5,
        help='默认置信度阈值 (默认: 0.5)'
    )
    parser.add_argument(
        '--download',
        action='store_true',
        help='如果模型不存在，自动下载'
    )
    
    args = parser.parse_args()
    
    global model, model_name, confidence_threshold
    confidence_threshold = args.confidence
    
    # 确定模型路径
    model_path = args.model
    if not model_path:
        model_path = find_model()
        if not model_path:
            if args.download:
                logger.info("未找到本地模型，将自动下载 yolov8n.pt")
                model_path = 'yolov8n.pt'
            else:
                logger.error("未找到模型文件")
                logger.error("请使用 --model 指定模型路径，或使用 --download 自动下载")
                logger.error("支持的模型: yolov8n.pt, yolov8s.pt, yolov8m.pt, yolov8l.pt, yolov8x.pt")
                return 1
    
    model_name = Path(model_path).name
    
    # 加载模型
    try:
        model = load_model(model_path, args.device, auto_download=args.download)
    except Exception as e:
        logger.error(f"无法加载模型: {e}")
        return 1
    
    # 启动服务
    logger.info("=" * 60)
    logger.info("YOLOv8 检测服务")
    logger.info("=" * 60)
    logger.info(f"模型: {model_name}")
    logger.info(f"设备: {args.device}")
    logger.info(f"地址: http://{args.host}:{args.port}")
    logger.info(f"API 端点: http://{args.host}:{args.port}/detect")
    logger.info(f"健康检查: http://{args.host}:{args.port}/health")
    logger.info(f"默认置信度: {confidence_threshold}")
    logger.info("=" * 60)
    logger.info("服务已启动，等待请求...")
    logger.info("按 Ctrl+C 停止服务")
    
    app.run(
        host=args.host,
        port=args.port,
        debug=False,
        threaded=True
    )
    
    return 0


if __name__ == '__main__':
    exit(main())
