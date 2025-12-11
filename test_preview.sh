#!/bin/bash

# GB28181 预览测试脚本
API_URL="http://localhost:9080"

# 1. 获取设备列表
echo "=== 获取GB28181设备列表 ==="
DEVICES=$(curl -s "$API_URL/api/gb28181/devices" | python3 -m json.tool)
echo "$DEVICES"

# 提取第一个设备的ID - 适配新的API响应格式
DEVICE_ID=$(echo "$DEVICES" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['devices'][0]['id'] if data.get('devices') else '')" 2>/dev/null)

if [ -z "$DEVICE_ID" ]; then
    echo "❌ 没有找到GB28181设备"
    exit 1
fi

echo -e "\n✓ 找到设备: $DEVICE_ID"

# 2. 获取设备信息（通道已经在devices中了）
echo -e "\n=== 设备通道信息 ==="
CHANNEL_ID=$(echo "$DEVICES" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['devices'][0]['channels'][0]['id'] if data.get('devices') and data['devices'][0].get('channels') else '')" 2>/dev/null)

if [ -z "$CHANNEL_ID" ]; then
    echo "❌ 没有找到通道"
    exit 1
fi

echo -e "✓ 找到通道: $CHANNEL_ID"

# 3. 启动预览
echo -e "\n=== 启动预览 ==="
PREVIEW=$(curl -s -X POST "$API_URL/api/gb28181/devices/$DEVICE_ID/preview/start" \
    -H "Content-Type: application/json" \
    -d "{\"channelId\": \"$CHANNEL_ID\"}" | python3 -m json.tool)
echo "$PREVIEW"

# 提取流URL
FLV_URL=$(echo "$PREVIEW" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['data']['flv_url'] if data.get('data') else '')" 2>/dev/null)

if [ ! -z "$FLV_URL" ]; then
    echo -e "\n✓ FLV URL: $FLV_URL"
    echo "✓ 预览已启动，等待RTP流..."
else
    echo "❌ 获取FLV URL失败"
    exit 1
fi

# 4. 等待一段时间观察日志
echo -e "\n=== 日志输出（等待15秒） ==="
sleep 15
tail -50 logs/server.log | grep -E "RTP|ACK|Response|stream|INVITE|SIP"
