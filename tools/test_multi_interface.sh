#!/bin/bash
# 测试多网络接口的 ONVIF 发现功能

echo "=== 多网络接口 ONVIF 发现测试 ==="
echo ""

# 显示所有网络接口
echo "1. 当前系统的网络接口:"
ip addr show | grep -E "^[0-9]:|inet |inet6" | grep -v "127.0.0.1"
echo ""

# 显示哪些接口支持多播
echo "2. 检查多播支持的接口:"
netstat -i 2>/dev/null | grep -v "Iface\|Kernel" || ip link show | grep -E "^[0-9].*UP"
echo ""

# 检查路由表
echo "3. 检查网络路由:"
ip route show | head -10
echo ""

# 尝试 ping 多播地址以检查多播支持
echo "4. 尝试多播地址 239.255.255.250:3702:"
# 使用 socat 进行简单的多播测试
if command -v socat &> /dev/null; then
    (echo "test" | timeout 2 socat - UDP4-DATAGRAM:239.255.255.250:3702 2>&1) || echo "多播可能被防火墙阻止"
else
    echo "socat 未安装，跳过"
fi

echo ""
echo "5. 启动服务器并观察 ONVIF 发现日志 (10秒):"
pkill -f "./server" 2>/dev/null || true
sleep 1

cd /home/jl/下载/zpip/zpip

# 启动服务器并记录日志
./server > /tmp/discovery_test.log 2>&1 &
SERVER_PID=$!

# 等待启动
sleep 10

# 杀死服务器
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null

# 显示相关日志
echo ""
echo "6. ONVIF 发现相关日志:"
grep -i "interface\|multicast\|multicast\|listening\|Starting ONVIF\|found.*device\|probe" /tmp/discovery_test.log || echo "未找到发现日志"

echo ""
echo "完成测试。"
