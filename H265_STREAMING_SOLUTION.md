# H.265 流播放问题解决方案

## 问题诊断

### 核心问题
GB28181 设备推送的视频流使用 **H.265/HEVC 编码**，但 **HTTP-FLV 格式不支持 H.265**。

### 技术背景
- **流媒体存在**：ZLM 中已有流数据（rtp/34020000001310000005），码率正常（~248 KB/s）
- **编码格式**：H.265 (HEVC), 1920x1080@25fps
- **问题根源**：FLV 容器格式只支持 H.264/AVC 视频编码，不支持 H.265
- **表现**：访问 `.live.flv` 返回 `Content-Length: 0`（ZLM 无法生成 FLV 封装）

### 当前流媒体状态（经过验证）
```bash
$ curl "http://localhost:8081/index/api/getMediaList?secret=<secret>" | jq .

# 可用的流格式：
- ✅ HLS (schema: "hls")
- ✅ TS (schema: "ts") 
- ✅ RTSP (schema: "rtsp")
- ✅ RTMP (schema: "rtmp")
- ✅ fMP4 (schema: "fmp4")
- ❌ HTTP-FLV（未生成，因为 H.265 不兼容）
```

## 解决方案

### 方案一：使用 HLS (推荐) ✅

**优点**：
- 原生支持 H.265 编码
- 无需转码，服务器负载低
- 兼容性好（所有现代浏览器都支持）

**缺点**：
- 延迟较高（5-10 秒）

**实现**：

1. **前端修改** - 优先使用 HLS：
```typescript
// frontend/src/components/PreviewPlayer.vue
const playUrl = streamInfo.hls_url || streamInfo.flv_url

// 或使用 video 标签直接播放
<video :src="streamInfo.hls_url" controls autoplay />
```

2. **后端已支持**：
```go
// internal/api/helpers.go
HlsURL: fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID)
```

3. **测试**：
```bash
# 访问测试页面
http://localhost:5173/test-stream.html

# 或直接测试 HLS
curl http://localhost:9080/zlm/rtp/34020000001310000005/hls.m3u8
```

### 方案二：配置 FFmpeg 实时转码 🔧

**优点**：
- 保持使用 HTTP-FLV
- jessibuca 播放器可直接使用
- 低延迟（1-3 秒）

**缺点**：
- 增加服务器 CPU 负载
- 需要重新配置 ZLM

**实现**：

1. **修改 ZLM 配置** - `configs/zlm_config.ini`:
```ini
[ffmpeg]
bin=/usr/bin/ffmpeg
# H.265 转 H.264 (低延迟模式)
cmd=%s -rtsp_transport tcp -i rtsp://127.0.0.1:8554/%s/%s -c:v libx264 -preset ultrafast -tune zerolatency -b:v 2M -c:a copy -f flv rtmp://127.0.0.1:1935/%s/%s_h264

# 或者直接使用 FFmpeg 拉流转推（在流启动时触发）
# 通过 ZLM API 动态添加 FFmpeg 转码代理
```

2. **通过 ZLM API 添加转码流**：
```bash
curl -X POST "http://localhost:8081/index/api/addFFmpegSource" \
  -d "secret=lJVRv67NnTsUMdq7nwybzCUBTcsyyR7x" \
  -d "src_url=rtsp://127.0.0.1:8554/rtp/34020000001310000005" \
  -d "dst_url=rtmp://127.0.0.1:1935/rtp/34020000001310000005_h264" \
  -d "timeout_ms=10000" \
  -d "enable_hls=1" \
  -d "enable_mp4=0"
```

3. **前端使用转码后的流**：
```javascript
const transcodedUrl = '/zlm/rtp/34020000001310000005_h264.live.flv'
```

### 方案三：使用 WebRTC (实验性) 🚀

**优点**：
- 超低延迟（<1 秒）
- 支持 H.265（部分浏览器）

**缺点**：
- 配置复杂
- 浏览器兼容性有限

**实现**：需要配置 ZLM 的 WebRTC 模块（超出本文档范围）

### 方案四：使用 RTMP/RTSP 协议

**适用场景**：
- 桌面应用或专业播放器
- 不适合网页播放器

## 推荐实施步骤

### 快速方案（使用 HLS）

1. **修改前端播放器逻辑**：
```vue
<!-- frontend/src/components/PreviewPlayer.vue -->
<template>
  <video 
    v-if="useNativePlayer" 
    :src="streamUrl" 
    controls 
    autoplay 
    style="width: 100%; background: black;"
  />
  <div v-else :id="containerId" class="jessibuca-player"></div>
</template>

<script setup>
const useNativePlayer = computed(() => {
  // 如果是 HLS 或 MP4，使用原生播放器
  return streamUrl.value.includes('.m3u8') || streamUrl.value.includes('.mp4')
})

const streamUrl = computed(() => {
  if (!streamInfo.value) return ''
  // 优先使用 HLS（支持 H.265）
  return streamInfo.value.hls_url || streamInfo.value.flv_url
})
</script>
```

2. **后端添加 HLS 提示**：
```go
// internal/api/handlers_gb28181.go
respondRaw(w, http.StatusOK, map[string]interface{}{
    "success": true,
    "message": "预览启动成功，使用 HLS 格式（支持 H.265）",
    "data": map[string]interface{}{
        // ...
        "recommended_url": urls.HlsURL,  // 推荐使用 HLS
        "codec_warning": "流使用 H.265 编码，FLV 格式不可用，请使用 HLS",
    },
})
```

