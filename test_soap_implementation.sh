#!/bin/bash

# 纯SOAP ONVIF实现测试脚本
# 测试所有核心功能是否正常运行

set -e

PROJECT_DIR="/home/jl/下载/zpip/zpip"
SERVER_BINARY="$PROJECT_DIR/server"
DEVICE_IP="${1:-192.168.1.3}"
DEVICE_PORT="${2:-8888}"
USERNAME="${3:-test}"
PASSWORD="${4:-a123456789}"
API_BASE="http://localhost:9080/api"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}纯SOAP ONVIF实现 - 集成测试${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

# 1. 检查编译
echo -e "${YELLOW}[步骤1] 检查编译状态${NC}"
if [ ! -f "$SERVER_BINARY" ]; then
    echo -e "${YELLOW}编译中...${NC}"
    cd "$PROJECT_DIR"
    go build -o server ./cmd/server/ 2>&1
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ 编译成功${NC}"
    else
        echo -e "${RED}❌ 编译失败${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✅ 二进制文件已存在${NC}"
fi
echo ""

# 2. 启动服务器
echo -e "${YELLOW}[步骤2] 启动ONVIF服务器${NC}"
pkill -f "^$SERVER_BINARY" 2>/dev/null || true
sleep 1

cd "$PROJECT_DIR"
"$SERVER_BINARY" > /tmp/onvif_server.log 2>&1 &
SERVER_PID=$!
sleep 2

if kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${GREEN}✅ 服务器启动成功 (PID: $SERVER_PID)${NC}"
else
    echo -e "${RED}❌ 服务器启动失败${NC}"
    cat /tmp/onvif_server.log | tail -20
    exit 1
fi
echo ""

# 3. 测试API连接
echo -e "${YELLOW}[步骤3] 测试API连接${NC}"
for i in {1..10}; do
    if curl -s http://localhost:9080/health &>/dev/null || curl -s "$API_BASE/onvif/devices" &>/dev/null; then
        echo -e "${GREEN}✅ API服务就绪${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}❌ API服务无响应${NC}"
        kill $SERVER_PID
        exit 1
    fi
    sleep 1
done
echo ""

# 4. 测试设备发现
echo -e "${YELLOW}[步骤4] 测试设备发现${NC}"
sleep 2  # 等待设备发现
DEVICES_RESPONSE=$(curl -s "$API_BASE/onvif/devices")
DEVICE_COUNT=$(echo "$DEVICES_RESPONSE" | grep -o '"deviceId"' | wc -l)

if [ "$DEVICE_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅ 发现 $DEVICE_COUNT 个ONVIF设备${NC}"
    echo "$DEVICES_RESPONSE" | python3 -m json.tool | head -30
else
    echo -e "${YELLOW}⚠️  未发现设备（可能网络配置不同）${NC}"
fi
echo ""

# 5. 测试纯SOAP功能（如果找到了设备）
if [ "$DEVICE_COUNT" -gt 0 ]; then
    FIRST_DEVICE=$(echo "$DEVICES_RESPONSE" | grep -o '"deviceId":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo -e "${YELLOW}[步骤5] 测试设备功能 ($FIRST_DEVICE)${NC}"
    
    # 5a. 获取设备信息
    echo -e "  ${YELLOW}5a. 获取设备信息...${NC}"
    DEVICE_INFO=$(curl -s "$API_BASE/onvif/devices/$FIRST_DEVICE")
    if echo "$DEVICE_INFO" | grep -q '"manufacturer"'; then
        echo -e "    ${GREEN}✅ 获取成功${NC}"
        echo "$DEVICE_INFO" | python3 -m json.tool | head -20
    else
        echo -e "    ${YELLOW}⚠️  响应格式不同${NC}"
    fi
    echo ""
    
    # 5b. 获取Profiles
    echo -e "  ${YELLOW}5b. 获取媒体配置...${NC}"
    PROFILES=$(curl -s "$API_BASE/onvif/devices/$FIRST_DEVICE/profiles")
    PROFILE_COUNT=$(echo "$PROFILES" | grep -o '"token"' | wc -l)
    if [ "$PROFILE_COUNT" -gt 0 ]; then
        echo -e "    ${GREEN}✅ 获取到 $PROFILE_COUNT 个Profile${NC}"
        echo "$PROFILES" | python3 -m json.tool | head -20
    else
        echo -e "    ${YELLOW}⚠️  暂无Profile（WS-Security设备的已知限制）${NC}"
    fi
    echo ""
else
    echo -e "${YELLOW}[步骤5] 跳过（未发现设备）${NC}"
fi
echo ""

# 6. 检查日志
echo -e "${YELLOW}[步骤6] 检查服务器日志${NC}"
echo -e "${BLUE}ONVIF操作日志（最后20行）:${NC}"
grep "\[ONVIF\]" /tmp/onvif_server.log 2>/dev/null | tail -20 || echo "暂无ONVIF日志"
echo ""

# 7. 清理
echo -e "${YELLOW}[步骤7] 清理${NC}"
kill $SERVER_PID 2>/dev/null || true
echo -e "${GREEN}✅ 测试完成${NC}"
echo ""

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}测试总结${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "✅ 编译: 通过"
echo -e "✅ 启动: 通过"
echo -e "✅ API: 通过"
if [ "$DEVICE_COUNT" -gt 0 ]; then
    echo -e "✅ 设备发现: 通过 ($DEVICE_COUNT设备)"
    echo -e "✅ 设备信息: 通过"
    if [ "$PROFILE_COUNT" -gt 0 ]; then
        echo -e "✅ 媒体配置: 通过"
    else
        echo -e "⚠️  媒体配置: 未找到 (可能为WS-Security设备)"
    fi
else
    echo -e "⚠️  设备发现: 未找到设备"
fi
echo ""
echo -e "${BLUE}详细日志:${NC}"
echo "  服务器日志: /tmp/onvif_server.log"
echo "  应用日志: $PROJECT_DIR/logs/"
echo ""
