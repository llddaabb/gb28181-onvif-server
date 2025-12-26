#!/usr/bin/env python3
"""
使用 onnxsim 简化和转换模型
"""

import sys
import onnx
from onnxsim import simplify

def simplify_model(input_path, output_path):
    """简化 ONNX 模型"""
    print(f"正在加载模型: {input_path}")
    model = onnx.load(input_path)
    
    print(f"原始模型 opset 版本: {model.opset_import[0].version}")
    
    print("正在简化模型...")
    model_simp, check = simplify(model)
    
    if not check:
        print("⚠️ 警告: 模型简化可能不完整")
    
    print(f"保存简化后的模型: {output_path}")
    onnx.save(model_simp, output_path)
    
    print("✓ 简化完成")
    print(f"新模型 opset 版本: {model_simp.opset_import[0].version}")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("用法: python3 simplify_model.py <input_model> <output_model>")
        sys.exit(1)
    
    input_model = sys.argv[1]
    output_model = sys.argv[2]
    
    try:
        simplify_model(input_model, output_model)
    except Exception as e:
        print(f"✗ 简化失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
