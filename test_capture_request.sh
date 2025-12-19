#!/bin/bash
# 启动服务并捕获 SOAP 请求体

set -e

echo "🚀 启动 Go 服务..."
timeout 10 ./server 2>&1 | grep -E "SOAP请求体预览|SOAP响应体预览|GetSystemDateAndTime|✓|❌|✅" &
SERVER_PID=$!

sleep 3

echo ""
echo "📡 触发 API 调用..."

# 这将触发 GetProfiles 请求
curl -s -X GET "http://localhost:8080/api/onvif/devices" | head -c 200

sleep 2

echo ""
echo ""
echo "✅ 测试完成"
echo ""
echo "检查生成的请求体，对比脚本方式:"
echo "  cat /tmp/script_request.xml"
