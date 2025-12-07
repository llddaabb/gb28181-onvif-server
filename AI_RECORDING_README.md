# AI智能录像功能

## 功能概述

AI智能录像功能通过AI视觉分析自动检测视频流中是否有人，实现按需录像，避免全时录像浪费存储空间。

## 核心特性

### 1. 智能检测模式
- **人形检测模式** (person): 检测到人时自动开始录像，无人时自动停止
- **移动检测模式** (motion): 检测到移动物体时录像（待实现）
- **连续录像模式** (continuous): 始终录像
- **手动录像模式** (manual): 仅手动控制

### 2. 资源优化策略
- 使用轻量级YOLOv8-nano模型（最小尺寸320x320）
- 默认检测间隔2秒，平衡实时性和CPU占用
- 支持HTTP API调用外部AI服务，减少本地资源消耗
- 单线程池设计，限制并发检测数量

### 3. 智能录像控制
- **录像延迟**: 检测到人后继续录制10秒（可配置）
- **最小录像时长**: 至少录制5秒，避免碎片文件（可配置）
- **自动启停**: 根据检测结果自动控制录像开关

## 架构设计

```
┌─────────────────┐
│   视频流        │
│  (RTSP/RTMP)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  FrameGrabber   │  ← 使用FFmpeg每2秒抓取一帧
│  (320x320)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  AI Detector    │  ← HTTP API调用外部AI服务
│  (Person Det)   │     或本地ONNX模型（待实现）
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ StreamRecorder  │  ← 检测逻辑 + 录像控制
│ (State Machine) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   ZLM录像       │  ← 调用现有录像API
│  (MP4/FMP4)     │
└─────────────────┘
```

## 配置说明

### config.yaml 配置

```yaml
AI:
    Enable: false                              # 是否启用AI录像功能
    APIEndpoint: http://localhost:8000/detect  # AI检测API地址
    ModelPath: ./models/yolov8n.onnx           # 本地模型路径（备用）
    Confidence: 0.5                            # 检测置信度阈值 (0.0-1.0)
    DetectInterval: 2                          # 检测间隔(秒)
    RecordDelay: 10                            # 录像延迟(秒)
    MinRecordTime: 5                           # 最小录像时长(秒)
```

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| Enable | false | 是否启用AI功能 |
| APIEndpoint | http://localhost:8000/detect | 外部AI服务地址 |
| ModelPath | ./models/yolov8n.onnx | 本地模型路径（未使用时可忽略） |
| Confidence | 0.5 | 置信度阈值，越高误报越少 |
| DetectInterval | 2 | 检测间隔，建议1-5秒 |
| RecordDelay | 10 | 检测到人后继续录多久 |
| MinRecordTime | 5 | 最小录像时长，避免太多小文件 |

## API接口

### 1. 启动AI录像

```bash
POST /api/ai/recording/start
Content-Type: application/json

{
    "channel_id": "34020000001320000001",
    "stream_url": "rtsp://192.168.1.100:554/stream",
    "mode": "person"  # person, motion, continuous, manual
}
```

响应：
```json
{
    "success": true,
    "channel_id": "34020000001320000001",
    "mode": "person"
}
```

### 2. 停止AI录像

```bash
POST /api/ai/recording/stop
Content-Type: application/json

{
    "channel_id": "34020000001320000001"
}
```

### 3. 查询单个通道状态

```bash
GET /api/ai/recording/status?channel_id=34020000001320000001
```

响应：
```json
{
    "success": true,
    "status": {
        "channel_id": "34020000001320000001",
        "mode": "person",
        "is_recording": true,
        "last_detect_time": "2025-12-07T14:30:00Z",
        "last_person_time": "2025-12-07T14:29:55Z",
        "stats": {
            "total_detections": 150,
            "person_detections": 45,
            "recording_sessions": 3,
            "total_record_time": "15m30s"
        }
    }
}
```

