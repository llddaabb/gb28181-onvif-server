#!/bin/bash
# 测试 ONVIF WS-Discovery 发现功能

echo "=== ONVIF WS-Discovery 诊断工具 ==="
echo ""

# 1. 检查网络接口
echo "1. 检查网络接口:"
ip link show | grep -E "^[0-9]:|<UP"
echo ""

# 2. 检查多播支持
echo "2. 检查多播支持:"
ip addr show | grep -E "inet |inet6 " | grep -v "127.0.0.1"
echo ""

# 3. 检查 iptables (可选)
echo "3. 检查防火墙规则 (iptables):"
sudo iptables -L -n 2>/dev/null | grep -i "3702\|239" || echo "未找到相关规则或需要 sudo 权限"
echo ""

# 4. 发送 WS-Discovery Probe 并监听响应
echo "4. 发送 WS-Discovery Probe 并监听响应 (10秒):"
cat > /tmp/probe.xml << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:wsa="http://www.w3.org/2005/08/addressing"
               xmlns:wsd="http://schemas.xmlsoap.org/ws/2005/04/discovery">
  <soap:Header>
    <wsa:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</wsa:Action>
    <wsa:MessageID>urn:uuid:test-probe</wsa:MessageID>
    <wsa:ReplyTo>
      <wsa:Address>http://www.w3.org/2005/08/addressing/anonymous</wsa:Address>
    </wsa:ReplyTo>
    <wsa:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</wsa:To>
  </soap:Header>
  <soap:Body>
    <wsd:Probe>
      <wsd:Types>tdn:NetworkVideoTransmitter</wsd:Types>
    </wsd:Probe>
  </soap:Body>
</soap:Envelope>
EOF

# 使用 socat 或 nc 发送多播消息
if command -v socat &> /dev/null; then
    timeout 10 socat UDP4-SENDTO:239.255.255.250:3702 < /tmp/probe.xml
    echo "Probe sent via socat"
elif command -v nc &> /dev/null; then
    timeout 10 bash -c "cat /tmp/probe.xml | nc -u -w 10 239.255.255.250 3702"
    echo "Probe sent via nc"
else
    echo "需要安装 socat 或 nc 工具"
fi

rm -f /tmp/probe.xml

echo ""
echo "5. 尝试检测网络上的 ONVIF 设备 (使用 nmap):"
if command -v nmap &> /dev/null; then
    # 获取本地网络信息
    LOCAL_IP=$(hostname -I | awk '{print $1}')
    LOCAL_NET=$(echo $LOCAL_IP | sed 's/\.[0-9]*$/.0\/24/')
    echo "扫描网络: $LOCAL_NET"
    nmap -p 80,443,554,8080,8554 --open -q $LOCAL_NET 2>/dev/null | grep -E "^Nmap|^Host|^[0-9]"
else
    echo "nmap 未安装，跳过扫描"
fi

echo ""
echo "诊断完成！"
