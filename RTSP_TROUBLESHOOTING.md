# ONVIF 预览启动 500 错误排查指南

## 问题描述
调用 `POST /api/onvif/devices/{deviceId}/preview/start` 时返回 HTTP 500 错误，导致无法启动预览。

## 根本原因
该错误通常由以下几个原因导致：

### 1. **RTSP URL 不可访问** (最常见，约 70%)
- 生成的 RTSP URL 格式正确，但设备上的 RTSP 服务无法提供该流
- 典型错误: `404 Not Found`

### 2. **RTSP 认证失败** (约 15%)
- 设备上配置的用户名和密码与实际不匹配
- 典型错误: `401 Unauthorized`

### 3. **网络连接问题** (约 10%)
- 设备离线或网络不通
- RTSP 端口被防火墙阻止
- 典型错误: `Connection refused`, `dial tcp: ...`

### 4. **设备没有 RTSP 服务** (约 5%)
- 某些 ONVIF 设备可能不支持 RTSP 流或需要特殊配置

## 解决方案

### 步骤 1: 检查后端日志

查看完整的错误信息：
```bash
# 查看最后 50 行日志
tail -50 logs/server.log | grep -A 5 "preview/start"

# 或查看调试日志
tail -50 logs/debug.log | grep -A 5 "启动预览失败"
```

日志会显示生成的 RTSP URL，例如：
```
[ONVIF] 启动预览失败: add stream proxy failed: DESCRIBE:404 Not Found
(RTSP URL: rtsp://admin:a123456789@192.168.1.232:554/Streaming/Channels/101)
```

### 步骤 2: 使用诊断脚本

项目提供了 RTSP 诊断脚本，可以快速检测问题：

```bash
./diagnose_rtsp.sh 192.168.1.232 554 /Streaming/Channels/101 admin a123456789
```

输出示例：
```
✓ 设备可以 ping 通
✓ RTSP 端口 554 可以连接
✗ RTSP 路径不存在 (404)

请尝试以下常见路径之一：
  /Streaming/Channels/101 (海康)
  /live/1 (大华)
  /livestream (宇视)
```

### 步骤 3: 验证 RTSP URL

#### 方式 A: 使用 VLC 播放器
1. 在 VLC 中选择 "媒体" → "打开网络串流"
2. 输入 RTSP URL，例如：`rtsp://admin:password@192.168.1.232:554/Streaming/Channels/101`
3. 如果 VLC 可以播放，说明 RTSP URL 正确，问题可能在其他地方

#### 方式 B: 使用 curl 测试 (无认证)
```bash
curl -v rtsp://192.168.1.232:554/Streaming/Channels/101
```

#### 方式 C: 使用 curl 测试 (带认证)
```bash
curl -v --user admin:password rtsp://192.168.1.232:554/Streaming/Channels/101
```

#### 方式 D: 使用 ffprobe
```bash
ffprobe rtsp://admin:password@192.168.1.232:554/Streaming/Channels/101
```

### 步骤 4: 常见设备的 RTSP 路径

| 设备厂商 | 常见路径 | 备注 |
|--------|--------|------|
| 海康威视 | `/Streaming/Channels/101` | 标准路径 |
| 海康威视 | `/stream` | 某些型号 |
| 大华 | `/live/1` | 标准路径 |
| 大华 | `/stream/live/0` | 某些型号 |
| 宇视 | `/livestream` | 标准路径 |
| Axis | `/axis-media/media.amp` | 标准路径 |
| Sony | `/media/video1` | 标准路径 |
| 通用 | `/stream` | 某些设备 |

### 步骤 5: 修正 RTSP URL

如果你找到了正确的 RTSP 路径，有两种方式更新配置：

#### 方式 A: 在前端编辑设备
1. 打开 ONVIF 设备管理
2. 找到设备，点击"编辑凭证"按钮
3. 更新用户名和密码（如果需要）

#### 方式 B: 直接修改后端生成逻辑
编辑 `internal/api/handlers_onvif.go` 中的 `handleStartONVIFPreview` 函数，修改 RTSP URL 生成部分（约第 320 行）：

```go
// 修改这一行来自定义 RTSP 路径
rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:554/your/custom/path", username, password, device.IP)
```

## 高级诊断

