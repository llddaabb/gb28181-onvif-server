#!/usr/bin/env python3
"""
将 ONNX 模型转换为指定的 Opset 版本
用法: python3 convert_onnx_opset.py <input_model> <output_model> <target_opset>
"""

import sys
import onnx
from onnx import version_converter

def convert_opset(input_path, output_path, target_opset=11):
    """转换 ONNX 模型到指定 opset 版本"""
    print(f"正在加载模型: {input_path}")
    model = onnx.load(input_path)
    
    print(f"原始模型 opset 版本: {model.opset_import[0].version}")
    print(f"目标 opset 版本: {target_opset}")
    
    print("正在转换...")
    converted_model = version_converter.convert_version(model, target_opset)
    
    print(f"保存转换后的模型: {output_path}")
    onnx.save(converted_model, output_path)
    
    print("✓ 转换完成")
    print(f"新模型 opset 版本: {converted_model.opset_import[0].version}")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("用法: python3 convert_onnx_opset.py <input_model> <output_model> [target_opset]")
        print("示例: python3 convert_onnx_opset.py yolov8s.onnx yolov8s_opset11.onnx 11")
        sys.exit(1)
    
    input_model = sys.argv[1]
    output_model = sys.argv[2]
    target_opset = int(sys.argv[3]) if len(sys.argv) > 3 else 11
    
    try:
        convert_opset(input_model, output_model, target_opset)
    except Exception as e:
        print(f"✗ 转换失败: {e}")
        sys.exit(1)
