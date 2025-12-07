#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AI检测器测试工具
用于测试AI检测服务的功能
"""

import argparse
import base64
import json
import sys
import requests
from pathlib import Path

def test_health(endpoint):
    """测试健康检查接口"""
    print("=" * 60)
    print("测试健康检查接口...")
    print("=" * 60)
    
    try:
        response = requests.get(f"{endpoint}/health", timeout=5)
        response.raise_for_status()
        
        result = response.json()
        print(f"✓ 服务状态: {result.get('status')}")
        print(f"✓ 模型路径: {result.get('model')}")
        print(f"✓ 置信度阈值: {result.get('confidence')}")
        return True
    except Exception as e:
        print(f"✗ 健康检查失败: {e}")
        return False

def test_detect_file(endpoint, image_path):
    """测试文件检测"""
    print("\n" + "=" * 60)
    print(f"测试图片检测: {image_path}")
    print("=" * 60)
    
    if not Path(image_path).exists():
        print(f"✗ 图片文件不存在: {image_path}")
        return False
    
    try:
        with open(image_path, 'rb') as f:
            files = {'image': f}
            response = requests.post(f"{endpoint}/detect", files=files, timeout=30)
            response.raise_for_status()
        
        result = response.json()
        
        if result.get('success'):
            count = result.get('count', 0)
            detections = result.get('detections', [])
            
            print(f"✓ 检测成功")
            print(f"✓ 检测到 {count} 个目标")
            
            if detections:
                print("\n检测结果:")
                for i, det in enumerate(detections, 1):
                    label = det.get('label', 'unknown')
                    conf = det.get('confidence', 0)
                    bbox = det.get('bbox', [])
                    print(f"  [{i}] {label} - 置信度: {conf:.2f} - 位置: {bbox}")
            else:
                print("  未检测到目标")
            
            return True
        else:
            print(f"✗ 检测失败: {result.get('error')}")
            return False
            
    except Exception as e:
        print(f"✗ 检测请求失败: {e}")
        return False

def test_detect_base64(endpoint, image_path):
    """测试Base64检测"""
    print("\n" + "=" * 60)
    print(f"测试Base64检测: {image_path}")
    print("=" * 60)
    
    if not Path(image_path).exists():
        print(f"✗ 图片文件不存在: {image_path}")
        return False
    
    try:
        with open(image_path, 'rb') as f:
            img_data = f.read()
            img_base64 = base64.b64encode(img_data).decode('utf-8')
        
        data = {'image_base64': img_base64}
        response = requests.post(
            f"{endpoint}/detect",
            json=data,
            timeout=30,
            headers={'Content-Type': 'application/json'}
        )
        response.raise_for_status()
        
        result = response.json()
        
        if result.get('success'):
            count = result.get('count', 0)
            detections = result.get('detections', [])
            
            print(f"✓ 检测成功")
            print(f"✓ 检测到 {count} 个目标")
            
            if detections:
                print("\n检测结果:")
                for i, det in enumerate(detections, 1):
                    label = det.get('label', 'unknown')
                    conf = det.get('confidence', 0)
                    bbox = det.get('bbox', [])
                    print(f"  [{i}] {label} - 置信度: {conf:.2f} - 位置: {bbox}")
            else:
                print("  未检测到目标")
            
            return True
        else:
            print(f"✗ 检测失败: {result.get('error')}")
            return False
            
    except Exception as e:
        print(f"✗ Base64检测失败: {e}")
        return False

def create_test_image():
    """创建测试图片"""
    try:
        import cv2
        import numpy as np
        
        # 创建纯色测试图片
        img = np.zeros((480, 640, 3), dtype=np.uint8)
        img[:] = (50, 50, 50)
        
        # 添加文字
        cv2.putText(
            img,
            'AI Detector Test Image',
            (100, 240),
            cv2.FONT_HERSHEY_SIMPLEX,
            1,
            (255, 255, 255),
            2
        )
        
        # 保存
        test_img_path = 'test_image.jpg'
        cv2.imwrite(test_img_path, img)
        print(f"✓ 创建测试图片: {test_img_path}")
        return test_img_path
        
    except ImportError:
        print("✗ 需要 opencv-python 来创建测试图片")
        return None

def main():
    parser = argparse.ArgumentParser(description='AI检测器测试工具')
    parser.add_argument(
        '--endpoint',
        default='http://localhost:8001',
        help='AI检测服务地址 (默认: http://localhost:8001)'
    )
    parser.add_argument(
        '--image',
        help='测试图片路径'
    )
    parser.add_argument(
        '--create-test-image',
        action='store_true',
        help='创建测试图片'
    )
    
    args = parser.parse_args()
    
    print("\n" + "=" * 60)
    print("AI检测器测试工具")
    print("=" * 60)
    print(f"服务地址: {args.endpoint}")
    print("")
    
    # 测试健康检查
    if not test_health(args.endpoint):
        print("\n✗ 服务未运行或不可用")
        print("提示: 请先运行 ./start_ai_detector.sh start")
        sys.exit(1)
    
    # 处理图片
    image_path = args.image
    
    if args.create_test_image:
        test_img = create_test_image()
        if test_img:
            image_path = test_img
    
    if image_path:
        # 测试文件检测
        success1 = test_detect_file(args.endpoint, image_path)
        
        # 测试Base64检测
        success2 = test_detect_base64(args.endpoint, image_path)
        
        if success1 and success2:
            print("\n" + "=" * 60)
            print("✓ 所有测试通过")
            print("=" * 60)
        else:
            print("\n" + "=" * 60)
            print("✗ 部分测试失败")
            print("=" * 60)
            sys.exit(1)
    else:
        print("\n提示: 使用 --image 参数指定测试图片")
        print("或使用 --create-test-image 创建测试图片")
        print("\n示例:")
        print("  python3 test_ai_detector.py --image path/to/image.jpg")
        print("  python3 test_ai_detector.py --create-test-image")

if __name__ == '__main__':
    main()
