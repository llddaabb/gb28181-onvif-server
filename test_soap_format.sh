#!/bin/bash
# 测试 SOAP 请求格式

echo "🔍 测试 SOAP 请求格式"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 启动服务（后台）
timeout 20 ./server > /tmp/server_test.log 2>&1 &
SERVER_PID=$!

sleep 2

# 发送请求以触发 503 错误日志
echo "📡 发送请求..."
curl -s "http://localhost:8080/api/onvif/devices/192.168.1.250:8888/profiles" > /dev/null 2>&1 &

sleep 3

# 提取 SOAP 请求体
echo ""
echo "【Go 生成的 SOAP 请求】"
if [ -f /tmp/go_soap_request.xml ]; then
  cat /tmp/go_soap_request.xml
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
  echo "❌ 未生成请求文件"
fi

echo ""
echo "【脚本参考格式】"
# 从脚本提取 WSSE 请求格式
grep -A 20 'wsse_request=' /home/jl/下载/zpip/zpip/onvif_test.sh | head -22

# 关闭服务
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
