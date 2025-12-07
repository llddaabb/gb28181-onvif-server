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
