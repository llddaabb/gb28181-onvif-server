#!/bin/bash
# 启动服务并捕获 SOAP 请求体以便对比

set -e

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"
ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "🔍 SOAP 请求体对比工具"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 生成脚本方式的请求
echo "【1️⃣ 生成脚本方式请求】"
nonce=$(openssl rand -base64 16 2>/dev/null)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d 2>/dev/null)${timestamp}${PASSWORD}" | openssl sha1 -binary 2>/dev/null | base64)

script_request="<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<s:Envelope xmlns:s=\"http://www.w3.org/2003/05/soap-envelope\">
  <s:Header>
    <Security s:mustUnderstand=\"1\" xmlns=\"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd\">
      <UsernameToken>
        <Username>$USERNAME</Username>
        <Password Type=\"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest\">$digest</Password>
        <Nonce EncodingType=\"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary\">$nonce</Nonce>
        <Created xmlns=\"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd\">$timestamp</Created>
      </UsernameToken>
    </Security>
  </s:Header>
  <s:Body>
    <GetSystemDateAndTime xmlns=\"http://www.onvif.org/ver10/device/wsdl\"/>
  </s:Body>
</s:Envelope>"

echo "$script_request" > /tmp/script_soap_request.xml
echo "✅ 已保存到: /tmp/script_soap_request.xml"
echo ""

# 启动 Go 服务（后台）
echo "【2️⃣ 启动 Go 服务并触发请求】"
rm -f /tmp/go_soap_request.xml

# 启动服务
timeout 15 ./server > /tmp/server.log 2>&1 &
SERVER_PID=$!

sleep 2

# 触发请求
echo "发送 API 调用..."
curl -s "http://localhost:8080/api/onvif/devices/$DEVICE_IP:$DEVICE_PORT/profiles" > /dev/null 2>&1 &

# 等待请求完成
sleep 3

# 检查是否生成了请求文件
if [ -f /tmp/go_soap_request.xml ]; then
  echo "✅ Go 请求已捕获: /tmp/go_soap_request.xml"
  
  echo ""
  echo "【3️⃣ 对比两个请求】"
  echo ""
  
  # 计算差异
  diff_output=$(diff -u /tmp/script_soap_request.xml /tmp/go_soap_request.xml 2>&1 || true)
  
  if [ -z "$diff_output" ]; then
    echo "✅ 两个请求完全相同！"
  else
    echo "❌ 两个请求有差异:"
    echo ""
    echo "$diff_output"
  fi
else
  echo "⚠️ Go 请求未被捕获（可能未发生 503 错误）"
  echo ""
  echo "检查服务日志:"
  tail -20 /tmp/server.log | grep -E "SOAP|GetSystemDateAndTime|GetProfiles|503"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 清理
kill $SERVER_PID 2>/dev/null || true
