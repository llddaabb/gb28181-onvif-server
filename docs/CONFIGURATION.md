# 配置管理指南

## 概述

本系统采用 **单一配置源 (Single Source of Truth)** 设计原则，所有系统配置统一存放在 `configs/config.yaml` 中，ZLMediaKit 的配置由程序在启动时自动生成。

## 配置架构

```
┌─────────────────────────────────────────────┐
│         configs/config.yaml                 │
│  (唯一的配置源，包含所有系统配置)           │
└────────────────┬────────────────────────────┘
                 │
                 ├──> GB28181 配置
                 ├──> ONVIF 配置
                 ├──> API 服务配置
                 ├──> ZLM 配置 ──────┐
                 ├──> AI 检测配置    │
                 └──> 认证配置       │
                                    │
                    (启动时生成)     │
                                    ▼
┌─────────────────────────────────────────────┐
│     zlm_config.ini (自动生成)               │
│  (供 ZLMediaKit 进程读取，无需手动维护)    │
└─────────────────────────────────────────────┘
```

## 主配置文件：config.yaml

### 文件位置
```
configs/config.yaml
```

### 主要配置段

#### 1. GB28181（SIP 服务器配置）
```yaml
GB28181:
  SipIP: "0.0.0.0"              # SIP 监听地址
  SipPort: 5060                 # SIP 监听端口
  Realm: "3402000000"           # SIP Realm
  ServerID: "34020000002000000001"  # 服务器 ID
  HeartbeatInterval: 60         # 心跳间隔（秒）
  RegisterExpires: 3600         # 注册超时时间（秒）
```

#### 2. ONVIF（设备发现配置）
```yaml
ONVIF:
  MediaPortRange: "8000-9000"   # 媒体端口范围
  EnableCheck: true             # 启用设备健康检查
  CheckInterval: 120            # 检查间隔（秒）
  DiscoveryInterval: 60         # 发现间隔（秒）
```

#### 3. API（REST API 服务配置）
```yaml
API:
  Host: "0.0.0.0"               # API 监听地址
  Port: 9080                    # API 监听端口
  CorsAllowOrigins:
    - "*"                       # CORS 允许的源
  StaticDir: "www"              # 静态文件目录
```

#### 4. Debug（调试配置）
```yaml
Debug:
  Enabled: true                 # 启用调试
  LogLevel: "info"              # 日志级别
  LogFile: "logs/debug.log"     # 日志文件
  Services:
    - "*"                       # 调试的服务（* 表示全部）
  Timestamp: true               # 在日志中显示时间戳
  CallerInfo: true              # 在日志中显示调用者信息
```

#### 5. ZLM（ZLMediaKit 媒体服务器配置）

**进程管理：**
```yaml
ZLM:
  UseEmbedded: true             # 使用嵌入式 ZLM
  AutoRestart: true             # 进程异常退出时自动重启
  MaxRestarts: 5                # 最大重启次数
```

**API 配置：**
```yaml
  API:
    Debug: false                # ZLM API 调试模式
    Secret: "your-secret-key"   # ZLM API 密钥（需修改！）
    SnapRoot: "./www/snap/"     # 截图保存路径
    DefaultSnap: "./www/logo.png"  # 默认截图
```

**协议支持：**
```yaml
  Protocol:
    EnableHLS: true             # 启用 HLS
    EnableRTSP: true            # 启用 RTSP
    EnableRTMP: true            # 启用 RTMP
    EnableFMP4: true            # 启用 fMP4
    # ... 更多协议配置
```

**HTTP 服务：**
```yaml
  HTTP:
    Port: 8081                  # HTTP 端口
    SSLPort: 8443               # HTTPS 端口
    KeepAliveSecond: 30         # 连接保活时间
    AllowCrossDomains: true     # 允许跨域
```

**流传输协议：**
```yaml
  RTMP:
    Port: 1935                  # RTMP 端口
    Enhanced: true              # 启用增强 RTMP（H.265 支持）
  
  RTSP:
    Port: 8554                  # RTSP 端口
    DirectProxy: true           # 直接代理模式
  
  RTPProxy:
    Port: 10000                 # RTP 代理端口（GB28181）
    PortRange: "30000-35000"    # 端口范围
  
  RTC:
    Port: 8003                  # RTC 端口
    TCPPort: 8000               # RTC TCP 端口
```

**录制配置：**
```yaml
  Record:
    AppName: "record"           # 录制应用名
    RecordPath: "./recordings"  # 录制文件保存路径
    FileSecond: 600             # 单个文件最大时长（秒）
    SampleMS: 500               # 采样间隔（毫秒）
```

**HLS 配置：**
```yaml
  HLS:
    SegDur: 2                   # 分片时长（秒）
    SegNum: 3                   # 分片数量
    SegRetain: 5                # 保留分片数
    DeleteDelaySec: 10          # 删除延迟（秒）
```

#### 6. AI（智能检测配置）
```yaml
AI:
  Enable: true                  # 启用 AI 检测
  DetectorType: "embedded"      # 检测器类型
  Confidence: 0.55              # 置信度阈值
  IoUThreshold: 0.45            # IoU 阈值
  DetectInterval: 2             # 检测间隔（秒）
  RecordDelay: 15               # 录像延迟（秒）
```

#### 7. Auth（认证配置）
```yaml
Auth:
  Enable: true                  # 启用认证
  JWTSecret: "your-secret"      # JWT 密钥
  TokenExpiry: 24               # Token 过期时间（小时）
  UsersFile: "configs/users.json"  # 用户配置文件
  DefaultAdmin: "admin"         # 默认管理员用户名
  DefaultPassword: "admin123"   # 默认管理员密码
```

## ZLM 配置自动生成

