#!/bin/bash
# 生成完整的脚本方式 SOAP 请求以供对比

DEVICE_IP="${1:-192.168.1.250}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"

echo "🔍 SOAP 请求体精确对比"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 生成脚本方式的请求体
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

echo "【脚本方式 SOAP 请求体】"
echo "$script_request"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 保存到临时文件用于对比
echo "$script_request" > /tmp/script_request.xml

echo "✅ 脚本请求已保存到: /tmp/script_request.xml"
echo ""
echo "现在启动 Go 服务，查看其生成的请求:"
echo "  ./server 2>&1 | grep -A 20 '📋 SOAP请求体预览'"
echo ""
echo "然后对比两个请求体:"
echo "  diff -u /tmp/script_request.xml /tmp/go_request.xml"
