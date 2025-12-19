#!/bin/bash
# 对比脚本方式和 Go 方式的 WSSE 认证

set -e

# 配置
DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-admin}"
PASSWORD="${4:-a123456789}"

ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "════════════════════════════════════════════════════════════"
echo "WSSE 认证对比测试"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "目标设备: $ENDPOINT"
echo "用户名: $USERNAME"
echo "密码: $PASSWORD"
echo ""

# ============================================================================
# 方式1: 脚本方式（来自 onvif_test.sh）
# ============================================================================
echo "【1️⃣ 脚本方式 (Shell + openssl)】"
echo "────────────────────────────────────────────────────────────"

nonce=$(openssl rand -base64 16 2>/dev/null || head -c 16 /dev/urandom | base64)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d 2>/dev/null)${timestamp}${PASSWORD}" | openssl sha1 -binary 2>/dev/null | base64)

echo "  Nonce (base64): $nonce"
echo "  Timestamp: $timestamp"
echo "  Digest: $digest"
echo ""

wsse_request="<?xml version=\"1.0\" encoding=\"UTF-8\"?>
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

echo "发送脚本方式的SOAP请求..."
response=$(curl -s -X POST "$ENDPOINT" \
  -H "Content-Type: application/soap+xml; charset=utf-8" \
  -d "$wsse_request" 2>/dev/null)

if echo "$response" | grep -q "GetSystemDateTimeResponse\|UTCDateTime"; then
  echo "✅ 脚本方式成功!"
  echo ""
  echo "响应片段:"
  echo "$response" | head -c 300
  echo "..."
else
  echo "❌ 脚本方式失败!"
  echo ""
  echo "HTTP 状态码:"
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$ENDPOINT" \
    -H "Content-Type: application/soap+xml; charset=utf-8" \
    -d "$wsse_request"
  echo ""
  echo "错误响应:"
  echo "$response" | head -c 300
  echo "..."
fi

echo ""
echo ""

# ============================================================================
# 方式2: Go 服务方式
# ============================================================================
echo "【2️⃣ Go 服务方式】"
echo "────────────────────────────────────────────────────────────"

# 检查 Go 服务是否运行
if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
  echo "✅ Go 服务正在运行"
  echo ""
  
  # 调用 API 获取设备
  echo "获取设备列表..."
  devices=$(curl -s http://localhost:8080/api/onvif/devices | grep -o '"DeviceID":"[^"]*"' | head -1)
  
  if [ -z "$devices" ]; then
    echo "⚠️ 设备列表为空，尝试手动添加设备..."
    echo ""
    echo "调用 POST /api/onvif/devices 添加设备..."
    add_response=$(curl -s -X POST http://localhost:8080/api/onvif/devices \
      -H "Content-Type: application/json" \
      -d "{\"xaddr\":\"$ENDPOINT\",\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
    
    echo "添加响应:"
    echo "$add_response" | head -c 300
  else
    device_id=$(echo "$devices" | sed 's/.*"DeviceID":"\([^"]*\)".*/\1/')
    echo "找到设备: $device_id"
    echo ""
    echo "获取 Profiles..."
    profiles=$(curl -s "http://localhost:8080/api/onvif/devices/$device_id/profiles")
    
    if echo "$profiles" | grep -q "token\|name"; then
      echo "✅ Go 方式成功!"
      echo ""
      echo "响应片段:"
      echo "$profiles" | head -c 300
    else
      echo "❌ Go 方式失败!"
      echo ""
      echo "错误响应:"
      echo "$profiles" | head -c 300
    fi
  fi
else
  echo "❌ Go 服务未运行"
  echo ""
  echo "请先启动服务:"
  echo "  ./server"
fi

echo ""
echo ""
echo "════════════════════════════════════════════════════════════"
echo "对比完成"
echo "════════════════════════════════════════════════════════════"
