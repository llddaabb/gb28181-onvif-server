# 录像流式播放功能说明

## 概述

已实现基于 ffmpeg 的录像流式播放功能，使用硬件加速将录像文件推流到 ZLM 服务器，然后通过 FLV 格式播放。

## 主要特性

### 1. 硬件加速检测 (`internal/mediautil/hwaccel.go`)

- 自动检测系统可用的硬件加速类型（CUDA/QSV/VAAPI/VideoToolbox）
- 按优先级选择最佳加速方式
- 测试实际可用性，避免误检
- 全局单例模式，启动时异步检测

支持的硬件加速：
- **CUDA**: NVIDIA GPU 加速（优先级最高）
- **QSV**: Intel Quick Sync Video
- **VAAPI**: Linux VAAPI 加速
- **VideoToolbox**: macOS 硬件加速
- **None**: 软件解码（后备方案）

### 2. ffmpeg 推流管理器 (`internal/mediautil/ffmpeg_stream.go`)

- 管理多个并发推流会话
- 自动使用检测到的硬件加速
- 优雅的会话启动/停止
- 会话超时自动清理
- 进程异常监控

推流流程：
```
录像文件.mp4 → ffmpeg(硬件加速) → RTMP → ZLM → FLV → 浏览器
```

### 3. API 接口 (`internal/api/recording_handlers.go`)

#### 3.1 开始推流播放
```
GET /api/recording/zlm/stream/{app}/{stream}/{file}
```

返回：
```json
{
  "success": true,
  "streamId": "recording_1736812345_1",
  "flvUrl": "http://localhost:8080/live/recording_1736812345_1.live.flv",
  "playUrl": "http://localhost:8080/live/recording_1736812345_1.live.flv",
  "downloadUrl": "http://localhost:9080/api/recording/zlm/file/live/channel1/file.mp4",
  "hwAccel": "cuda",
  "note": "FLV流式播放，保留MP4下载"
}
```

#### 3.2 停止推流
```
POST /api/recording/zlm/stream/stop?streamId=recording_1736812345_1
```

#### 3.3 列出所有推流会话
```
GET /api/recording/zlm/stream/sessions
```

返回：
```json
{
  "success": true,
  "sessions": [
    {
      "streamId": "recording_1736812345_1",
      "filePath": "/path/to/recording.mp4",
      "flvUrl": "http://localhost:8080/live/recording_1736812345_1.live.flv",
      "rtmpUrl": "rtmp://127.0.0.1:1935/live/recording_1736812345_1",
      "hwAccel": "cuda",
      "running": true,
      "duration": 125.5,
      "startTime": "2026-01-04 15:30:45"
    }
  ],
  "total": 1
}
```

#### 3.4 下载录像（保留）
```
GET /api/recording/zlm/file/{app}/{stream}/{file}
```

直接下载 MP4 文件，支持 Range 请求。

### 4. 前端修改 (`frontend/src/views/RecordingPlayback.vue`)

#### 修改点：

1. **播放接口切换**：
   - 旧：`/api/recording/zlm/play/...`（直接 MP4）
   - 新：`/api/recording/zlm/stream/...`（ffmpeg 推流 FLV）

2. **停止播放清理**：
   - 旧：调用 `/api/recording/zlm/stop?key=...`
   - 新：调用 `/api/recording/zlm/stream/stop?streamId=...`

3. **硬件加速提示**：
   - 显示使用的硬件加速类型（CUDA/QSV/VAAPI/None）

4. **下载功能**：
   - 保留原有的 MP4 下载按钮
   - 使用 `downloadUrl` 字段

## 优势

### 相比直接 MP4 播放：

1. **兼容性更好**：
   - FLV 格式浏览器支持更广泛
   - 避免 H.265 编码兼容性问题
   - 统一使用 FLV 容器格式

2. **性能更优**：
   - 硬件加速解码，CPU 占用低
   - 多路并发推流不影响性能
   - 服务器端完成转码，客户端轻量化

3. **功能更强**：
   - 支持实时转码
   - 支持多种硬件加速
   - 灵活的流控制（启动/停止）

4. **用户体验**：
   - 自动检测硬件加速
   - 播放启动更快
   - 保留下载功能

## 配置要求

### 系统要求：

1. **ffmpeg 已安装**：
   ```bash
   ffmpeg -version
   ```

2. **硬件加速驱动**（可选）：
   - NVIDIA: CUDA 驱动 + nvidia-docker（容器）
   - Intel: Intel Media SDK
   - Linux: VAAPI 驱动

