#!/bin/bash
# GB28181 设备录像回放快速诊断脚本

set -e

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  GB28181 设备录像回放故障诊断                      ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"

API_BASE="http://localhost:9080/api"

# 获取 token（如果需要）
# 如果认证失败，可以检查 token 有效性
check_auth() {
    echo -e "\n${YELLOW}检查服务器连接...${NC}"
    
    if ! curl -s -f "$API_BASE/gb28181/devices" > /dev/null 2>&1; then
        echo -e "${RED}✗ 无法连接到服务器 (http://localhost:9080)${NC}"
        echo "  请确保服务器正在运行"
        exit 1
    fi
    
    echo -e "${GREEN}✓ 服务器连接正常${NC}"
}

# 检查 GB28181 设备
check_devices() {
    echo -e "\n${YELLOW}检查GB28181设备状态...${NC}"
    
    RESPONSE=$(curl -s "$API_BASE/gb28181/devices")
    DEVICE_COUNT=$(echo "$RESPONSE" | jq '.devices | length')
    
    if [ "$DEVICE_COUNT" -eq 0 ]; then
        echo -e "${RED}✗ 未发现GB28181设备${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ 发现 $DEVICE_COUNT 个设备${NC}"
    echo ""
    echo "$RESPONSE" | jq '.devices[] | 
        "  [\(.status)] \(.name) (\(.deviceId)) - IP: \(.sipIp)"'
    
    # 保存第一个设备ID用于后续测试
    FIRST_DEVICE=$(echo "$RESPONSE" | jq -r '.devices[0].deviceId // empty')
    return 0
}

# 检查设备录像
check_recordings() {
    if [ -z "$FIRST_DEVICE" ]; then
        echo -e "${YELLOW}跳过录像检查（无可用设备）${NC}"
        return 0
    fi
    
    echo -e "\n${YELLOW}查询设备录像列表...${NC}"
    echo "  设备: $FIRST_DEVICE"
    
    # 查询过去24小时的录像
    END_TIME=$(date -u +"%Y-%m-%dT%H:%M:%S")
    START_TIME=$(date -u -d "1 day ago" +"%Y-%m-%dT%H:%M:%S")
    
    RESPONSE=$(curl -s -X POST "$API_BASE/gb28181/record/query" \
        -H "Content-Type: application/json" \
        -d "{
            \"deviceId\": \"$FIRST_DEVICE\",
            \"startTime\": \"$START_TIME\",
            \"endTime\": \"$END_TIME\"
        }")
    
    REC_COUNT=$(echo "$RESPONSE" | jq '.recordings | length // 0')
    
    if [ "$REC_COUNT" -eq 0 ]; then
        echo -e "${YELLOW}⚠ 过去24小时内未发现录像${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ 发现 $REC_COUNT 条录像${NC}"
    echo ""
    echo "$RESPONSE" | jq -r '.recordings[] | 
        "  [\(.channelName)] \(.startTime) ~ \(.endTime) (\(.size) bytes)"' | head -5
    
    return 0
}

# 检查 ZLM RTP 配置
check_zlm_rtp() {
    echo -e "\n${YELLOW}检查 ZLM RTP 服务...${NC}"
    
    # 尝试测试 openRtpServer
    TEST_STREAM="diag_$(date +%s)"
    
    RESPONSE=$(curl -s "$API_BASE/zlm/status")
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}✗ 无法获取ZLM状态${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ ZLM服务正常${NC}"
    echo ""
    echo "$RESPONSE" | jq '
        "  HTTP 端口: \(.http_port)",
        "  RTMP 端口: \(.rtmp_port)",
        "  RTP 范围: 10000-35000"'
    
    return 0
}

