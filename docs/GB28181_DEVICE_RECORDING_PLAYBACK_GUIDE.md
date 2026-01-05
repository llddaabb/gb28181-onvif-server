# GB28181设备录像回放故障排查指南

## 问题描述
"设备录像回放功能无法播放，ZLM只支持rtmp推流，是否是硬盘录像机的推流不正确"

## 核心原因分析

### 架构说明
```
┌─────────────────┐
│  GB28181 设备   │  (硬盘录像机/IPC)
│  (DVR/NVR/IPC)  │
└────────┬────────┘
         │ INVITE + SDP (录像回放请求)
         ↓
┌─────────────────────────────┐
│  应用程序 (本服务器)         │
│  - 生成录像回放SDP          │
│  - 指定ZLM RTP接收端口      │
└────────┬────────────────────┘
         │ INVITE 包含RTP接收地址和端口
         ↓
┌─────────────────┐
│   ZLM 服务器    │
│  (RTP接收端口)  │  PS流 → FLV转码
└────────┬────────┘
         │ HTTP FLV 地址
         ↓
┌─────────────────┐
│   前端播放器    │
│  (EasyPlayer)   │
└─────────────────┘
```

### 关键点
1. **不是RTMP推流**：GB28181设备录像回放用的是**RTP协议推流**，不是RTMP
2. **ZLM支持RTP**：ZLM已配置 `/index/api/openRtpServer` 接口用于接收RTP PS流
3. **问题症状**：
   - 前端获到FLV URL但无法播放
   - 或者INVITE请求被拒
   - 或者超时无响应

## 故障排查步骤

### 第1步：检查设备是否在线
```bash
# 查看GB28181注册的设备
curl -s "http://localhost:9080/api/gb28181/devices" | jq '.devices[] | {deviceId, name, status}'
```

**预期结果**：设备状态应为"在线"

### 第2步：查询设备录像列表
```bash
# 查询设备录像
curl -X POST "http://localhost:9080/api/gb28181/record/query" \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "34020000001180000001",  # 替换为实际设备ID
    "startTime": "2026-01-05T00:00:00",
    "endTime": "2026-01-06T00:00:00"
  }'
```

**预期结果**：应返回录像列表，包含多条记录

**如果无录像**：
- 检查设备是否有录制功能
- 验证指定时间范围内是否有录像
- 设备上检查录像存储配置

### 第3步：诊断RTP服务

**访问诊断接口**（需要认证）：
```bash
curl -s -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:9080/api/gb28181/record/playback/diagnose" | jq .
```

**预期结果**：应看到：
- ✓ ZLM API Client: OK
- ✓ RTP Server Open: OK (port 分配成功)
- ✓ List RTP Servers: OK

### 第4步：手动发起回放请求
```bash
curl -X POST "http://localhost:9080/api/gb28181/record/playback" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "channelId": "34020000001310000007",  # 替换为实际通道ID
    "startTime": "2026-01-05T00:00:00",
    "endTime": "2026-01-05T01:00:00"
  }'
```

**预期返回**：
```json
{
  "success": true,
  "streamId": "34020000001310000007_1704xxx",
  "flvUrl": "http://127.0.0.1:8081/rtp/34020000001310000007_1704xxx.live.flv",
  "ssrc": "1xxxxx"
}
```

### 第5步：检查FLV地址是否可访问

```bash
# 获得flvUrl后（例：http://127.0.0.1:8081/rtp/stream_xxx.live.flv）
# 使用curl测试是否有流数据
curl -I "http://127.0.0.1:8081/rtp/34020000001310000007_1704xxx.live.flv"
```

**预期响应**：
```
HTTP/1.1 200 OK
Content-Type: video/x-flv
```

**如果404**：RTP流未被ZLM接收

### 第6步：查看服务器日志

```bash
# 查看GB28181发送INVITE的日志
tail -100 /tmp/server_test.log | grep -E "INVITE|推流|RTP"

# 查看ZLM日志
tail -100 /home/jl/zpip/zpip/build/zlm-runtime/log/console.log | grep -E "RTP|PS|stream"
```

## 常见问题和解决方案

### 问题1：INVITE请求被拒（timeout）
**症状**：发送回放请求后报错 "timeout"

**原因**：
- 设备未收到INVITE
- 设备无法连接到指定的RTP接收端口
- 网络防火墙阻止

**解决方案**：
1. 检查防火墙规则
```bash
sudo iptables -L -n | grep 10000  # 检查RTP端口
# 如需开放RTP端口
sudo ufw allow 10000:35000/udp
```

2. 在硬盘录像机上验证：
   - 确认能ping通服务器IP
   - 检查网络配置，确保在同一网段或可路由

3. 查看应用日志中的INVITE目标
```bash
tail -50 /tmp/server_test.log | grep "目标设备"
# 应该显示：目标设备=xxx.xxx.xxx.xxx:5060
```

### 问题2：INVITE成功但FLV无法播放
**症状**：
- API返回flvUrl
- 但访问URL无内容（404或0字节）

**原因**：
- 设备没有推送RTP数据到ZLM
- RTP流数据损坏或格式不兼容
- ZLM RTP接收端口关闭

