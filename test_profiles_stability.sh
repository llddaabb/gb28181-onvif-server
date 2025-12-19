#!/bin/bash

# ONVIF 获取配置文件稳定性测试脚本
# 用于测试改进后的获取Profiles功能

echo "========================================"
echo "ONVIF 获取配置文件稳定性测试"
echo "========================================"
echo ""

# 检查参数
if [ $# -lt 2 ]; then
    echo "使用方法: $0 <设备IP> <设备端口> [用户名] [密码]"
    echo "例如: $0 192.168.1.232 8080 admin a123456789"
    echo ""
    exit 1
fi

DEVICE_IP=$1
DEVICE_PORT=$2
USERNAME=${3:-admin}
PASSWORD=${4:-a123456789}
DEVICE_ID="${DEVICE_IP}:${DEVICE_PORT}"
BASE_URL="http://localhost:8080/api"

echo "测试设备信息:"
echo "  设备ID: $DEVICE_ID"
echo "  用户名: $USERNAME"
echo "  密码: $PASSWORD"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 函数：打印日志
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# 测试 1: 检查设备是否存在
log_info "测试 1: 检查设备是否存在..."
RESPONSE=$(curl -s "${BASE_URL}/onvif/devices" | grep -c "$DEVICE_ID")
if [ "$RESPONSE" -gt 0 ]; then
    log_success "设备存在"
else
    log_warning "设备不存在，尝试添加设备..."
    curl -s -X POST "${BASE_URL}/onvif/devices" \
        -H "Content-Type: application/json" \
        -d "{
            \"ip\": \"$DEVICE_IP\",
            \"port\": $DEVICE_PORT,
            \"username\": \"$USERNAME\",
            \"password\": \"$PASSWORD\"
        }" | grep -q "success"
    
    if [ $? -eq 0 ]; then
        log_success "设备添加成功"
    else
        log_error "设备添加失败"
        exit 1
    fi
fi
echo ""

# 测试 2: 获取配置文件 (第一次)
log_info "测试 2: 获取配置文件 (第一次)..."
START_TIME=$(date +%s%N | cut -b1-13)
RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/onvif/devices/${DEVICE_ID}/profiles")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)
END_TIME=$(date +%s%N | cut -b1-13)
ELAPSED=$((END_TIME - START_TIME))

if [ "$HTTP_CODE" -eq 200 ]; then
    PROFILE_COUNT=$(echo "$BODY" | grep -o '"token"' | wc -l)
    log_success "获取成功 (耗时: ${ELAPSED}ms, 配置数: $PROFILE_COUNT)"
    echo "$BODY" | head -c 200
    echo ""
else
    log_error "获取失败 (HTTP $HTTP_CODE, 耗时: ${ELAPSED}ms)"
    echo "$BODY" | head -c 200
    echo ""
fi
echo ""

# 测试 3: 获取配置文件 (第二次 - 测试缓存)
log_info "测试 3: 获取配置文件 (第二次 - 测试缓存/稳定性)..."
START_TIME=$(date +%s%N | cut -b1-13)
RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/onvif/devices/${DEVICE_ID}/profiles")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)
END_TIME=$(date +%s%N | cut -b1-13)
ELAPSED=$((END_TIME - START_TIME))

if [ "$HTTP_CODE" -eq 200 ]; then
    PROFILE_COUNT=$(echo "$BODY" | grep -o '"token"' | wc -l)
    log_success "获取成功 (耗时: ${ELAPSED}ms, 配置数: $PROFILE_COUNT)"
    echo "$BODY" | head -c 200
    echo ""
else
    log_error "获取失败 (HTTP $HTTP_CODE, 耗时: ${ELAPSED}ms)"
fi
echo ""

# 测试 4: 测试多次并发请求 (压力测试)
log_info "测试 4: 测试多次请求稳定性 (共5次)..."
SUCCESS_COUNT=0
FAIL_COUNT=0

for i in {1..5}; do
    RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/onvif/devices/${DEVICE_ID}/profiles")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [ "$HTTP_CODE" -eq 200 ]; then
        ((SUCCESS_COUNT++))
        echo -e "  ${GREEN}[#$i]${NC} 成功"
    else
        ((FAIL_COUNT++))
        echo -e "  ${RED}[#$i]${NC} 失败 (HTTP $HTTP_CODE)"
    fi
    sleep 0.5
done

echo ""
log_info "稳定性测试结果:"
if [ $SUCCESS_COUNT -eq 5 ]; then
    log_success "所有请求成功 ($SUCCESS_COUNT/$((SUCCESS_COUNT + FAIL_COUNT)))"
else
    log_warning "部分请求失败 ($SUCCESS_COUNT/$((SUCCESS_COUNT + FAIL_COUNT)) 成功)"
fi
echo ""

# 总结
echo "========================================"
echo "测试完成"
echo "========================================"
echo ""
echo "建议:"
if [ $SUCCESS_COUNT -lt 5 ]; then
    log_warning "获取配置文件成功率不够高，请检查："
    echo "  1. 设备是否在线"
    echo "  2. 凭证是否正确"
    echo "  3. 网络连接是否稳定"
    echo "  4. 查看后端日志: tail -f logs/debug.log | grep ONVIF"
else
    log_success "获取配置文件稳定性良好！"
fi
echo ""
