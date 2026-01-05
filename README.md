# GB28181/ONVIF 视频监控服务器

集成 GB28181、ONVIF 协议与 ZLMediaKit 流媒体服务器的视频监控管理平台，支持实时预览、录像回放、AI 检测等功能。

## 特性

- ✅ **GB28181 国标协议**：设备注册、实时预览、PTZ 控制、录像回放
- ✅ **ONVIF 协议**：设备发现、媒体流管理、多接口支持
- ✅ **流媒体服务**：集成 ZLMediaKit，支持 RTSP/RTMP/HLS/WebRTC
- ✅ **AI 检测**：基于 YOLOv8 的实时目标检测（可选）
- ✅ **Web 管理界面**：现代化的 Vue3 前端界面
- ✅ **跨平台支持**：Linux (amd64/arm64), Windows, macOS

## 快速开始

### 方式一：使用预编译发布包（推荐）

1. **下载发布包**
   ```bash
   # 下载对应平台的发布包
   # gb28181-server-linux-amd64.tar.gz
   # gb28181-server-linux-arm64.tar.gz
   # gb28181-server-windows-amd64.zip
   # gb28181-server-darwin-arm64.tar.gz
   ```

2. **解压并配置**
   ```bash
   tar -xzf gb28181-server-linux-amd64.tar.gz
   cd gb28181-server-linux-amd64
   
   # 编辑配置文件
   vim configs/config.yaml
   ```

3. **启动服务**
   ```bash
   # Linux/macOS
   ./gb28181-server
   
   # Windows
   gb28181-server.exe
   ```

4. **访问管理界面**
   ```
   http://localhost:9080
   ```

### 方式二：从源码构建

#### 前置要求

- Go 1.21+
- Node.js 18+ 和 npm（用于构建前端）
- CMake 3.10+（用于构建 ZLMediaKit）
- GCC/G++ 或 Clang

#### 构建步骤

1. **克隆仓库**
   ```bash
   git clone <repository-url>
   cd zpip
   ```

2. **完整构建（包含 ZLM）**
   ```bash
   # 编译 ZLMediaKit + 前端 + 服务器
   make build-all
   
   # 或分步构建
   make build-zlm        # 编译 ZLMediaKit
   make build-frontend   # 编译前端
   make build-server     # 编译服务器
   ```

3. **快速构建（仅服务器，使用外部 ZLM）**
   ```bash
   make build-quick
   ```

4. **运行**
   ```bash
   ./dist/gb28181-server
   ```

## 配置说明

### 主配置文件：configs/config.yaml

```yaml
# GB28181 配置
GB28181:
  SipIP: "0.0.0.0"              # SIP 监听地址
  SipPort: 5060                 # SIP 端口
  Realm: "3402000000"           # SIP Realm（必须修改）
  ServerID: "34020000002000000001"  # 服务器 ID（必须修改）
  LocalIP: "192.168.1.100"      # 本机 IP（自动检测）

# ONVIF 配置
ONVIF:
  MediaPortRange: "8000-9000"   # 媒体端口范围
  EnableCheck: true             # 启用设备检查

# API 服务配置
API:
  Host: "0.0.0.0"
  Port: 9080                    # Web 管理端口

# ZLMediaKit 配置
ZLM:
  UseEmbedded: true             # 使用嵌入式 ZLM
  AutoRestart: true             # 自动重启
  API:
    Port: 10080                 # ZLM API 端口
    Secret: "your-secret-key"   # API 密钥（必须修改！）
  RTSP:
    Port: 554                   # RTSP 端口
  RTMP:
    Port: 1935                  # RTMP 端口
  HTTP:
    Port: 10080                 # HTTP 端口

# AI 检测配置（可选）
AI:
  Enabled: false                # 启用 AI 检测
  Backend: "onnx"               # 后端：onnx/embedded
  ModelPath: "models/yolov8s.onnx"
  ConfidenceThreshold: 0.5
```

详细配置说明请参考：[配置文档](docs/CONFIGURATION.md)

## 使用方法

### 1. GB28181 设备接入

1. **配置设备**
   - SIP 服务器地址：`<服务器IP>:5060`
   - SIP 服务器 ID：`34020000002000000001`
   - SIP Realm：`3402000000`
   - 设备 ID：`34020000001310000001`（必须以 Realm 开头）

2. **查看设备状态**
   ```bash
   curl http://localhost:9080/api/gb28181/devices
   ```

3. **实时预览**
   - Web 界面：访问 `http://localhost:9080`，在设备列表中点击"预览"
   - API：`POST /api/gb28181/devices/{deviceId}/preview`

4. **录像回放**
   - Web 界面：在设备详情中选择时间范围，点击"回放"
   - API：`POST /api/gb28181/devices/{deviceId}/playback`

详细说明：[GB28181 录像回放指南](docs/GB28181_DEVICE_RECORDING_PLAYBACK_GUIDE.md)

### 2. ONVIF 设备接入

1. **自动发现**
   ```bash
   curl -X POST http://localhost:9080/api/onvif/discover
   ```

2. **手动添加**
   ```bash
   curl -X POST http://localhost:9080/api/onvif/devices \
     -H "Content-Type: application/json" \
     -d '{
       "host": "192.168.1.100",
       "username": "admin",
       "password": "admin123"
     }'
   ```

3. **获取流地址**
   ```bash
   curl http://localhost:9080/api/onvif/devices/{deviceId}/profiles
   ```