### 查看 ZLM 代理日志
```bash
# 查看 ZLM 是否成功添加了代理流
# 如果日志中显示 "already exists"，说明之前的代理还未清理
tail -100 logs/debug.log | grep "AddStreamProxy\|StreamProxy"
```

### 检查 ZLM 服务状态
```bash
# 确认 ZLM 服务正在运行
ps aux | grep -i zlm

# 检查 ZLM API 端口 (默认 8080)
curl http://localhost:8080/api/getMediaList
```

### 手动测试 RTSP 连接
```bash
# 使用 ffmpeg 测试 RTSP 流
ffmpeg -rtsp_transport tcp -i "rtsp://admin:password@192.168.1.232:554/Streaming/Channels/101" \
  -c copy -t 5 -f null -

# 输出中应该包含视频流信息，如果有错误会显示具体的拒绝原因
```

## 完整的问题排查流程

```
[用户尝试启动预览]
         ↓
[返回 500 错误] ← 查看错误信息
         ↓
[运行诊断脚本] → 诊断 RTSP 连接问题
         ↓
[问题类型]
  ├─ [404] RTSP 路径不存在
  │   └─ 在设备网页界面查询正确的路径
  │   └─ 尝试其他常见路径
  │   └─ 更新 RTSP 路径后重试
  │
  ├─ [401] 认证失败
  │   └─ 验证用户名和密码
  │   └─ 在前端更新设备凭证
  │   └─ 重试启动预览
  │
  ├─ [Connection Refused] 网络问题
  │   └─ 检查设备是否在线
  │   └─ 检查防火墙设置
  │   └─ 检查 RTSP 端口设置
  │
  └─ [其他错误]
      └─ 查看完整服务器日志
      └─ 在设备网页查看 RTSP 配置
      └─ 尝试使用 VLC 手动连接
         ↓
[问题解决] ✓ 预览启动成功
```

## 常见错误信息解释

| 错误信息 | 可能原因 | 解决方案 |
|--------|--------|--------|
| `404 Not Found` | RTSP 路径错误 | 查询正确的 RTSP 路径 |
| `401 Unauthorized` | 凭证错误 | 验证用户名和密码 |
| `Connection refused` | 端口错误或服务未运行 | 检查 RTSP 端口和服务状态 |
| `Timeout` | 网络不通或设备离线 | 检查网络和设备状态 |
| `DESCRIBE failed` | RTSP 服务有问题 | 重启设备的 RTSP 服务 |
| `Unavailable` | 设备资源占用 | 停止其他预览，重试 |

## 工具和命令速查表

```bash
# 诊断脚本
./diagnose_rtsp.sh <IP> <端口> <路径> [用户名] [密码]

# 查看最近的错误日志
tail -100 logs/server.log | grep -i "preview\|rtsp\|404\|401"

# 使用 curl 测试 RTSP DESCRIBE
curl -v -X DESCRIBE --user admin:password rtsp://192.168.1.232:554/path

# 使用 ffprobe 检查视频信息
ffprobe -v error rtsp://admin:password@192.168.1.232:554/path

# 使用 ffmpeg 验证连接
ffmpeg -rtsp_transport tcp -i "rtsp://admin:password@192.168.1.232:554/path" \
  -c copy -t 5 -f null -
```

## 获得帮助

如果以上步骤都无法解决问题，请：

1. 收集以下信息：
   - 设备型号和固件版本
   - 网络拓扑图
   - 完整的服务器日志（logs/server.log 的最后 200 行）
   - 诊断脚本的输出结果
   - VLC 或其他 RTSP 客户端能否连接

2. 查看 ONVIF 设备管理界面中的：
   - 设备状态（是否显示为在线）
   - 已发现的媒体服务地址
   - 已获取的配置文件信息

3. 在 GitHub 或项目论坛上提交 issue，包含上述信息

## 相关文件
- 诊断脚本: [diagnose_rtsp.sh](diagnose_rtsp.sh)
- 处理函数: [internal/api/handlers_onvif.go](internal/api/handlers_onvif.go) 
- 预览管理: [internal/preview/manager.go](internal/preview/manager.go)
- 改进说明: [PROFILES_STABILITY_IMPROVEMENTS.md](PROFILES_STABILITY_IMPROVEMENTS.md)

## 更新日期
2025-12-19
