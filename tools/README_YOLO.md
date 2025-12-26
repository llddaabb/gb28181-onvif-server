# YOLOv8 检测服务

这个 Python 脚本提供 HTTP API 接口，供 GB28181 服务器调用进行目标检测。

## 安装依赖

```bash
# 创建虚拟环境（推荐）
python3 -m venv venv
source venv/bin/activate  # Linux/Mac
# 或 venv\Scripts\activate  # Windows

# 安装依赖
pip install -r tools/requirements.txt
```

## 准备模型

### 方式 1: 自动下载
```bash
python tools/yolo_server.py --download
```

首次运行会自动下载 YOLOv8n 模型（约 6MB）。

### 方式 2: 手动下载
从 [Ultralytics](https://github.com/ultralytics/assets/releases) 下载模型：
- yolov8n.pt (6.3 MB) - 最快，准确度较低
- yolov8s.pt (21.5 MB) - 平衡
- yolov8m.pt (49.7 MB) - 更准确
- yolov8l.pt (83.7 MB) - 很准确
- yolov8x.pt (131.7 MB) - 最准确，最慢

将模型文件放到 `models/` 目录。

## 使用方法

### 基本启动
```bash
# 使用默认设置（CPU，端口 8000）
python tools/yolo_server.py

# 自动查找并使用本地模型
python tools/yolo_server.py

# 如果没有模型，自动下载
python tools/yolo_server.py --download
```

### 指定模型
```bash
# 使用特定模型
python tools/yolo_server.py --model models/yolov8s.pt

# 使用 nano 模型（最快）
python tools/yolo_server.py --model models/yolov8n.pt

# 使用 large 模型（更准确）
python tools/yolo_server.py --model models/yolov8l.pt
```

### 使用 GPU 加速
```bash
# 使用 CUDA（需要安装 CUDA 和 cuDNN）
python tools/yolo_server.py --device cuda

# 使用特定 GPU
python tools/yolo_server.py --device 0
```

### 自定义配置
```bash
python tools/yolo_server.py \
    --model models/yolov8s.pt \
    --device cuda \
    --host 0.0.0.0 \
    --port 8000 \
    --confidence 0.5
```

## 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--model` | 自动查找 | 模型文件路径 |
| `--device` | cpu | 运行设备 (cpu/cuda/0/1/2/3) |
| `--host` | 0.0.0.0 | 监听地址 |
| `--port` | 8000 | 监听端口 |
| `--confidence` | 0.5 | 默认置信度阈值 (0.0-1.0) |
| `--download` | - | 自动下载模型（如果不存在） |

## API 接口

### 检测接口
```
POST /detect
Content-Type: image/jpeg
X-Confidence-Threshold: 0.5 (可选)

Body: 图像二进制数据（JPEG）
```

响应示例：
```json
{
  "success": true,
  "has_person": true,
  "person_count": 2,
  "confidence": 0.85,
  "boxes": [
    {
      "x1": 100.5,
      "y1": 200.3,
      "x2": 300.7,
      "y2": 500.9,
      "confidence": 0.85,
      "class": "person"
    }
  ]
}
```

### 健康检查
```
GET /health
```

响应示例：
```json
{
  "status": "ok",
  "model": "yolov8n.pt",
  "device": "cpu"
}
```

## 配置 GB28181 服务器

在 `configs/config.yaml` 中配置：

```yaml
ai:
  enabled: true
  detector_type: http
  api_endpoint: "http://localhost:8000/detect"
  confidence: 0.5
  iou_threshold: 0.45
  detect_interval: 2
  record_delay: 3
  min_record_time: 10
```

或使用自动模式：
```yaml
ai:
  enabled: true
  detector_type: auto  # 优先 ONNX，失败后使用 HTTP
  api_endpoint: "http://localhost:8000/detect"
  model_path: ./models/yolov8n.onnx
```

## 测试

### 使用 curl 测试
```bash
# 测试健康检查
curl http://localhost:8000/health

# 测试检测（使用测试图像）
curl -X POST \
  -H "Content-Type: image/jpeg" \
  -H "X-Confidence-Threshold: 0.5" \
  --data-binary "@test_image.jpg" \
  http://localhost:8000/detect
```

### 使用 Python 测试
```python
import requests

# 健康检查
response = requests.get('http://localhost:8000/health')
print(response.json())

# 检测
with open('test_image.jpg', 'rb') as f:
    response = requests.post(
        'http://localhost:8000/detect',
        data=f.read(),
        headers={
            'Content-Type': 'image/jpeg',
            'X-Confidence-Threshold': '0.5'
        }
    )
    print(response.json())
```

## 性能建议

### CPU 模式
- 使用 yolov8n.pt（最快）
- 预期检测时间：0.5-2 秒/帧

### GPU 模式
- 需要安装 CUDA 和 cuDNN
- 使用 yolov8s.pt 或更大的模型
- 预期检测时间：0.05-0.2 秒/帧

## 生产部署

### 使用 Gunicorn
```bash
pip install gunicorn

gunicorn -w 4 -b 0.0.0.0:8000 \
  --timeout 120 \
  --preload \
  'tools.yolo_server:app'
```

### 使用 Docker
创建 Dockerfile：
```dockerfile
FROM python:3.10-slim

WORKDIR /app

# 安装依赖
COPY tools/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制代码
COPY tools/yolo_server.py .

# 下载模型
RUN python -c "from ultralytics import YOLO; YOLO('yolov8n.pt')"

EXPOSE 8000

CMD ["python", "yolo_server.py", "--host", "0.0.0.0", "--port", "8000"]
```

构建运行：
```bash
docker build -t yolo-server .
docker run -p 8000:8000 yolo-server
```

### 使用 systemd 服务
创建 `/etc/systemd/system/yolo-server.service`：
```ini
[Unit]
Description=YOLOv8 Detection Service
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/yolo-server
Environment="PATH=/opt/yolo-server/venv/bin"
ExecStart=/opt/yolo-server/venv/bin/python tools/yolo_server.py
Restart=always

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable yolo-server
sudo systemctl start yolo-server
sudo systemctl status yolo-server
```

## 故障排除

### 问题：导入 ultralytics 失败
```bash
pip install --upgrade ultralytics
```

### 问题：CUDA 不可用
```bash
# 检查 PyTorch CUDA 支持
python -c "import torch; print(torch.cuda.is_available())"

# 重新安装 PyTorch with CUDA
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu118
```

### 问题：内存不足
- 使用更小的模型 (yolov8n.pt)
- 减少并发请求
- 增加系统内存

### 问题：检测速度慢
- 使用 GPU 加速
- 使用更小的模型
- 降低输入图像分辨率

## 日志

服务运行时会输出检测日志：
```
2025/12/25 14:30:00 [INFO] 正在加载模型: models/yolov8n.pt
2025/12/25 14:30:02 [INFO] ✓ 模型加载成功: models/yolov8n.pt
2025/12/25 14:30:02 [INFO] ✓ 使用设备: cpu
2025/12/25 14:30:02 [INFO] ====================================
2025/12/25 14:30:02 [INFO] YOLOv8 检测服务
2025/12/25 14:30:02 [INFO] ====================================
2025/12/25 14:30:02 [INFO] 服务已启动，等待请求...
2025/12/25 14:30:15 [INFO] ✓ 检测到 2 个人，置信度: 0.85
```