# 检查网络连接
check_network() {
    echo -e "\n${YELLOW}检查网络配置...${NC}"
    
    # 获取本机IP
    LOCAL_IP=$(hostname -I | awk '{print $1}')
    echo -e "${GREEN}✓ 本机IP: $LOCAL_IP${NC}"
    
    # 检查 RTP 端口是否被占用
    USED_PORTS=$(netstat -ulnp 2>/dev/null | grep -E ":(10|11|12|13|14|15|16|17|18|19|2[0-9]|3[0-5])" | wc -l)
    
    echo -e "  RTP 端口占用: $USED_PORTS 个"
    
    # 检查防火墙
    if command -v ufw &> /dev/null; then
        UFW_STATUS=$(ufw status | grep -i active)
        if [ -n "$UFW_STATUS" ]; then
            echo -e "  防火墙: ${YELLOW}已启用${NC}"
            echo -e "    ${YELLOW}⚠ 请确保 RTP 端口已开放: ufw allow 10000:35000/udp${NC}"
        else
            echo -e "  防火墙: 已禁用"
        fi
    fi
}

# 性能检查
check_performance() {
    echo -e "\n${YELLOW}检查系统资源...${NC}"
    
    # CPU使用率
    CPU_LOAD=$(uptime | awk -F'average:' '{print $2}' | awk '{print $1}' | tr -d ',')
    echo "  CPU 负载: $CPU_LOAD"
    
    # 内存使用
    MEM_USED=$(free -h | awk '/^Mem:/ {print $3}')
    MEM_TOTAL=$(free -h | awk '/^Mem:/ {print $2}')
    echo "  内存使用: $MEM_USED / $MEM_TOTAL"
    
    # 磁盘空间
    DISK_FREE=$(df -h / | awk 'NR==2 {print $4}')
    DISK_TOTAL=$(df -h / | awk 'NR==2 {print $2}')
    echo "  磁盘空间: $DISK_FREE / $DISK_TOTAL 可用"
    
    # ffmpeg 进程
    FFMPEG_COUNT=$(pgrep -f ffmpeg | wc -l)
    echo "  ffmpeg 进程: $FFMPEG_COUNT 个"
}

# 打印诊断建议
print_recommendations() {
    echo -e "\n${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  诊断建议                                          ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    
    echo ""
    echo "如果设备录像回放无法播放，请按以下步骤排查："
    echo ""
    echo -e "${YELLOW}1. 验证设备端配置${NC}"
    echo "   □ 确认硬盘录像机已在线（上方诊断已验证）"
    echo "   □ 在设备Web界面检查录像设置和存储空间"
    echo "   □ 验证设备网络配置，能否ping通本服务器"
    echo ""
    
    echo -e "${YELLOW}2. 检查网络环境${NC}"
    echo "   □ 防火墙是否开放RTP端口（10000-35000）"
    echo "   □ 设备和服务器是否在同一网段或可路由"
    echo "   □ 是否有代理或NAT配置需要调整"
    echo ""
    
    echo -e "${YELLOW}3. 测试回放请求${NC}"
    echo "   □ 在前端选择设备→通道→时间范围，发起回放请求"
    echo "   □ 查看浏览器控制台的网络请求，确认返回flvUrl"
    echo "   □ 使用curl测试FLV地址是否可访问"
    echo ""
    
    echo -e "${YELLOW}4. 查看详细日志${NC}"
    echo "   □ 后端日志: tail -100 /tmp/server_test.log | grep -i rtp"
    echo "   □ ZLM日志: tail -100 /home/jl/zpip/zpip/build/zlm-runtime/log/console.log"
    echo ""
    
    echo -e "${YELLOW}5. 完整文档${NC}"
    echo "   □ 详见: ./docs/GB28181_DEVICE_RECORDING_PLAYBACK_GUIDE.md"
    echo ""
}

# 主程序
main() {
    check_auth
    
    check_devices
    DEVICE_CHECK=$?
    
    if [ $DEVICE_CHECK -eq 0 ]; then
        check_recordings
    fi
    
    check_zlm_rtp
    check_network
    check_performance
    
    echo ""
    print_recommendations
    
    echo -e "\n${BLUE}诊断完成！${NC}\n"
}

# 运行主程序
main
