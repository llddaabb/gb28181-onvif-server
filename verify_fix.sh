#!/bin/bash
# 快速验证修复

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"

ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "🧪 修复验证"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 1. 脚本方式
echo "【脚本方式】"
nonce=$(openssl rand -base64 16 2>/dev/null)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d 2>/dev/null)${timestamp}${PASSWORD}" | openssl sha1 -binary 2>/dev/null | base64)

http_code=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$ENDPOINT" \
  -H "Content-Type: application/soap+xml; charset=utf-8" \
  -d "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
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
</s:Envelope>" 2>/dev/null)

if [ "$http_code" = "200" ]; then
  echo "✅ 脚本方式成功 (HTTP $http_code)"
else
  echo "❌ 脚本方式失败 (HTTP $http_code)"
fi

echo ""

# 2. Go 方式
echo "【Go 服务方式】"
if ! curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
  echo "⚠️ Go 服务未运行，跳过测试"
else
  # 添加设备
  curl -s -X POST http://localhost:8080/api/onvif/devices \
    -H "Content-Type: application/json" \
    -d "{\"xaddr\":\"$ENDPOINT\",\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" > /dev/null 2>&1
  
  sleep 1
  
  # 获取设备ID
  device_id="${DEVICE_IP}:${DEVICE_PORT}"
  
  # 测试 API
  api_response=$(curl -s "http://localhost:8080/api/onvif/devices/$device_id/profiles")
  
  if echo "$api_response" | grep -q "token\|名称\|error"; then
    if echo "$api_response" | grep -q "error"; then
      echo "❌ Go 方式失败"
      echo "   错误: $(echo "$api_response" | head -c 200)"
    else
      echo "✅ Go 方式成功 (获取到 Profiles)"
    fi
  else
    if [ "$api_response" = "[]" ] || [ -z "$api_response" ]; then
      echo "⚠️ Go 方式返回空列表"
    else
      echo "❓ Go 方式响应: $(echo "$api_response" | head -c 100)"
    fi
  fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
