# AI智能录像检测器

基于 YOLOv8 的人员检测服务，用于智能录像功能。

## 功能特性

- ✅ 基于 ONNX 的 YOLOv8 人员检测
- ✅ HTTP REST API 接口
- ✅ 支持图片文件和 Base64 编码
- ✅ 可配置的检测置信度
- ✅ 自动健康检查
- ✅ 日志记录

## 快速开始

### 1. 安装依赖

```bash
# 运行依赖安装脚本
./setup_ai_detector.sh
```

或手动安装：

```bash
pip3 install flask opencv-python numpy onnxruntime requests pillow
```

### 2. 下载模型文件

下载 YOLOv8s ONNX 模型：

```bash
# 创建模型目录
mkdir -p third-party/zlm/models

# 下载模型（约 22MB）
wget https://github.com/ultralytics/assets/releases/download/v0.0.0/yolov8s.onnx \
  -O third-party/zlm/models/yolov8s.onnx
```

或使用其他 YOLOv8 模型：
- `yolov8n.onnx` - 最小模型 (6MB)
- `yolov8m.onnx` - 中等模型 (52MB)
- `yolov8l.onnx` - 大型模型 (88MB)

### 3. 启动服务

```bash
# 使用默认配置启动
./start_ai_detector.sh start

# 查看服务状态
./start_ai_detector.sh status

# 查看日志
./start_ai_detector.sh logs
```

### 4. 配置主服务

编辑 `configs/config.yaml`：

```yaml
AI:
  Enable: true
  APIEndpoint: http://localhost:8000
  Confidence: 0.5
  DetectInterval: 2
  RecordDelay: 10
  StopDelay: 30
```

### 5. 测试检测

```bash
# 运行测试
./start_ai_detector.sh test

# 或手动测试
curl http://localhost:8000/health
```

## 使用方法

### 管理脚本命令

```bash
./start_ai_detector.sh {start|stop|restart|status|logs|test}
```

**命令说明：**
- `start` - 启动 AI 检测服务
- `stop` - 停止 AI 检测服务
- `restart` - 重启 AI 检测服务
- `status` - 查看服务状态和配置
- `logs` - 实时查看日志（Ctrl+C 退出）
- `test` - 测试检测功能

### 环境变量配置

启动时可以通过环境变量自定义配置：

```bash
# 自定义端口
AI_DETECTOR_PORT=8001 ./start_ai_detector.sh start

# 自定义置信度
AI_CONFIDENCE=0.6 ./start_ai_detector.sh start

# 自定义模型路径
AI_MODEL_PATH=/path/to/model.onnx ./start_ai_detector.sh start

# 自定义输入尺寸
AI_INPUT_SIZE=640 ./start_ai_detector.sh start

# 组合使用
AI_DETECTOR_PORT=8001 AI_CONFIDENCE=0.7 ./start_ai_detector.sh start
```

**可用环境变量：**

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `AI_DETECTOR_PORT` | HTTP 服务端口 | 8000 |
| `AI_MODEL_PATH` | ONNX 模型文件路径 | third-party/zlm/models/yolov8s.onnx |
| `AI_CONFIDENCE` | 检测置信度阈值 (0.0-1.0) | 0.5 |
| `AI_INPUT_SIZE` | 输入图像尺寸 | 320 |
| `AI_DEVICE` | 运行设备 (cpu/cuda) | cpu |
| `AI_WORKERS` | 工作线程数 | 2 |

## API 接口

### 健康检查

```bash
GET /health
```

响应：
```json
{
  "status": "ok",
  "model": "third-party/zlm/models/yolov8s.onnx",
  "confidence": 0.5
}
```

### 人员检测

```bash
POST /detect
Content-Type: multipart/form-data

image: <图片文件>
```

或使用 Base64：

```bash
POST /detect
Content-Type: application/json

{
  "image_base64": "<base64编码的图片>"
}
```