3. **ZLM 配置**：
   - RTMP 端口：1935（默认）
   - HTTP-FLV 端口：8080（默认）

### 检查硬件加速：

```bash
# 查看支持的硬件加速
ffmpeg -hwaccels

# 测试 CUDA
ffmpeg -f lavfi -i testsrc -hwaccel cuda -f null -

# 测试 QSV
ffmpeg -f lavfi -i testsrc -hwaccel qsv -f null -

# 测试 VAAPI
ffmpeg -f lavfi -i testsrc -hwaccel vaapi -f null -
```

## 日志示例

### 启动时检测：

```
[硬件加速] 开始检测可用的硬件加速...
[硬件加速] ffmpeg -hwaccels 输出:
Hardware acceleration methods:
cuda
qsv
vaapi
[硬件加速] ✓ cuda 可用
[硬件加速] ✓ qsv 可用
[硬件加速] ✗ vaapi 列出但测试失败
[硬件加速] 选择最佳加速: cuda
```

### 推流播放：

```
[ffmpeg推流] 准备推流: /path/to/recording.mp4 -> rtmp://127.0.0.1:1935/live/recording_1736812345_1
[ffmpeg推流] 使用硬件加速: cuda
[ffmpeg推流] 成功启动推流会话: recording_1736812345_1, PID: 12345
[ffmpeg推流] 推流成功: streamId=recording_1736812345_1, flvUrl=http://localhost:8080/live/recording_1736812345_1.live.flv
```

### 停止推流：

```
[ffmpeg推流] 停止推流会话: recording_1736812345_1
[ffmpeg推流] 会话 recording_1736812345_1 已停止
```

## 故障排查

### 1. 硬件加速不可用

**现象**：日志显示 `使用硬件加速: none`

**原因**：
- ffmpeg 未编译硬件加速支持
- 驱动未安装或版本不兼容
- GPU 不可用（容器环境）

**解决**：
```bash
# 检查 ffmpeg 编译配置
ffmpeg -version

# 安装 CUDA 版本 ffmpeg（Ubuntu）
apt-get install ffmpeg-cuda

# 检查 NVIDIA 驱动
nvidia-smi
```

### 2. 推流启动失败

**现象**：`推流失败: failed to start ffmpeg`

**原因**：
- ffmpeg 不在 PATH
- 录像文件不存在或损坏
- ZLM RTMP 端口未开启

**解决**：
```bash
# 检查 ffmpeg
which ffmpeg

# 检查 ZLM RTMP 端口
netstat -tuln | grep 1935

# 手动测试推流
ffmpeg -re -i test.mp4 -c copy -f flv rtmp://127.0.0.1:1935/live/test
```

### 3. 播放黑屏或无画面

**原因**：
- ZLM 未收到流
- 网络防火墙阻止
- 浏览器不支持 FLV

**解决**：
```bash
# 检查 ZLM 流列表
curl http://localhost:8080/index/api/getMediaList

# 直接访问 FLV
curl -I http://localhost:8080/live/recording_1736812345_1.live.flv

# 使用 ffplay 测试
ffplay http://localhost:8080/live/recording_1736812345_1.live.flv
```

## 性能优化建议

1. **使用硬件加速**：
   - 首选 CUDA（NVIDIA GPU）
   - 次选 QSV（Intel CPU）
   - 软件解码性能最差

2. **限制并发推流数**：
   - 建议不超过 10 路并发
   - 根据 CPU/GPU 性能调整

3. **定期清理过期会话**：
   ```go
   // 每小时清理运行超过 2 小时的会话
   go func() {
       ticker := time.NewTicker(1 * time.Hour)
       for range ticker.C {
           ffmpegStreamMgr.CleanupExpiredSessions(2 * time.Hour)
       }
   }()
   ```

4. **使用 copy 模式**（如果编码兼容）：
   - 当录像已经是 H.264 时使用 `-c:v copy`
   - 避免重新编码，性能最优

## 未来改进方向

1. **智能编码选择**：
   - 检测录像编码格式
   - H.264 直接 copy
   - H.265 转码为 H.264

2. **码率控制**：
   - 根据网络带宽调整码率
   - 支持多码率自适应

3. **会话持久化**：
   - 重启后恢复推流会话
   - 断点续播

4. **更多协议支持**：
   - HLS (m3u8)
   - DASH
   - WebRTC

## 总结

本次更新实现了完整的录像流式播放功能，使用 ffmpeg 硬件加速推流到 ZLM，再通过 FLV 格式播放。相比直接 MP4 播放，具有更好的兼容性、性能和用户体验，同时保留了 MP4 下载功能。
