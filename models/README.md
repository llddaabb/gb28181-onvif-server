# AI模型文件

## YOLOv8s ONNX模型

由于模型文件较大（约43MB），没有包含在Git仓库中。

### 下载方法

#### 方法1：自动下载（推荐）

```bash
./download_ai_model.sh
```

#### 方法2：手动导出

```bash
pip3 install ultralytics
python3 -c "from ultralytics import YOLO; YOLO('yolov8s.pt').export(format='onnx')"
mv yolov8s.onnx models/
```

#### 方法3：从Release下载

访问 [Releases](https://github.com/llddaabb/gb28181-onvif-server/releases) 页面下载预编译的模型文件。

### 模型信息

- **模型名称**: YOLOv8s
- **格式**: ONNX
- **大小**: ~43MB
- **用途**: 人员检测（person detection）
- **输入**: 320x320 或 640x640 RGB图像
- **输出**: 检测框 + 置信度

### 验证模型

```bash
# 检查模型文件
ls -lh models/yolov8s.onnx

# 测试AI检测服务
./start_ai_detector.sh test
```

### 其他模型

如果需要使用其他YOLO模型：

```python
from ultralytics import YOLO

# YOLOv8n (更快，精度较低)
YOLO('yolov8n.pt').export(format='onnx')

# YOLOv8m (中等)
YOLO('yolov8m.pt').export(format='onnx')

# YOLOv8l (更精确，更慢)
YOLO('yolov8l.pt').export(format='onnx')
```

然后修改环境变量：

```bash
export AI_MODEL_PATH=models/yolov8n.onnx
./start_ai_detector.sh start
```
