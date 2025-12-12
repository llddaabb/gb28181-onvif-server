#!/bin/bash
# GB28181 流媒体状态诊断工具

STREAM_ID="$1"
if [ -z "$STREAM_ID" ]; then
    echo "用法: ./check_stream.sh <stream_id>"
    echo "示例: ./check_stream.sh 34020000001310000005"
    exit 1
fi

echo "=== 检查流媒体状态: $STREAM_ID ==="
echo ""

# 1. 检查后端服务
echo "1. 检查后端服务健康状态..."
curl -s http://localhost:9080/api/health || echo "❌ 后端服务未运行"
echo ""

# 2. 检查 ZLM 服务
echo "2. 检查 ZLM 服务..."
ZLM_STATUS=$(curl -s -w "%{http_code}" -o /dev/null http://localhost:8081/index/api/getServerConfig 2>/dev/null)
if [ "$ZLM_STATUS" == "200" ] || [ "$ZLM_STATUS" == "400" ]; then
    echo "✅ ZLM 服务运行中 (HTTP:8081)"
else
    echo "❌ ZLM 服务未响应 (状态码: $ZLM_STATUS)"
fi
echo ""

# 3. 检查通道列表
echo "3. 检查通道列表..."
curl -s http://localhost:9080/api/channel/list | jq '.data[] | {id, name, status}' 2>/dev/null || echo "❌ 无法获取通道列表"
echo ""

# 4. 检查 FLV 流（通过代理）
echo "4. 检查 FLV 流状态（通过后端代理）..."
FLV_URL="http://localhost:9080/zlm/rtp/${STREAM_ID}.live.flv"
echo "URL: $FLV_URL"
curl -s -I "$FLV_URL" | grep -E "HTTP|Content-Length|Content-Type"
echo ""

# 5. 检查 FLV 流（直接访问 ZLM）
echo "5. 检查 FLV 流状态（直接访问 ZLM）..."
FLV_DIRECT="http://localhost:8081/rtp/${STREAM_ID}.live.flv"
echo "URL: $FLV_DIRECT"
curl -s -I "$FLV_DIRECT" | grep -E "HTTP|Content-Length|Content-Type"
echo ""

# 6. 检查 HLS 流
echo "6. 检查 HLS 流状态..."
HLS_URL="http://localhost:9080/zlm/rtp/${STREAM_ID}/hls.m3u8"
echo "URL: $HLS_URL"
curl -s -I "$HLS_URL" | grep -E "HTTP|Content-Length|Content-Type"
echo ""

# 7. 尝试获取 ZLM 媒体列表（需要 secret）
echo "7. 检查 ZLM 配置中的 secret..."
SECRET=$(grep -r "secret=" configs/zlm_config.ini 2>/dev/null | head -1 | cut -d'=' -f2)
if [ -n "$SECRET" ]; then
    echo "找到 secret: $SECRET"
    echo "获取媒体列表..."
    curl -s "http://localhost:8081/index/api/getMediaList?secret=$SECRET" | jq '.' 2>/dev/null || echo "❌ 无法解析响应"
else
    echo "⚠️  未找到 secret，尝试空 secret..."
    curl -s "http://localhost:8081/index/api/getMediaList?secret=" | jq '.' 2>/dev/null || echo "❌ 无法解析响应"
fi
echo ""

# 8. 检查进程状态
echo "8. 检查相关进程..."
ps aux | grep -E "MediaServer|./server" | grep -v grep || echo "⚠️  未找到相关进程"
echo ""

# 9. 网络端口检查
echo "9. 检查网络端口监听..."
echo "ZLM HTTP (8081):"
ss -tlnp | grep :8081 || echo "❌ 端口 8081 未监听"
echo "后端 API (9080):"
ss -tlnp | grep :9080 || echo "❌ 端口 9080 未监听"
echo "ZLM RTSP (8554):"
ss -tlnp | grep :8554 || echo "❌ 端口 8554 未监听"
echo ""

echo "=== 诊断完成 ==="
echo ""
echo "💡 提示："
echo "  - 如果 Content-Length 为 0，说明流还未推送数据"
echo "  - GB28181 设备通常需要 3-5 秒建立 RTP 连接"
echo "  - 可以等待几秒后重新运行此脚本"
echo "  - 查看后端日志: tail -f logs/*.log"