### 4. 查询所有通道状态

```bash
GET /api/ai/recording/status/all
```

### 5. 获取AI配置

```bash
GET /api/ai/config
```

### 6. 更新AI配置

```bash
PUT /api/ai/config
Content-Type: application/json

{
    "Enable": true,
    "Confidence": 0.6,
    "DetectInterval": 3
}
```

## 外部AI服务

### 服务要求

AI检测服务需要实现以下接口：

**请求:**
```
POST /detect
Content-Type: image/jpeg
X-Confidence-Threshold: 0.5

[JPEG图像数据]
```

**响应:**
```json
{
    "success": true,
    "has_person": true,
    "person_count": 2,
    "confidence": 0.85,
    "boxes": [
        {
            "x1": 100,
            "y1": 150,
            "x2": 300,
            "y2": 450,
            "confidence": 0.85,
            "class": "person"
        }
    ]
}
```

### 推荐AI服务

1. **自建服务**: 
   - 使用Python FastAPI + ultralytics
   - YOLOv8模型，支持CPU/GPU
   - 简单部署，资源可控

2. **云服务**:
   - 阿里云视觉智能开放平台
   - 腾讯云AI视觉
   - 百度AI开放平台

## 性能优化

### CPU/GPU选择策略

系统自动选择最优后端：

1. **优先HTTP API**: 调用外部服务，本地零负载
2. **GPU加速**: 如有CUDA/OpenCL，自动使用（需额外配置）
3. **CPU推理**: 使用轻量模型，限制线程数

### 资源占用估算

基于320x320输入、YOLOv8-nano模型：

| 模式 | CPU占用 | 内存占用 | GPU占用 |
|------|---------|---------|---------|
| HTTP API | <1% | <50MB | 0 |
| CPU推理 | 5-10% | 200MB | 0 |
| GPU推理 | <1% | 300MB | 500MB VRAM |

### 优化建议

1. **检测间隔**: 不需要实时检测，2-5秒足够
2. **输入尺寸**: 320x320足以检测人形，无需更高分辨率
3. **批处理**: 关闭批处理（batch_size=1），降低延迟
4. **模型选择**: YOLOv8n > YOLOv8s > YOLOv8m（越小越快）

## 使用场景

### 1. 家庭监控
- 只在有人活动时录像
- 节省存储空间（可减少80%录像）
- 快速检索有人的视频片段

### 2. 商业场所
- 营业时间智能录像
- 客流统计（通过person_count）
- 安防事件快速定位

### 3. 仓库/停车场
- 无人时不录像
- 异常入侵自动触发
- 长期存储成本降低

## 故障排查

### 1. AI功能未启动
检查：
- `config.yaml` 中 `AI.Enable` 是否为 `true`
- 日志中是否有 "AI录像管理器已初始化"

### 2. 检测失败
检查：
- AI服务是否可访问: `curl http://localhost:8000/detect`
- FFmpeg是否安装: `which ffmpeg`
- 流地址是否正确

### 3. 录像不停止
可能原因：
- RecordDelay 设置过长
- 误检测（降低Confidence阈值）
- 检测服务返回错误

### 4. CPU占用过高
优化方案：
- 增加 DetectInterval（如改为5秒）
- 使用HTTP API而非本地推理
- 减少并发检测通道数

## 未来计划

- [ ] 移动检测模式实现
- [ ] 本地ONNX模型推理支持
- [ ] WebUI配置界面
- [ ] 检测结果可视化（画框）
- [ ] 多类别检测（车辆、动物等）
- [ ] 行为分析（跌倒、打架等）
- [ ] 人脸识别集成

## 技术栈

- **Go**: 主服务框架
- **FFmpeg**: 视频帧捕获
- **HTTP API**: AI检测接口
- **YOLO**: 人形检测模型（外部服务）
- **ZLMediaKit**: 录像存储

## 许可证

本功能遵循项目主许可证。