### 完整方案（FFmpeg 转码）

1. **安装 FFmpeg**（如果未安装）：
```bash
sudo apt update
sudo apt install ffmpeg
```

2. **创建转码脚本** - `scripts/setup_transcode.sh`:
```bash
#!/bin/bash
# 为所有 H.265 流添加 H.264 转码

ZLM_SECRET="lJVRv67NnTsUMdq7nwybzCUBTcsyyR7x"
ZLM_HOST="localhost:8081"

# 获取当前所有流
STREAMS=$(curl -s "http://$ZLM_HOST/index/api/getMediaList?secret=$ZLM_SECRET" | \
  jq -r '.data[] | select(.tracks[].codec_id_name=="H265") | .stream' | uniq)

for stream in $STREAMS; do
  echo "添加转码: $stream → ${stream}_h264"
  
  curl -X POST "http://$ZLM_HOST/index/api/addFFmpegSource" \
    -d "secret=$ZLM_SECRET" \
    -d "src_url=rtsp://127.0.0.1:8554/rtp/$stream" \
    -d "dst_url=rtmp://127.0.0.1:1935/rtp/${stream}_h264" \
    -d "timeout_ms=10000" \
    -d "enable_hls=1" \
    -d "enable_mp4=0"
done
```

3. **运行转码脚本**：
```bash
chmod +x scripts/setup_transcode.sh
./scripts/setup_transcode.sh
```

4. **前端使用转码流**：
```javascript
// 修改流 URL 构造逻辑
const getPlayUrl = (streamId, codec) => {
  if (codec === 'H265') {
    // H.265 流使用 HLS 或转码后的 FLV
    return `/zlm/rtp/${streamId}/hls.m3u8`  // HLS
    // 或
    return `/zlm/rtp/${streamId}_h264.live.flv`  // 转码后的 FLV
  }
  return `/zlm/rtp/${streamId}.live.flv`  // H.264 原生 FLV
}
```

## 测试与验证

### 1. 使用测试页面
访问 `http://localhost:5173/test-stream.html` 测试各种格式。

### 2. 检查流状态
```bash
# 运行诊断脚本
./check_stream.sh 34020000001310000005

# 手动检查
curl -I http://localhost:9080/zlm/rtp/34020000001310000005/hls.m3u8
curl -I http://localhost:9080/zlm/rtp/34020000001310000005.live.flv
```

### 3. 验证编码格式
```bash
# 查看流编码信息
curl -s "http://localhost:8081/index/api/getMediaList?secret=lJVRv67NnTsUMdq7nwybzCUBTcsyyR7x" | \
  jq '.data[] | {stream, codec: .tracks[0].codec_id_name}'
```

## 常见问题

### Q1: HLS 延迟太高怎么办？
**A**: 调整 HLS 切片参数：
```ini
[hls]
segDur=1  # 切片时长改为 1 秒（默认 2 秒）
segNum=2  # 减少缓冲切片数量
```

### Q2: FFmpeg 转码占用 CPU 过高？
**A**: 使用硬件加速或调整预设：
```bash
# 使用 GPU 加速（NVIDIA）
cmd=%s -hwaccel cuda -i %s -c:v h264_nvenc -preset fast ...

# 降低码率
-b:v 1M  # 从 2M 降到 1M
```

### Q3: jessibuca 能播放 HLS 吗？
**A**: jessibuca 主要支持 FLV/WebSocket-FLV，不直接支持 HLS。如果要使用 HLS，建议：
1. 使用原生 `<video>` 标签
2. 使用 hls.js 库
3. 或配置 FFmpeg 转码为 FLV

### Q4: 如何自动检测编码并选择合适的格式？
**A**: 后端返回流信息时包含编码信息：
```go
"codec": "H265",
"recommended_format": "hls",  // 或 "flv_transcoded"
"available_urls": {
  "hls": "/zlm/rtp/xxx/hls.m3u8",
  "flv_transcoded": "/zlm/rtp/xxx_h264.live.flv"
}
```

前端根据 `recommended_format` 选择播放器和 URL。

## 总结

| 方案 | 延迟 | CPU 负载 | 兼容性 | 推荐度 |
|------|------|----------|--------|--------|
| HLS | 5-10s | 低 | ✅✅✅ | ⭐⭐⭐⭐⭐ |
| FFmpeg 转码 | 1-3s | 高 | ✅✅✅ | ⭐⭐⭐⭐ |
| WebRTC | <1s | 中 | ✅✅ | ⭐⭐⭐ |
| 原生 RTSP | 实时 | 低 | ❌ | ⭐⭐ |

**最终建议**：
1. **展示类应用**：使用 HLS（简单、稳定）
2. **监控类应用**：配置 FFmpeg 转码（平衡延迟和负载）
3. **实时通信**：考虑 WebRTC（需要额外开发）

## 参考资料
- [ZLM 官方文档](https://github.com/ZLMediaKit/ZLMediaKit)
- [FFmpeg H.265 转码指南](https://trac.ffmpeg.org/wiki/Encode/H.265)
- [HLS 协议规范](https://datatracker.ietf.org/doc/html/rfc8216)