### 生成流程

1. **启动时加载 config.yaml**
   ```go
   cfg, err := config.Load("configs/config.yaml")
   ```

2. **调用生成方法**
   ```go
   configContent := cfg.ZLM.GenerateConfigINI()
   ```

3. **传递给 ZLM 进程**
   ```go
   zlmProcess.SetConfigContent(configContent)
   zlmProcess.Start()
   ```

4. **写入临时配置文件**
   - 路径：`{ZLM工作目录}/conf/config.ini`
   - ZLM 进程启动时自动读取此文件

### 自动生成的配置段

| 配置段 | 说明 |
|--------|------|
| `[api]` | ZLM API 配置（密钥、截图等） |
| `[ffmpeg]` | FFmpeg 集成配置 |
| `[protocol]` | 协议支持配置（HLS、RTMP 等） |
| `[general]` | 通用配置 |
| `[hls]` | HLS 特定配置 |
| `[hook]` | 钩子/回调配置 |
| `[http]` | HTTP 服务配置 |
| `[record]` | 录制配置 |
| `[rtmp]` | RTMP 协议配置 |
| `[rtsp]` | RTSP 协议配置 |
| `[rtp]` | RTP 配置 |
| `[rtp_proxy]` | RTP 代理配置（GB28181） |
| `[rtc]` | WebRTC 配置 |
| `[srt]` | SRT 协议配置 |
| `[shell]` | 调试 Shell 配置 |

## 配置管理最佳实践

### ✅ 推荐做法

1. **编辑 config.yaml**
   - 修改 ZLM 配置时，只需编辑 config.yaml
   - 重启服务，配置自动生效

2. **使用版本控制**
   ```bash
   git add configs/config.yaml
   git commit -m "Update ZLM configuration"
   ```

3. **保留示例文件**
   - `config.example.yaml` 作为参考模板
   - 新部署时复制为 `config.yaml`

4. **定期备份**
   ```bash
   cp configs/config.yaml configs/config.yaml.backup
   ```

### ❌ 避免做法

1. **手动编辑 zlm_config.ini**
   - 这是自动生成的文件，手动修改会被覆盖
   - 可能导致配置不一致

2. **在不同地方维护相同的配置**
   - ZLM 配置应只在 config.yaml 中维护
   - 避免多处更新造成的不同步

3. **混合使用多个配置源**
   - 不要同时编辑 config.yaml 和 zlm_config.ini
   - 单一源原则确保一致性

## 配置验证

启动时自动验证配置，检查项包括：

| 检查项 | 类型 | 说明 |
|--------|------|------|
| API 密钥 | ⚠️ 警告 | 未设置时安全性降低 |
| 端口有效性 | ❌ 错误 | 端口号超出范围（0-65535） |
| 必填字段 | ❌ 错误 | GB28181 Realm、ServerID 等 |
| FFmpeg 路径 | ⚠️ 警告 | 文件不存在时无法转码 |
| 录制目录 | ⚠️ 警告 | 目录不存在时自动创建 |
| 日志目录 | ⚠️ 警告 | 目录不存在时自动创建 |

**说明：**
- ❌ 错误：配置无法启动，必须修复
- ⚠️ 警告：服务可启动，但可能影响功能

## 常见问题

### Q: zlm_config.ini 在哪里？

A: 这是个临时生成的文件：
- 嵌入式模式：`{临时目录}/zlm-runtime/conf/config.ini`
- 外部 ZLM：由指定的 ZLM 进程在其工作目录下

### Q: 如何修改 ZLM 配置？

A: 只需编辑 `config.yaml` 中的 `ZLM:` 段，然后重启服务即可。

### Q: 启动时提示配置错误怎么办？

A: 检查日志输出：
```
[错误] API 端口无效: 99999
[警告] FFmpeg 不在指定路径: /usr/bin/ffmpeg
```

按照错误信息修正 config.yaml。

### Q: 如何在运行时更新配置？

A: 目前需要重启服务。未来可支持 HTTP API 动态更新。

## 配置迁移

### 从多文件迁移到统一配置

如果之前使用多个配置文件：

1. 导出 ZLM 配置段
2. 添加到 config.yaml 中的 `ZLM:` 段
3. 验证所有参数值正确
4. 启动服务，观察日志输出
5. 删除旧的 zlm_config.ini

## 相关命令

```bash
# 验证配置（启动服务会自动验证）
./server -config configs/config.yaml

# 禁用 ZLM（用于调试其他组件）
./server -no-zlm

# 使用外部 ZLM
./server -external-zlm

# 禁用自动端口清理
./server -no-port-clean

# 自定义配置文件路径
./server -config /path/to/custom/config.yaml
```

## 文件关系图

```
配置相关文件树：
├── configs/
│   ├── config.yaml             # 唯一的配置源 ✅（纳入版本控制）
│   ├── config.example.yaml     # 配置示例（参考用）
│   ├── users.json              # 认证用户列表
│   ├── storage.json            # 存储配置
│   └── zlm_config.ini          # 自动生成（不应纳入版本控制）❌
├── logs/
│   ├── debug.log               # 应用日志
│   └── ...
└── internal/config/
    └── config.go               # 配置管理代码
        ├── Load()              # 加载配置
        ├── Validate()          # 验证配置
        ├── GenerateConfigINI() # 生成 ZLM 配置
        └── Save()              # 保存配置
```

## 更新日志

### v1.0.0 (2026-01-05)

- ✅ 实现单一配置源设计
- ✅ 自动生成 ZLM 配置
- ✅ 配置验证机制
- ✅ 详细的配置文档

---

**需要帮助？** 查看 [config.yaml](../configs/config.yaml) 了解所有可用配置项。
