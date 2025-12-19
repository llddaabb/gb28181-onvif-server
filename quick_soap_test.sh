#!/bin/bash
# 快速 SOAP 测试 - 直接调用 onvif_test.sh 中的方法

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"
ENDPOINT="http://${DEVICE_IP}:${DEVICE_PORT}/onvif/device_service"

echo "🧪 快速 SOAP 测试"
echo "设备: $ENDPOINT"
echo ""

# 生成 WSSE 请求
nonce=$(openssl rand -base64 16)
timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
digest=$(echo -n "$(echo "$nonce" | base64 -d)${timestamp}${PASSWORD}" | openssl sha1 -binary | base64)

request='<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    <Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken>
        <Username>'"$USERNAME"'</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">'"$digest"'</Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">'"$nonce"'</Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">'"$timestamp"'</Created>
      </UsernameToken>
    </Security>
  </s:Header>
  <s:Body>
    <GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>
  </s:Body>
</s:Envelope>'

echo "【发送 SOAP 请求】"
echo "$request" | tee /tmp/test_request.xml
echo ""

echo "【调用设备】"
response=$(curl -v -X POST "$ENDPOINT" \
    -H "Content-Type: application/soap+xml; charset=utf-8" \
    -H "SOAPAction: http://www.onvif.org/ver10/device/wsdl/GetSystemDateAndTime" \
    -d "$request" 2>&1)

echo ""
echo "【响应】"
echo "$response" | grep -E "HTTP/|<GetSystemDateAndTimeResponse|错误:|<html>"

if echo "$response" | grep -q "GetSystemDateAndTimeResponse"; then
    echo ""
    echo "✅ 成功！设备返回了正确的 SOAP 响应"
else
    echo ""
    echo "❌ 失败！设备返回了错误"
fi
