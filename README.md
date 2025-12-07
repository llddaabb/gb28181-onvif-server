# GB28181/ONVIF 智能视频监控平台

基于 Go + ZLMediaKit 的企业级视频监控解决方案，支持 GB28181 和 ONVIF 协议，集成 AI 人员检测智能录像功能。

## ✨ 核心特性

### 📹 多协议支持
- **GB28181** - 完整的国标协议支持，设备注册、目录查询、实时预览
- **ONVIF** - 标准 ONVIF 协议，自动设备发现和管理
- **多格式流媒体** - RTSP、RTMP、HLS、FLV 多种流媒体协议

### 🤖 AI 智能录像
- **YOLOv8 人员检测** - 基于 ONNX Runtime 的高性能检测
- **智能触发录像** - 检测到人员时自动开始录像，节省存储空间
- **实时状态监控** - 检测统计、录像会话、存储时长追踪
- **灵活配置** - 可调节置信度阈值、检测频率等参数

### 🎯 录像管理
- **持久化录像** - 录像状态自动保存，断流恢复后自动重启
- **智能守护** - 10秒检测间隔，自动重启中断的录像任务
- **多种录像模式** - 支持手动录像和 AI 智能录像
- **录像回放** - 完整的录像查询、播放、下载功能

### 🌐 Web 管理界面
- **Vue 3 现代化界面** - TypeScript + Vite 构建
- **实时流预览** - 多通道实时视频预览
- **设备管理** - GB28181 和 ONVIF 设备统一管理
- **录像回放** - 时间轴选择、在线播放
- **系统监控** - ZLM 状态、AI 检测服务监控

## 🚀 快速开始

### 系统要求

- **操作系统**: Linux (Ubuntu 20.04+)
- **Go**: 1.19+
- **Python**: 3.10+
- **Node.js**: 16+

### 一键启动

```bash
# 克隆项目
git clone <repository-url>
cd zpip

# 启动所有服务
./start.sh start
```

服务启动后访问：
- **Web 管理**: http://localhost:5173
- **API 接口**: http://localhost:9080
- **ZLM HTTP**: http://localhost:8080
- **AI 检测**: http://localhost:8001

### 详细安装步骤

#### 1. 后端服务

```bash
# 编译 Go 服务
go build -o server cmd/server/main.go

# 启动服务
./server -config ./configs/config.yaml
```

#### 2. ZLMediaKit

```bash
# 下载并设置 ZLM
./download_zlm.sh
./setup_zlm.sh

# 启动 ZLM
./start_zlm.sh
```

#### 3. AI 检测服务

```bash
# 安装依赖
./setup_ai_detector.sh

# 下载模型
./download_ai_model.sh

# 启动 AI 检测器
./start_ai_detector.sh start
```

#### 4. 前端界面

```bash
cd frontend

# 安装依赖
npm install

# 开发模式
npm run dev

# 生产构建
npm run build
```

## 📖 使用指南

### AI 智能录像配置

1. **启动 AI 录像**
```bash
curl -X POST http://localhost:9080/api/ai/recording/start \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": "34020000001320000132",
    "app": "live",
    "stream": "channel1"
  }'
```

2. **查看 AI 录像状态**
```bash
# 查看所有通道
curl http://localhost:9080/api/ai/recording/status/all

# 查看单个通道
curl http://localhost:9080/api/ai/recording/status?channel_id=34020000001320000132
```

3. **停止 AI 录像**
```bash
curl -X POST http://localhost:9080/api/ai/recording/stop \
  -H "Content-Type: application/json" \
  -d '{"channel_id": "34020000001320000132"}'
```

### 通道管理

```bash
# 查看所有通道（包含 AI 录像状态）
curl http://localhost:9080/api/channel/list

# 查看单个通道详情
curl http://localhost:9080/api/channel/34020000001320000132

# 添加通道
curl -X POST http://localhost:9080/api/channel/add \
  -H "Content-Type: application/json" \
  -d '{
    "channelId": "test001",
    "channelName": "测试通道",
    "deviceType": "onvif"
  }'
```

### GB28181 设备管理

```bash
# 查看已注册设备
curl http://localhost:9080/api/gb28181/devices

# 查看设备通道
curl http://localhost:9080/api/gb28181/devices/{deviceId}/channels

# 开始预览
curl -X POST http://localhost:9080/api/gb28181/devices/{deviceId}/channels/{channelId}/preview/start
```

## 🔧 配置说明

### config.yaml

```yaml
Server:
  Host: "0.0.0.0"
  Port: 9080
  LogLevel: "info"

GB28181:
  Enable: true
  ServerID: "34020000002000000001"
  ServerDomain: "3402000000"
  ServerIP: "192.168.1.100"
  ServerPort: 5060

ONVIF:
  Enable: true
  DiscoveryInterval: 60

ZLM:
  Embedded: true
  HTTPPort: 8080
  RTSPPort: 8554
  RTMPPort: 1935

AI:
  Enable: true
  APIEndpoint: "http://localhost:8001/detect"
  Confidence: 0.5
  DetectInterval: 2
```

### AI 检测器配置

```bash
# 环境变量配置
export AI_DETECTOR_PORT=8001
export AI_MODEL_PATH=models/yolov8s.onnx
export AI_CONFIDENCE=0.5
export AI_INPUT_SIZE=320

# 启动
./start_ai_detector.sh start
```

## 📁 项目结构

