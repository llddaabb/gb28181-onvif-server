# AI模型下载指南

由于GitHub下载可能受网络限制，这里提供多种获取YOLOv8模型的方法。

## 方法1：自动下载脚本（推荐）

```bash
./download_ai_model.sh
```

选择模型类型（推荐选择 2 - YOLOv8s）

## 方法2：手动下载（网络受限时）

### 下载地址

选择以下任一模型下载：

**YOLOv8n** (~6MB, 最快)
- https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8n.onnx
- 或浏览器访问: https://github.com/ultralytics/assets/releases

**YOLOv8s** (~22MB, 推荐)
- https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8s.onnx

**YOLOv8m** (~52MB, 高精度)
- https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8m.onnx

### 保存路径

下载后保存到: `third-party/zlm/models/yolov8s.onnx`

```bash
# 创建目录
mkdir -p third-party/zlm/models

# 移动下载的文件
mv ~/Downloads/yolov8s.onnx third-party/zlm/models/
```

## 方法3：使用pip安装ultralytics并导出

```bash
# 安装ultralytics
pip3 install ultralytics

# 导出ONNX模型
python3 << 'PYTHON'
from ultralytics import YOLO

# 加载模型
model = YOLO('yolov8s.pt')

# 导出为ONNX
model.export(format='onnx')
print("模型已导出为 yolov8s.onnx")
PYTHON

# 移动到指定目录
mkdir -p third-party/zlm/models
mv yolov8s.onnx third-party/zlm/models/
```

## 方法4：使用国内镜像（如果可用）

某些国内镜像可能提供预训练模型下载，搜索：
- "YOLOv8 ONNX 模型 国内下载"
- 清华镜像、阿里云镜像等

## 验证模型文件

下载完成后验证文件：

```bash
# 检查文件大小（YOLOv8s约22MB）
ls -lh third-party/zlm/models/yolov8s.onnx

# 确认不是空文件
file third-party/zlm/models/yolov8s.onnx
```

输出应该类似：
```
-rw-r--r-- 1 user user 22M Dec 7 15:00 third-party/zlm/models/yolov8s.onnx
third-party/zlm/models/yolov8s.onnx: data
```

## 启动服务

模型下载完成后：

```bash
# 启动AI检测服务
./start_ai_detector.sh start

# 查看状态
./start_ai_detector.sh status

# 测试
./start_ai_detector.sh test
```

## 故障排查

### 文件为空或0字节

```bash
# 删除空文件
rm third-party/zlm/models/yolov8s.onnx

# 重新下载
./download_ai_model.sh
```

### 下载中断

使用wget的断点续传：
```bash
wget -c https://github.com/ultralytics/assets/releases/download/v8.3.0/yolov8s.onnx \
  -O third-party/zlm/models/yolov8s.onnx
```

### 无法访问GitHub

- 使用方法3（pip安装后导出）
- 寻找国内镜像
- 使用代理/VPN
