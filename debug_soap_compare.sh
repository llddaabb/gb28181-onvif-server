#!/bin/bash
# SOAP 请求对比调试工具

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"
ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "🔍 SOAP 请求调试工具"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "设备: $ENDPOINT"
echo "用户: $USERNAME / $PASSWORD"
echo ""

# ============================================================================
# 1. 生成脚本方式的 SOAP 请求
# ============================================================================
echo "【步骤 1】生成脚本方式的 SOAP 请求"

nonce=$(openssl rand -base64 16 2>/dev/null)
nonce_raw=$(echo "$nonce" | base64 -d 2>/dev/null | xxd -p | tr -d '\n')
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

# ============================================================================
# 2. 用脚本方式调用设备（测试是否成功）
# ============================================================================
echo "【步骤 2】用脚本方式调用设备"

response=$(curl -s -w "\nHTTP_CODE:%{http_code}\n" -X POST "$ENDPOINT" \
    -H "Content-Type: application/soap+xml; charset=utf-8" \
    -H "SOAPAction: http://www.onvif.org/ver10/device/wsdl/GetSystemDateAndTime" \
    -d "$script_request" 2>&1)

http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
response_body=$(echo "$response" | grep -v "HTTP_CODE:")

echo "$response_body" > /tmp/script_soap_response.xml

if [ "$http_code" = "200" ]; then
    echo "✅ 脚本调用成功 (HTTP $http_code)"
    echo "响应已保存到: /tmp/script_soap_response.xml"
elif echo "$response_body" | grep -q "GetSystemDateAndTimeResponse"; then
    echo "✅ 脚本调用成功（包含正确响应）"
    echo "响应已保存到: /tmp/script_soap_response.xml"
else
    echo "❌ 脚本调用失败 (HTTP $http_code)"
    echo "响应前 200 字符:"
    echo "$response_body" | head -c 200
fi
echo ""

# ============================================================================
# 3. 启动 Go 服务触发请求
# ============================================================================
echo "【步骤 3】触发 Go 程序生成请求"

# 先清理旧文件
rm -f /tmp/go_soap_request.xml /tmp/go_soap_response.txt

# 启动服务
timeout 15 ./server > /tmp/server_debug.log 2>&1 &
SERVER_PID=$!
sleep 2

# 发送 API 请求触发 SOAP 调用
curl -s "http://localhost:8080/api/onvif/devices/${DEVICE_IP}:${DEVICE_PORT}/test" > /dev/null 2>&1 &

# 等待请求生成
sleep 3

# 检查是否生成了文件
if [ -f /tmp/go_soap_request.xml ]; then
    echo "✅ Go 请求已捕获: /tmp/go_soap_request.xml"
else
    echo "⚠️  Go 请求未生成，尝试触发其他接口..."
    curl -s "http://localhost:8080/api/onvif/devices/${DEVICE_IP}:${DEVICE_PORT}/profiles" > /dev/null 2>&1
    sleep 2
    
    if [ -f /tmp/go_soap_request.xml ]; then
        echo "✅ Go 请求已捕获: /tmp/go_soap_request.xml"
    else
        echo "❌ 无法生成 Go 请求文件"
        echo ""
        echo "服务器日志（最后 30 行）:"
        tail -30 /tmp/server_debug.log | grep -E "SOAP|GetSystemDateAndTime|503"
    fi
fi

# 停止服务
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo ""

# ============================================================================
# 4. 对比两个请求
# ============================================================================
if [ -f /tmp/script_soap_request.xml ] && [ -f /tmp/go_soap_request.xml ]; then
    echo "【步骤 4】对比两个 SOAP 请求"
    echo ""
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "【文本差异】"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    diff -u /tmp/script_soap_request.xml /tmp/go_soap_request.xml || true
    echo ""
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "【字节统计】"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "脚本请求: $(wc -c < /tmp/script_soap_request.xml) 字节"
    echo "Go 请求:  $(wc -c < /tmp/go_soap_request.xml) 字节"
    echo ""
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "【十六进制对比（前 200 字节）】"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "脚本:"
    xxd /tmp/script_soap_request.xml | head -10
    echo ""
    echo "Go:"
    xxd /tmp/go_soap_request.xml | head -10
    echo ""
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "【文件位置】"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "脚本请求:   /tmp/script_soap_request.xml"
echo "脚本响应:   /tmp/script_soap_response.xml"
echo "Go 请求:    /tmp/go_soap_request.xml"
echo "服务日志:   /tmp/server_debug.log"
echo ""
echo "查看完整内容:"
echo "  cat /tmp/script_soap_request.xml"
echo "  cat /tmp/go_soap_request.xml"
echo "  diff -y /tmp/script_soap_request.xml /tmp/go_soap_request.xml"
