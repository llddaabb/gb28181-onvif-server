#!/bin/bash
# 测试多网络接口的 ONVIF 发现 - 完整测试脚本

set -e

WORKSPACE="/home/jl/下载/zpip/zpip"
cd "$WORKSPACE"

echo "════════════════════════════════════════════════════════════════"
echo "多网络接口 ONVIF 发现测试"
echo "════════════════════════════════════════════════════════════════"
echo ""

# 1. 显示当前网络配置
echo "1️⃣  当前网络接口配置:"
echo "─────────────────────────────────────────────────────────────────"
ip -4 addr show | grep -E "^[0-9]:|inet " | sed 's/^[0-9]*: /  Interface: /g; s/^    inet /    IP: /g'
echo ""

# 2. 检查多播支持
echo "2️⃣  检查多播 (MULTICAST) 支持:"
echo "─────────────────────────────────────────────────────────────────"
for iface in $(ip link show | grep "^[0-9]" | awk '{print $2}' | sed 's/:$//'); do
    flags=$(ip link show "$iface" | grep "^$iface" | grep -o '<[^>]*>' | tr -d '<>')
    if [[ $flags == *"MULTICAST"* ]] && [[ $flags == *"UP"* ]]; then
        echo "  ✓ $iface: 支持多播"
    fi
done
echo ""

# 3. 构建服务器
echo "3️⃣  编译服务器..."
echo "─────────────────────────────────────────────────────────────────"
if go build -o server cmd/server/main.go 2>&1 | head -5; then
    echo "  ✓ 编译成功"
else
    echo "  ✗ 编译失败"
    exit 1
fi
echo ""

# 4. 启动服务器并观察发现
echo "4️⃣  启动服务器并观察 ONVIF 发现过程 (30秒)..."
echo "─────────────────────────────────────────────────────────────────"

# 清理旧进程
pkill -f "./server" 2>/dev/null || true
sleep 2

# 启动服务器
./server > /tmp/multi_interface_test.log 2>&1 &
SERVER_PID=$!
echo "  服务器启动中 (PID: $SERVER_PID)"

# 等待发现执行
sleep 30

# 停止服务器
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
echo "  ✓ 测试完成"
echo ""

# 5. 分析日志
echo "5️⃣  发现日志分析:"
echo "─────────────────────────────────────────────────────────────────"

# 统计发现次数
DISCOVERY_COUNT=$(grep -c "Starting ONVIF discovery" /tmp/multi_interface_test.log || echo 0)
echo "  发现执行次数: $DISCOVERY_COUNT"

# 显示接口发现情况
echo ""
echo "  接口发现情况:"
grep "Starting ONVIF WS-Discovery on" /tmp/multi_interface_test.log | sed 's/^/    /'

# 显示接口 IP 信息
echo ""
echo "  接口 IP 地址:"
grep "Interface.*IPs:" /tmp/multi_interface_test.log | head -1 | sed 's/^/    /'

# 查找已发现的设备
echo ""
echo "  已发现的 ONVIF 设备:"
FOUND_DEVICES=$(grep "Found total devices\|Parsed device" /tmp/multi_interface_test.log || echo "无")
if [ "$FOUND_DEVICES" = "无" ]; then
    echo "    (未发现任何 ONVIF 设备)"
    echo "    注: 这很正常，除非网络中有真实的 ONVIF 设备"
else
    echo "$FOUND_DEVICES" | sed 's/^/    /'
fi

echo ""
echo "6️⃣  完整 ONVIF 日志:"
echo "─────────────────────────────────────────────────────────────────"
grep "ONVIF\|multicast\|Interface" /tmp/multi_interface_test.log | sed 's/^/  /'

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "测试完成！"
echo "完整日志可在以下文件查看: /tmp/multi_interface_test.log"
echo "════════════════════════════════════════════════════════════════"
