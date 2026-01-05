# GB28181/ONVIF 视频监控服务器 - 发布说明

## 版本信息

**发布日期**: 2026年1月5日

**发布包**: `gb28181-server-linux-amd64-with-zlm.tar.gz` (101 MB)

## 📦 包含内容

### 核心组件
- ✅ **gb28181-server** - 主程序（已嵌入 ZLM + 前端 + 依赖库）
- ✅ **ZLMediaKit** - 媒体流服务器（嵌入式）
- ✅ **前端界面** - Vue3 管理控制台（嵌入式）

### 配置与文档
- ✅ `configs/config.yaml` - 配置文件
- ✅ `README.md` - 完整使用说明
- ✅ `start.sh` - 完整启动脚本（含检查和库路径设置）
- ✅ `quick_start.sh` - 快速启动脚本

### AI 智能检测（可选）
- ✅ `models/yolov8s.onnx` - YOLOv8s ONNX 模型
- ✅ `models/yolov8s.opset21.onnx` - YOLOv8s opset21 版本

### 依赖库
- ✅ `lib/libonnxruntime.so.1.16.3` - ONNXRuntime 库
- ✅ `lib/libonnxruntime.so.1` - 符号链接（自动加载）
- ✅ `lib/libonnxruntime.so` - 符号链接（兼容性）

### 目录结构
```
gb28181-server-linux-amd64-with-zlm/
├── gb28181-server              # 主程序
├── start.sh                    # 完整启动脚本
├── quick_start.sh              # 快速启动脚本
├── README.md                   # 文档
├── configs/
│   └── config.yaml             # 配置文件
├── lib/                        # 依赖库
│   ├── libonnxruntime.so.1.16.3
│   ├── libonnxruntime.so.1     # 符号链接
│   └── libonnxruntime.so       # 符号链接
├── models/                     # AI 模型文件
│   ├── yolov8s.onnx
│   ├── yolov8s.opset21.onnx
│   └── README.md
├── logs/                       # 日志目录（自动创建）
└── recordings/                 # 录像目录（自动创建）
```

## 🚀 快速开始

### 1. 解压发布包

```bash
tar -xzf gb28181-server-linux-amd64-with-zlm.tar.gz
cd gb28181-server-linux-amd64-with-zlm
```

### 2. 配置服务器

编辑 `configs/config.yaml` 修改以下关键配置：

```yaml
GB28181:
  SipIP: "0.0.0.0"
  SipPort: 5060
  LocalIP: "192.168.1.100"      # 修改为服务器实际 IP
  Realm: "3402000000"           # 修改为实际 Realm
  ServerID: "34020000002000000001"  # 修改为实际 ID

API:
  Port: 9080                    # Web 管理端口

ZLM:
  API:
    Secret: "your-secret-key"   # 修改为强密码
    
AI:
  Enabled: false                # 是否启用 AI 检测
  Backend: "onnx"               # 使用 ONNX 后端
  ModelPath: "models/yolov8s.onnx"
```

### 3. 启动服务

**方式一：完整启动（推荐）**
```bash
./start.sh
```
- 自动检查配置文件
- 自动检查端口占用
- 自动设置库文件路径
- 自动创建必要目录

**方式二：快速启动**
```bash
./quick_start.sh
```
- 跳过检查，直接启动
- 自动设置库文件路径

### 4. 访问管理界面

打开浏览器访问：
```
http://localhost:9080
```

## 🔧 配置说明

### GB28181 协议配置

```yaml
GB28181:
  SipIP: "0.0.0.0"              # SIP 监听 IP
  SipPort: 5060                 # SIP 监听端口
  LocalIP: "192.168.1.100"      # 服务器内网 IP（自动检测）
  Realm: "3402000000"           # SIP Realm（必须修改！）
  ServerID: "34020000002000000001"  # 服务器 ID（必须修改！）
  HeartbeatInterval: 60         # 心跳间隔（秒）
  RegisterExpires: 3600         # 注册过期时间（秒）
```

**关键点**：
- 设备 ID 必须以 `Realm` 开头
- `LocalIP` 必须是服务器的实际网络地址
- 需要在防火墙中开放 5060/UDP 端口

### ONVIF 配置

```yaml
ONVIF:
  MediaPortRange: "8000-9000"   # RTP/RTCP 端口范围
  EnableCheck: true             # 启用设备健康检查
  CheckInterval: 120            # 检查间隔（秒）
```

### API 配置

```yaml
API:
  Host: "0.0.0.0"
  Port: 9080                    # Web 端口
  CorsAllowOrigins:
    - "*"
```

### ZLM 配置

```yaml
ZLM:
  UseEmbedded: true             # 使用嵌入式 ZLM
  AutoRestart: true             # 进程异常时自动重启
  MaxRestarts: 5                # 最大重启次数
  API:
    Port: 10080                 # ZLM API 端口
    Secret: "your-secret"       # API 密钥
```

### AI 检测配置

