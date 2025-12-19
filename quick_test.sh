#!/bin/bash
# 快速对比测试 - 脚本 vs Go

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-admin}"
PASSWORD="${4:-a123456789}"

ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "🧪 WSSE 快速对比测试"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "设备: $ENDPOINT"
echo ""

# 测试脚本方式
echo "【1️⃣ 脚本方式测试】"
nonce=$(openssl rand -base64 16 2>/dev/null)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d 2>/dev/null)${timestamp}${PASSWORD}" | openssl sha1 -binary 2>/dev/null | base64)

response=$(curl -s -w "\n%{http_code}" -X POST "$ENDPOINT" \
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

# 提取状态码和响应体
http_code=$(echo "$response" | tail -1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ] && echo "$body" | grep -q "GetSystemDateAndTimeResponse"; then
  echo "✅ 脚本方式成功 (HTTP $http_code)"
else
  echo "❌ 脚本方式失败 (HTTP $http_code)"
  if [ -n "$body" ]; then
    echo "   响应: $(echo "$body" | head -c 150)..."
  fi
fi

echo ""
echo "【2️⃣ Go 服务测试】"

# 检查服务是否运行
if ! curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
  echo "❌ Go 服务未运行"
  echo ""
  echo "请先启动服务:"
  echo "  ./server"
  exit 1
fi

echo "✅ Go 服务运行中"

# 添加设备（如果不存在）
device_id="${DEVICE_IP}:${DEVICE_PORT}"
curl -s -X POST http://localhost:8080/api/onvif/devices \
  -H "Content-Type: application/json" \
  -d "{\"xaddr\":\"$ENDPOINT\",\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" > /dev/null 2>&1

sleep 1

# 获取 Profiles
profiles_response=$(curl -s "http://localhost:8080/api/onvif/devices/$device_id/profiles")

if echo "$profiles_response" | grep -q "token\|名称" || echo "$profiles_response" | grep -q "\[\]"; then
  if echo "$profiles_response" | grep -q "\[\]"; then
    echo "⚠️ Go 方式返回空列表 (可能认证失败)"
  else
    echo "✅ Go 方式成功 (获取到 Profiles)"
  fi
else
  echo "❌ Go 方式失败"
  echo "   响应: $(echo "$profiles_response" | head -c 150)..."
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 查看日志
echo ""
echo "💡 如需查看详细日志，运行:"
echo "  ./server 2>&1 | grep -E '❗|✅|📋|GetSystemDateAndTime'"