```
zpip/
├── cmd/
│   └── server/          # 主服务入口
├── internal/
│   ├── api/            # API 服务层
│   ├── gb28181/        # GB28181 协议实现
│   ├── onvif/          # ONVIF 协议实现
│   ├── zlm/            # ZLM 管理
│   ├── ai/             # AI 检测和录像
│   └── config/         # 配置管理
├── frontend/           # Vue 3 前端
│   ├── src/
│   │   ├── views/     # 页面组件
│   │   └── router/    # 路由配置
│   └── public/
├── configs/            # 配置文件
├── models/             # AI 模型文件
├── third-party/        # 第三方组件
│   └── zlm/           # ZLMediaKit
├── logs/              # 日志目录
├── start.sh           # 统一启动脚本
├── start_ai_detector.sh  # AI 检测器管理
└── README.md
```

## 🛠️ API 接口

### 通道管理
- `GET /api/channel/list` - 获取通道列表
- `GET /api/channel/{id}` - 获取通道详情
- `POST /api/channel/add` - 添加通道
- `DELETE /api/channel/{id}` - 删除通道

### AI 录像
- `POST /api/ai/recording/start` - 启动 AI 录像
- `POST /api/ai/recording/stop` - 停止 AI 录像
- `GET /api/ai/recording/status` - 查看录像状态
- `GET /api/ai/recording/status/all` - 查看所有录像状态

### GB28181
- `GET /api/gb28181/devices` - 获取设备列表
- `GET /api/gb28181/devices/{id}/channels` - 获取设备通道
- `POST /api/gb28181/devices/{id}/channels/{channelId}/preview/start` - 开始预览
- `POST /api/gb28181/devices/{id}/channels/{channelId}/preview/stop` - 停止预览

### ONVIF
- `GET /api/onvif/devices` - 获取 ONVIF 设备
- `POST /api/onvif/discover` - 手动发现设备
- `POST /api/onvif/devices/{id}/preview/start` - 开始预览

### 录像管理
- `GET /api/recording/query` - 查询录像
- `GET /api/recording/{id}` - 获取录像详情
- `GET /api/recording/{id}/download` - 下载录像

## 🎮 管理脚本

### start.sh - 主服务管理
```bash
./start.sh start    # 启动所有服务
./start.sh stop     # 停止所有服务
./start.sh restart  # 重启所有服务
./start.sh status   # 查看服务状态
```

### start_ai_detector.sh - AI 检测器管理
```bash
./start_ai_detector.sh start   # 启动
./start_ai_detector.sh stop    # 停止
./start_ai_detector.sh restart # 重启
./start_ai_detector.sh status  # 状态
./start_ai_detector.sh test    # 测试
./start_ai_detector.sh logs    # 查看日志
```

## 📊 AI 检测说明

### 支持的检测模型
- **YOLOv8s** - 平衡性能和精度（推荐）
- **YOLOv8n** - 轻量级，速度更快
- **YOLOv8m/l/x** - 更高精度，需要更多资源

### 检测流程
1. 从视频流抓取帧（可配置间隔）
2. 图像预处理和归一化
3. ONNX Runtime 推理
4. 后处理和 NMS
5. 检测到人员时触发录像

### 性能优化
- 可调整检测间隔（默认 2 秒）
- 可调整输入尺寸（默认 320x320）
- 支持 CPU 和 GPU 推理
- 自动批处理优化

## 🔒 安全建议

1. **修改默认密钥**
   - 修改 `configs/config.yaml` 中的 ZLM Secret
   - 修改 GB28181 ServerID 和域

2. **网络隔离**
   - 生产环境使用防火墙限制端口访问
   - 仅开放必要的端口

3. **访问控制**
   - 启用 API 认证
   - 使用 HTTPS/TLS 加密通信

## 🐛 故障排查

### 服务启动失败
```bash
# 查看日志
tail -100 logs/server.log
tail -100 logs/ai_detector.log

# 检查端口占用
lsof -i :9080
lsof -i :8080
lsof -i :5060
```

### AI 检测不工作
```bash
# 检查模型文件
ls -lh models/yolov8s.onnx

# 测试 AI 服务
curl http://localhost:8001/health

# 查看 AI 日志
./start_ai_detector.sh logs
```

### 录像失败
```bash
# 检查 ZLM 状态
curl http://localhost:8080/index/api/getServerConfig?secret=<your-secret>

# 查看录像目录权限
ls -la third-party/zlm/www/record/

# 检查磁盘空间
df -h
```

## 📝 更新日志

### v1.0.0 (2025-12-07)
- ✨ 初始版本发布
- ✅ GB28181 协议完整支持
- ✅ ONVIF 设备自动发现
- ✅ YOLOv8 AI 人员检测
- ✅ 智能录像触发机制
- ✅ 持久化录像管理
- ✅ Web 管理界面
- ✅ 统一服务管理脚本

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

- [ZLMediaKit](https://github.com/ZLMediaKit/ZLMediaKit) - 强大的流媒体服务器
- [Ultralytics YOLOv8](https://github.com/ultralytics/ultralytics) - 先进的目标检测模型
- [ONNX Runtime](https://onnxruntime.ai/) - 高性能推理引擎

## 📮 联系方式

- 项目主页: <https://github.com/yourusername/zpip>
- Issue 跟踪: <https://github.com/yourusername/zpip/issues>

---

⭐ 如果这个项目对你有帮助，请给个 Star！