详细说明：[ONVIF 多接口指南](docs/ONVIF_MULTI_INTERFACE.md)

### 3. AI 检测（可选）

1. **启用 AI 检测**
   ```yaml
   # configs/config.yaml
   AI:
     Enabled: true
     Backend: "onnx"
     ModelPath: "models/yolov8s.onnx"
   ```

2. **下载模型**
   ```bash
   # YOLOv8s ONNX 模型
   wget https://github.com/ultralytics/assets/releases/download/v0.0.0/yolov8s.onnx \
     -O models/yolov8s.onnx
   ```

3. **启动检测**
   ```bash
   curl -X POST http://localhost:9080/api/ai/channels/{channelId}/start \
     -H "Content-Type: application/json" \
     -d '{
       "enableRecording": true,
       "outputDir": "recordings/ai"
     }'
   ```

详细说明：[YOLO 检测指南](tools/README_YOLO.md)

## 发布打包

### 打包当前平台

```bash
# 打包不含 ZLM（适用于已安装 ZLM 的环境）
make package

# 打包含 ZLM（完整独立包）
make package-with-zlm
```

生成的发布包位于：`dist/release/gb28181-server-<platform>.tar.gz`

### 跨平台编译

```bash
# Linux amd64
make build-linux-amd64

# Linux arm64
make build-linux-arm64

# Windows amd64
make build-windows-amd64

# macOS arm64 (Apple Silicon)
make build-darwin-arm64

# 编译所有平台
make build-all-platforms
```

### 发布包结构

```
gb28181-server-linux-amd64/
├── gb28181-server          # 主程序（已嵌入前端）
├── configs/
│   └── config.yaml         # 配置文件
├── logs/                   # 日志目录
├── recordings/             # 录像目录
└── README.md              # 使用说明
```

## 系统要求

### 运行环境

- **CPU**：x86_64 或 ARM64
- **内存**：至少 512MB（推荐 2GB+）
- **磁盘**：至少 1GB 可用空间（录像需要更多）
- **操作系统**：
  - Linux: Ubuntu 18.04+, Debian 10+, CentOS 7+
  - Windows: Windows 10+
  - macOS: 10.15+

### 网络端口

默认使用以下端口（可在配置文件中修改）：

- `5060/UDP`：GB28181 SIP 信令
- `9080/TCP`：Web 管理界面 + API
- `10080/TCP`：ZLM HTTP/API
- `554/TCP`：RTSP
- `1935/TCP`：RTMP
- `8000-9000/TCP`：RTP/RTCP 媒体传输
- `30000-30500/UDP`：RTP 接收端口（动态分配）

确保防火墙允许这些端口的访问。

## 目录结构

```
.
├── cmd/server/             # 服务器入口
├── internal/               # 内部模块
│   ├── gb28181/           # GB28181 协议实现
│   ├── onvif/             # ONVIF 协议实现
│   ├── zlm/               # ZLM 集成
│   ├── ai/                # AI 检测模块
│   └── api/               # REST API
├── configs/               # 配置文件
├── frontend/              # Vue3 前端
├── docs/                  # 文档
├── scripts/               # 构建脚本
├── logs/                  # 日志目录
├── recordings/            # 录像目录
└── dist/                  # 编译输出
```

## 常见问题

### 1. GB28181 设备注册失败

- 检查设备 ID 是否以配置的 Realm 开头
- 检查网络连接和防火墙设置
- 检查 SIP 端口（5060）是否被占用

### 2. 录像回放无流

- 检查设备是否支持录像回放
- 查看 ZLM 日志：`tail -f build/zlm-runtime/log/*.log`
- 确认 RTP 端口范围未被占用

### 3. ONVIF 设备发现不到

- 确保设备和服务器在同一网络
- 检查设备是否启用了 ONVIF 服务
- 尝试手动添加设备

### 4. 端口被占用

```bash
# 检查端口占用
sudo lsof -i :5060
sudo lsof -i :9080

# 修改配置文件中的端口
vim configs/config.yaml
```

## 性能优化

详细优化指南请参考：[配置优化文档](docs/CONFIG_OPTIMIZATION.md)

## 开发指南

### 开发模式运行

```bash
# 启动后端（不嵌入前端）
make build-quick
./dist/gb28181-server

# 启动前端开发服务器（另一个终端）
cd frontend
npm install
npm run dev
```

前端开发服务器：`http://localhost:5173`

### 调试日志

在 `configs/config.yaml` 中启用调试：

```yaml
Debug:
  Enabled: true
  LogLevel: "debug"
  Services:
    - "gb28181"    # GB28181 模块
    - "onvif"      # ONVIF 模块
    - "zlm"        # ZLM 集成
    - "ai"         # AI 检测
```

### 运行测试

```bash
go test ./...
```

## 文档

- [配置管理指南](docs/CONFIGURATION.md)
- [配置优化指南](docs/CONFIG_OPTIMIZATION.md)
- [GB28181 录像回放指南](docs/GB28181_DEVICE_RECORDING_PLAYBACK_GUIDE.md)
- [ONVIF 多接口指南](docs/ONVIF_MULTI_INTERFACE.md)
- [录像流回放指南](docs/RECORDING_STREAM_PLAYBACK.md)
- [YOLO 检测指南](tools/README_YOLO.md)

## 许可证

见 [LICENSE](LICENSE) 文件。

## 支持

如有问题或建议，请提交 Issue。