响应：
```json
{
  "success": true,
  "count": 2,
  "detections": [
    {
      "label": "person",
      "confidence": 0.87,
      "bbox": [120, 80, 250, 400]
    },
    {
      "label": "person",
      "confidence": 0.92,
      "bbox": [350, 100, 480, 420]
    }
  ]
}
```

### 使用示例

**Python 示例：**

```python
import requests
import base64

# 读取图片
with open('test.jpg', 'rb') as f:
    img_data = f.read()
    img_base64 = base64.b64encode(img_data).decode('utf-8')

# 发送检测请求
response = requests.post(
    'http://localhost:8000/detect',
    json={'image_base64': img_base64}
)

result = response.json()
print(f"检测到 {result['count']} 个人")
```

**curl 示例：**

```bash
# 使用文件上传
curl -X POST -F "image=@test.jpg" http://localhost:8000/detect

# 使用 Base64
IMAGE_BASE64=$(base64 -w 0 test.jpg)
curl -X POST \
  -H "Content-Type: application/json" \
  -d "{\"image_base64\":\"$IMAGE_BASE64\"}" \
  http://localhost:8000/detect
```

## 系统集成

AI 检测服务会自动与主系统集成：

1. **启动检测服务**
   ```bash
   ./start_ai_detector.sh start
   ```

2. **启动主服务**
   ```bash
   ./start.sh
   ```

3. **在前端启用 AI 录像**
   - 进入"系统设置"页面
   - 在"AI智能录像"部分启用
   - 配置检测参数
   - 保存配置

4. **开始 AI 录像**
   - 进入"通道管理"页面
   - 点击通道的"AI录像"按钮
   - 系统会自动检测视频流中的人员
   - 有人时自动开始录像，无人时自动停止

## 性能优化

### 1. 选择合适的模型

- **yolov8n.onnx** (6MB) - 最快，适合低性能设备
- **yolov8s.onnx** (22MB) - 推荐，性能和精度平衡
- **yolov8m.onnx** (52MB) - 高精度，需要更多资源
- **yolov8l.onnx** (88MB) - 最高精度，需要强大硬件

### 2. 调整输入尺寸

```bash
# 更小的输入尺寸 = 更快的检测速度
AI_INPUT_SIZE=320 ./start_ai_detector.sh start  # 快速
AI_INPUT_SIZE=640 ./start_ai_detector.sh start  # 精确
```

### 3. 调整置信度

```bash
# 较高的置信度 = 较少的误报
AI_CONFIDENCE=0.7 ./start_ai_detector.sh start
```

### 4. 使用 GPU（如果可用）

```bash
# 安装 GPU 版本的 onnxruntime
pip3 install onnxruntime-gpu

# 启动时指定使用 GPU
AI_DEVICE=cuda ./start_ai_detector.sh start
```

## 故障排查

### 服务无法启动

1. 检查依赖：
   ```bash
   python3 -c "import flask, cv2, numpy, onnxruntime"
   ```

2. 检查模型文件：
   ```bash
   ls -lh third-party/zlm/models/yolov8s.onnx
   ```

3. 查看日志：
   ```bash
   ./start_ai_detector.sh logs
   ```

### 检测结果不准确

1. 调整置信度阈值（在 config.yaml 或环境变量中）
2. 尝试更大的模型文件
3. 调整检测间隔（DetectInterval）

### 端口已被占用

```bash
# 使用其他端口
AI_DETECTOR_PORT=8001 ./start_ai_detector.sh start

# 更新 config.yaml
AI:
  APIEndpoint: http://localhost:8001
```

### 内存占用过高

1. 使用更小的模型（yolov8n.onnx）
2. 减小输入尺寸（AI_INPUT_SIZE=320）
3. 增加检测间隔（DetectInterval）

## 日志文件

- **服务日志**: `logs/ai_detector.log`
- **主系统日志**: `logs/debug.log`

## 参考资料

- [YOLOv8 官方文档](https://docs.ultralytics.com/)
- [ONNX Runtime](https://onnxruntime.ai/)
- [Flask 文档](https://flask.palletsprojects.com/)

## 许可证

本项目遵循主项目的许可证。