**解决方案**：
1. 监控ZLM接收的RTP包
```bash
# 使用tcpdump监听RTP端口
sudo tcpdump -i any "udp port >= 10000 and udp port <= 35000" -c 20
```

2. 检查ZLM RTP服务器列表
```bash
# 在诊断接口中查看 "List RTP Servers" 的输出
# 应该包含刚才开启的stream
```

3. 重启ZLM服务
```bash
pkill MediaServer
sleep 2
cd /home/jl/zpip/zpip && ./dist/gb28181-server  
```

### 问题3：播放器显示黑屏但无报错
**症状**：
- flvUrl可正常访问（200 OK）
- 但播放器无视频内容

**原因**：
- RTP PS流格式不匹配（设备可能发送H.265而ZLM期望H.264）
- 音视频编码不兼容
- 流数据损坏

**解决方案**：
1. 在硬盘录像机上检查：
   - 录像文件的编码格式（H.264/H.265）
   - 分辨率和帧率设置
   - 录像压缩质量

2. 使用FFmpeg检查RTP流
```bash
# 实时捕获并转存为MP4检查
ffmpeg -i "rtp://127.0.0.1:10001" -c copy -t 5 test.mp4

# 检查流信息
ffprobe -show_format -show_streams test.mp4 | grep codec
```

3. 配置ffmpeg自动转码（编辑后端）
   - 修改ZLM配置强制H.264转码

### 问题4：同时播放多条录像时卡顿或崩溃
**症状**：单条可播放，多条时卡顿或服务崩溃

**原因**：
- RTP资源未正确释放
- 端口泄漏导致端口用尽
- ffmpeg进程过多消耗CPU

**解决方案**：
1. 添加播放结束时的清理逻辑
```javascript
// 前端播放完成或关闭时
axios.post('/api/gb28181/record/playback/stop', {
  channelId: selectedChannel,
  streamId: currentStreamId
})
```

2. 定期检查并清理泄漏的RTP端口
```bash
# 查看占用的RTP端口
lsof -i :10000-35000

# 查看僵尸进程
ps aux | grep "defunct"
```

3. 限制并发回放数量
   - 修改应用配置，最多同时允许N条录像回放

## 网络配置建议

### 多网卡环境
如果服务器有多个网卡，需要确保使用正确的出站IP：

```go
// 应用会自动选择与设备同网段的IP
// 当前逻辑：getLocalIPForRemote(device.SipIP)
// 验证方法：
// 1. 查看日志中的 "ZLM接收地址=" 
// 2. 该IP应能被设备正常访问
```

### 端口映射（内网穿透）
如果设备在远程网络：

```
远程设备 → [互联网] → 端口映射网关 → 本地应用
```

需要在网关上映射：
- SIP信令端口：5060 (TCP/UDP)
- RTP数据端口：10000-35000 (UDP)

## 快速诊断命令集

```bash
#!/bin/bash
echo "=== GB28181 设备录像回放诊断 ==="
echo ""

echo "1. 检查设备在线状态"
curl -s "http://localhost:9080/api/gb28181/devices" | \
  jq '.devices[] | {id: .deviceId, name: .name, status: .status}'

echo ""
echo "2. 查询第一个设备的录像"
DEVICE_ID=$(curl -s "http://localhost:9080/api/gb28181/devices" | jq -r '.devices[0].deviceId')
echo "Device: $DEVICE_ID"

echo ""
echo "3. 检查ZLM RTP配置"
curl -s "http://localhost:9080/api/gb28181/record/playback/diagnose" | \
  jq '.checks[] | {name, status, info}'

echo ""
echo "4. 查看当前打开的RTP端口"
ss -ulnp | grep "10\|11\|12\|13\|14\|15\|16\|17\|18\|19\|20\|21\|22\|23\|24\|25\|26\|27\|28\|29\|30\|31\|32\|33\|34\|35"

echo ""
echo "=== 诊断完成 ==="
```

## 终极解决方案：使用RTSP代理

如果GB28181 RTP方案仍有问题，可以考虑设备本身提供的RTSP流进行代理播放：

```bash
curl -X POST "http://localhost:9080/api/stream/proxy" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "rtsp://device_ip:554/stream",  # 设备RTSP地址
    "app": "playback",
    "stream": "device_replay"
  }'
```

## 相关配置文件

### ZLM RTP配置
位置：`/home/jl/zpip/zpip/build/zlm-runtime/conf/config.ini`

关键参数：
```ini
[rtp_proxy]
port=10000                  # RTP接收起始端口
port_range=30000-35000     # 端口范围
h264_pt=98                 # H.264 payload type
h265_pt=99                 # H.265 payload type  
ps_pt=96                   # PS payload type (GB28181用)
```

### 应用配置
位置：`/home/jl/zpip/zpip/configs/config.yaml`

关键参数：
```yaml
gb28181:
  sipIp: 0.0.0.0          # SIP监听地址
  sipPort: 5060
  realm: ParkingLot       # SIP域
  serverName: ZLM
```

## 联系支持

如需更多帮助，请提供：
1. 服务器日志：`tail -200 /tmp/server_test.log`
2. 硬盘录像机型号和软件版本
3. 网络拓扑图
4. 诊断接口输出
