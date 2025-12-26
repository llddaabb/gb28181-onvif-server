#!/usr/bin/env python3
"""
YOLOv8 检测服务测试客户端
"""

import sys
import requests
from pathlib import Path

def test_health(url: str = "http://localhost:8000"):
    """测试健康检查"""
    print("测试健康检查...")
    try:
        response = requests.get(f"{url}/health", timeout=5)
        if response.status_code == 200:
            data = response.json()
            print(f"✓ 服务正常运行")
            print(f"  模型: {data.get('model')}")
            print(f"  设备: {data.get('device')}")
            return True
        else:
            print(f"✗ 服务返回错误: {response.status_code}")
            return False
    except requests.exceptions.ConnectionError:
        print(f"✗ 无法连接到服务: {url}")
        print("  请确保 YOLOv8 服务正在运行")
        return False
    except Exception as e:
        print(f"✗ 错误: {e}")
        return False


def test_detect(image_path: str, url: str = "http://localhost:8000", confidence: float = 0.5):
    """测试检测接口"""
    print(f"\n测试检测接口...")
    print(f"  图像: {image_path}")
    print(f"  置信度: {confidence}")
    
    if not Path(image_path).exists():
        print(f"✗ 图像文件不存在: {image_path}")
        return False
    
    try:
        with open(image_path, 'rb') as f:
            response = requests.post(
                f"{url}/detect",
                data=f.read(),
                headers={
                    'Content-Type': 'image/jpeg',
                    'X-Confidence-Threshold': str(confidence)
                },
                timeout=30
            )
        
        if response.status_code != 200:
            print(f"✗ 服务返回错误: {response.status_code}")
            print(f"  {response.text}")
            return False
        
        data = response.json()
        
        if not data.get('success'):
            print(f"✗ 检测失败: {data.get('error')}")
            return False
        
        print(f"✓ 检测成功")
        print(f"  检测到人: {'是' if data.get('has_person') else '否'}")
        print(f"  人数: {data.get('person_count', 0)}")
        print(f"  最高置信度: {data.get('confidence', 0):.2f}")
        
        boxes = data.get('boxes', [])
        if boxes:
            print(f"  检测框数量: {len(boxes)}")
            for i, box in enumerate(boxes[:5], 1):  # 只显示前5个
                print(f"    [{i}] {box['class']}: {box['confidence']:.2f} "
                      f"({box['x1']:.0f},{box['y1']:.0f}) -> ({box['x2']:.0f},{box['y2']:.0f})")
        
        return True
    
    except Exception as e:
        print(f"✗ 错误: {e}")
        return False


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description='YOLOv8 检测服务测试客户端')
    parser.add_argument(
        '--url',
        type=str,
        default='http://localhost:8000',
        help='服务地址 (默认: http://localhost:8000)'
    )
    parser.add_argument(
        '--image',
        type=str,
        help='测试图像路径'
    )
    parser.add_argument(
        '--confidence',
        type=float,
        default=0.5,
        help='置信度阈值 (默认: 0.5)'
    )
    
    args = parser.parse_args()
    
    print("=" * 60)
    print("YOLOv8 检测服务测试")
    print("=" * 60)
    
    # 测试健康检查
    if not test_health(args.url):
        return 1
    
    # 测试检测
    if args.image:
        if not test_detect(args.image, args.url, args.confidence):
            return 1
    else:
        print("\n提示: 使用 --image 参数测试检测功能")
    
    print("\n" + "=" * 60)
    print("✓ 所有测试通过")
    print("=" * 60)
    
    return 0


if __name__ == '__main__':
    sys.exit(main())