```yaml
AI:
  Enabled: false                # 启用/禁用
  Backend: "onnx"               # 后端：onnx/embedded
  ModelPath: "models/yolov8s.onnx"
  ConfidenceThreshold: 0.5      # 检测阈值
```

## 📋 系统要求

### 最低配置
- CPU: x86_64 (amd64)
- 内存: 512 MB
- 磁盘: 1 GB 可用空间
- OS: Linux (Ubuntu 18.04+, Debian 10+, CentOS 7+)

### 推荐配置
- CPU: 2+ 核心
- 内存: 2 GB+
- 磁盘: 10 GB+ (用于存储录像)
- 网络: 1 Gbps

### 网络端口

| 协议 | 端口 | 类型 | 说明 |
|------|------|------|------|
| GB28181 SIP | 5060 | UDP | SIP 信令 |
| Web | 9080 | TCP | 管理界面 + API |
| ZLM API | 10080 | TCP | ZLM HTTP API |
| RTSP | 554 | TCP | RTSP 流媒体 |
| RTMP | 1935 | TCP | RTMP 推流 |
| RTP | 8000-9000 | TCP/UDP | 媒体传输 |
| RTP | 30000-30500 | UDP | RTP 接收 |

## 🔐 安全加固

### 1. 修改 API 密钥

```yaml
ZLM:
  API:
    Secret: "$(openssl rand -base64 32)"
```

### 2. 限制访问

```yaml
API:
  AllowedIPs:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
```

### 3. 启用 HTTPS（可选）

```yaml
API:
  EnableTLS: true
  CertFile: "/path/to/cert.pem"
  KeyFile: "/path/to/key.pem"
```

## 📊 监控与日志

### 查看日志

```bash
# 实时日志
tail -f logs/debug.log

# 特定模块
grep "GB28181" logs/debug.log
grep "ONVIF" logs/debug.log
```

### 日志文件

- `logs/debug.log` - 应用日志
- `build/zlm-runtime/log/` - ZLM 日志（内部）

### 检查服务状态

```bash
# 检查进程
ps aux | grep gb28181-server

# 检查端口
ss -tulpn | grep gb28181
netstat -tulpn | grep 5060
```

## 🐛 常见问题排查

### 问题1：库文件找不到

**错误**：
```
error while loading shared libraries: libonnxruntime.so.1: cannot open shared object file
```

**解决**：
启动脚本已自动设置 `LD_LIBRARY_PATH`，如手动启动需执行：
```bash
export LD_LIBRARY_PATH="./lib:$LD_LIBRARY_PATH"
./gb28181-server
```

### 问题2：端口被占用

**错误**：
```
listen tcp :5060: bind: address already in use
```

**解决**：
修改配置文件中的端口，或停止占用该端口的进程：
```bash
lsof -i :5060
kill -9 <PID>
```

### 问题3：GB28181 设备无法注册

**检查项**：
1. 设备 ID 是否以 Realm 开头
2. 防火墙是否允许 5060/UDP
3. `LocalIP` 是否正确设置
4. 设备与服务器是否在同一网络

### 问题4：Web 界面无法访问

**检查**：
```bash
# 检查服务是否运行
ps aux | grep gb28181-server

# 检查端口
ss -tulpn | grep 9080

# 测试连接
curl http://localhost:9080/
```

## 📦 扩展功能

### AI 目标检测启用

1. **配置启用**：
```yaml
AI:
  Enabled: true
  Backend: "onnx"
  ModelPath: "models/yolov8s.onnx"
```

2. **启动检测**：
```bash
curl -X POST http://localhost:9080/api/ai/channels/{channelId}/start \
  -H "Content-Type: application/json" \
  -d '{
    "enableRecording": true,
    "outputDir": "recordings/ai"
  }'
```

### 录像管理

**查询录像**：
```bash
curl http://localhost:9080/api/recording/list?channelId=34020000001310000001
```

**播放录像**：
```bash
curl -X POST http://localhost:9080/api/gb28181/devices/{deviceId}/playback
```

## 📞 支持

- 文档：见项目中的 `docs/` 目录
- 日志：查看 `logs/debug.log`
- 配置示例：`configs/config.yaml`

## 📝 更新日志

### v1.0.0 (2026-01-05)

#### 新增
- ✨ 完整的 GB28181 国标支持
- ✨ ONVIF 多接口支持
- ✨ 内置 ZLMediaKit 媒体服务器
- ✨ Vue3 Web 管理界面
- ✨ YOLOv8 AI 目标检测
- ✨ 完整的 REST API

#### 改进
- 📦 发布包包含所有依赖库
- 📦 自动库文件路径设置
- 📦 智能端口分配
- 🔧 完整的启动脚本
- 📚 详细的使用文档

#### 修复
- 🐛 OnnxRuntime 库加载问题
- 🐛 依赖项处理

## 许可证

详见项目根目录 LICENSE 文件

---

**祝您使用愉快！** 🎉

有任何问题欢迎反馈！
