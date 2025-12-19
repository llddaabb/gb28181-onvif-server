#!/bin/bash
# onvif_test.sh - 完整的 ONVIF 功能测试脚本

# 不在错误时立即退出，我们需要继续测试
set +e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 默认配置
DEVICE_IP="${1:-192.168.1.232}"
USERNAME="${2:-admin}"
PASSWORD="${3:-a123456789}"
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
TEST_HTTPS="${TEST_HTTPS:-true}"  # 是否测试HTTPS端点
SSL_VERIFY="${SSL_VERIFY:-false}"  # 是否验证SSL证书

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 日志函数
log_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    printf "${CYAN}║ %-58s ║${NC}\n" "$1"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

log_section() {
    echo ""
    echo -e "${BLUE}【$1】${NC}"
    echo -e "${BLUE}────────────────────────────────────────────────────────────${NC}"
}

log_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    printf "  %-50s: " "$1"
}

log_pass() {
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✓ PASS${NC} $1"
}

log_fail() {
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}✗ FAIL${NC} $1"
}

log_info() {
    echo -e "    ${CYAN}ℹ${NC} $1"
}

log_warn() {
    if [ -z "$1" ]; then
        echo -e "${YELLOW}⚠ WARN${NC}"
    else
        echo -e "    ${YELLOW}⚠${NC} $1"
    fi
}

# 测试辅助函数
test_port() {
    local ip=$1
    local port=$2
    timeout 2 bash -c "cat < /dev/null > /dev/tcp/$ip/$port" 2>/dev/null
    return $?
}

test_http() {
    local url=$1
    local curl_opts="-s -o /dev/null -w %{http_code} --max-time 3"
    
    # 如果是HTTPS且不验证证书，添加-k选项
    if [[ "$url" == https://* ]] && [ "$SSL_VERIFY" = "false" ]; then
        curl_opts="$curl_opts -k"
    fi
    
    local code=$(curl $curl_opts "$url" 2>/dev/null)
    if [ -z "$code" ] || [ "$code" = "000" ]; then
        echo "000"
    else
        echo "$code"
    fi
}

test_http_get() {
    local url=$1
    curl -s --max-time 5 "$url" 2>/dev/null
}

test_http_post() {
    local url=$1
    local data=$2
    curl -s --max-time 5 -X POST "$url" \
        -H "Content-Type: application/json" \
        -d "$data" 2>/dev/null
}

test_soap() {
    local endpoint=$1
    local username=$2
    local password=$3
    local show_debug=${4:-false}
    
    local soap_request='<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/></s:Body></s:Envelope>'
    
    # 设置curl选项，如果是HTTPS且不验证证书则添加-k
    local curl_opts="-s --max-time 5"
    if [[ "$endpoint" == https://* ]] && [ "$SSL_VERIFY" = "false" ]; then
        curl_opts="$curl_opts -k"
    fi
    
    # 先尝试无认证
    local response=$(curl $curl_opts -X POST "$endpoint" \
        -H "Content-Type: application/soap+xml; charset=utf-8" \
        -d "$soap_request" \
        2>/dev/null)
    
    if [ "$show_debug" = "true" ]; then
        echo "" >&2
        echo "  调试: 无认证响应片段: $(echo "$response" | head -c 200)" >&2
    fi
    
    if echo "$response" | grep -q "GetSystemDateAndTimeResponse"; then
        if [ "$show_debug" = "true" ]; then
            echo "  调试: ✓ 无认证成功" >&2
        fi
        return 0
    fi
    
    # 如果失败，尝试使用 Digest 认证
    if [ -n "$username" ] && [ -n "$password" ]; then
        response=$(curl $curl_opts -X POST "$endpoint" \
            --digest -u "$username:$password" \
            -H "Content-Type: application/soap+xml; charset=utf-8" \
            -d "$soap_request" \
            2>/dev/null)
        
        if [ "$show_debug" = "true" ]; then
            echo "  调试: Digest认证响应片段: $(echo "$response" | head -c 200)" >&2
        fi
        
        if echo "$response" | grep -q "GetSystemDateAndTimeResponse"; then
            if [ "$show_debug" = "true" ]; then
                echo "  调试: ✓ Digest认证成功" >&2
            fi
            return 0
        fi
        
        # 尝试 Basic 认证
        response=$(curl $curl_opts -X POST "$endpoint" \
            --basic -u "$username:$password" \
            -H "Content-Type: application/soap+xml; charset=utf-8" \
            -d "$soap_request" \
            2>/dev/null)
        
        if [ "$show_debug" = "true" ]; then
            echo "  调试: Basic认证响应片段: $(echo "$response" | head -c 200)" >&2
        fi
        
        if echo "$response" | grep -q "GetSystemDateAndTimeResponse"; then
            if [ "$show_debug" = "true" ]; then
                echo "  调试: ✓ Basic认证成功" >&2
            fi
            return 0
        fi
        
        # 尝试 WS-Security (UsernameToken) 认证
        if [ "$show_debug" = "true" ]; then
            echo "  调试: 尝试 WS-Security UsernameToken 认证..." >&2
        fi
        
        # 生成 WS-Security 认证参数
        local nonce=$(openssl rand -base64 16 2>/dev/null || head -c 16 /dev/urandom | base64)
        local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
        local digest=$(echo -n "$(echo "$nonce" | base64 -d 2>/dev/null)${timestamp}${password}" | openssl sha1 -binary 2>/dev/null | base64)
        
        # 如果 digest 生成失败，使用简化方案
        if [ -z "$digest" ]; then
            digest=$(echo -n "${password}" | base64)
        fi
        
        local wsse_request="<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<s:Envelope xmlns:s=\"http://www.w3.org/2003/05/soap-envelope\">
  <s:Header>
    <Security s:mustUnderstand=\"1\" xmlns=\"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd\">
      <UsernameToken>
        <Username>$username</Username>
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
        
        response=$(curl $curl_opts -X POST "$endpoint" \
            -H "Content-Type: application/soap+xml; charset=utf-8" \
            -d "$wsse_request" \
            2>/dev/null)
        
        if [ "$show_debug" = "true" ]; then
            echo "  调试: WS-Security响应片段: $(echo "$response" | head -c 200)" >&2
        fi
        
        if echo "$response" | grep -q "GetSystemDateAndTimeResponse"; then
            if [ "$show_debug" = "true" ]; then
                echo "  调试: ✓ WS-Security认证成功" >&2
            fi
            return 0
        fi
    fi
    
    if [ "$show_debug" = "true" ]; then
        echo "  调试: ✗ 所有认证方式均失败" >&2
    fi
    return 1
}

# 显示测试配置
log_header "ONVIF 设备完整功能测试"
echo "测试配置:"
echo "  设备地址: $DEVICE_IP"
echo "  用户名: $USERNAME"
echo "  服务器: $SERVER_URL"
echo "  测试HTTPS: $TEST_HTTPS"
echo "  SSL验证: $SSL_VERIFY"

# 第1部分: 网络连通性测试
log_section "网络连通性测试"

log_test "设备 Ping 测试"
if ping -c 3 -W 2 $DEVICE_IP &>/dev/null; then
    log_pass ""
else
    log_fail "设备无法 ping 通"
    log_warn "网络连接失败，部分测试将无法进行"
fi

# 端口扫描
OPEN_PORTS=()
for port in 80 443 554 8080 8081 8899 8888 5000 9000; do
    log_test "端口 $port"
    if test_port $DEVICE_IP $port; then
        log_pass ""
        OPEN_PORTS+=($port)
    else
        log_fail ""
    fi
done

if [ ${#OPEN_PORTS[@]} -eq 0 ]; then
    log_warn "没有发现开放的端口，ONVIF 测试可能失败"
else
    log_info "开放端口: ${OPEN_PORTS[*]}"
fi

# 第2部分: ONVIF 端点发现
log_section "ONVIF 端点发现"

ONVIF_PATHS=(
    "/onvif/device_service"
    "/onvif/services"
    "/onvif/device"
    "/onvif-http/device_service"
    "/onvif1/device_service"
    "/cgi-bin/onvif/device_service"
    "/ONVIF/device_service"
    "/Onvif/device_service"
)

VALID_ENDPOINTS=()
RECOMMENDED_ENDPOINT=""

for port in "${OPEN_PORTS[@]}"; do
    [[ $port -eq 554 ]] && continue
    
    for path in "${ONVIF_PATHS[@]}"; do
        # 测试HTTP端点
        endpoint="http://$DEVICE_IP:$port$path"
        log_test "测试 $endpoint"
        http_code=$(test_http "$endpoint")
        
        # 判断端点状态
        if [[ $http_code -ge 200 && $http_code -lt 300 ]]; then
            # 2xx - 成功
            log_pass "HTTP $http_code"
            if [[ "$path" == *"device_service"* || "$path" == "/onvif/device" ]]; then
                VALID_ENDPOINTS+=("$endpoint")
            fi
        elif [[ $http_code -eq 401 || $http_code -eq 403 ]]; then
            # 401/403 - 需要认证（端点存在）
            log_pass "HTTP $http_code (需要认证)"
            if [[ "$path" == *"device_service"* || "$path" == "/onvif/device" ]]; then
                VALID_ENDPOINTS+=("$endpoint")
            fi
        elif [[ $http_code -ge 300 && $http_code -lt 400 ]]; then
            # 3xx - 重定向
            log_warn "HTTP $http_code (重定向)"
        elif [[ $http_code -eq 404 ]]; then
            # 404 - 不存在
            log_fail "HTTP $http_code (未找到)"
        elif [[ $http_code -ge 500 && $http_code -lt 600 ]]; then
            # 5xx - 服务器错误
            log_fail "HTTP $http_code (服务器错误)"
        elif [[ $http_code -eq 0 || $http_code -eq 000 ]]; then
            # 000 - 连接失败
            log_fail "连接失败"
        else
            # 其他错误
            log_fail "HTTP $http_code"
        fi
        
        # 如果端口是443或TEST_HTTPS=true，也测试HTTPS
        if [ "$TEST_HTTPS" = "true" ] && [[ $port -eq 443 || $port -eq 80 ]]; then
            https_endpoint="https://$DEVICE_IP:$port$path"
            log_test "测试 $https_endpoint"
            https_code=$(test_http "$https_endpoint")
            
            if [[ $https_code -ge 200 && $https_code -lt 300 ]]; then
                log_pass "HTTPS $https_code"
                if [[ "$path" == *"device_service"* || "$path" == "/onvif/device" ]]; then
                    VALID_ENDPOINTS+=("$https_endpoint")
                fi
            elif [[ $https_code -eq 401 || $https_code -eq 403 ]]; then
                log_pass "HTTPS $https_code (需要认证)"
                if [[ "$path" == *"device_service"* || "$path" == "/onvif/device" ]]; then
                    VALID_ENDPOINTS+=("$https_endpoint")
                fi
            elif [[ $https_code -ge 300 && $https_code -lt 400 ]]; then
                log_warn "HTTPS $https_code (重定向)"
            elif [[ $https_code -eq 404 ]]; then
                log_fail "HTTPS $https_code (未找到)"
            elif [[ $https_code -ge 500 && $https_code -lt 600 ]]; then
                log_fail "HTTPS $https_code (服务器错误)"
            elif [[ $https_code -eq 0 || $https_code -eq 000 ]]; then
                log_fail "HTTPS连接失败"
            else
                log_fail "HTTPS $https_code"
            fi
        fi
    done
done

log_info "发现 ${#VALID_ENDPOINTS[@]} 个有效端点"

# 第3部分: ONVIF SOAP 接口测试
log_section "ONVIF SOAP 接口测试"

if [ ${#VALID_ENDPOINTS[@]} -eq 0 ]; then
    log_warn "未发现有效的 ONVIF 端点，跳过 SOAP 测试"
    log_info "建议: 尝试使用 WS-Discovery 发现真实的设备端点"
else
    echo ""
    log_info "开始详细 SOAP 测试（将显示调试信息）..."
    for endpoint in "${VALID_ENDPOINTS[@]}"; do
        log_test "SOAP 测试: $endpoint"
        if test_soap "$endpoint" "$USERNAME" "$PASSWORD" "true"; then
            log_pass "(认证成功)"
            if [ -z "$RECOMMENDED_ENDPOINT" ]; then
                RECOMMENDED_ENDPOINT="$endpoint"
                log_info "推荐端点: $RECOMMENDED_ENDPOINT"
            fi
        else
            log_fail "(SOAP 响应无效或认证失败)"
        fi
    done
fi

# 第4部分: WS-Discovery 测试
log_section "WS-Discovery 多播发现"

cat > /tmp/ws_discovery_probe.xml <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action>
    <a:MessageID>uuid:test-probe-12345</a:MessageID>
    <a:ReplyTo>
      <a:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:Address>
    </a:ReplyTo>
    <a:To s:mustUnderstand="1">urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To>
  </s:Header>
  <s:Body>
    <Probe xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery">
      <d:Types xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dp0="http://www.onvif.org/ver10/network/wsdl">dp0:NetworkVideoTransmitter</d:Types>
    </Probe>
  </s:Body>
</s:Envelope>
EOF

log_test "WS-Discovery Probe"
log_info "正在监听多播地址 239.255.255.250:3702..."
DISCOVERY_RESPONSE=$(timeout 5 bash -c "
    exec 3<>/dev/udp/239.255.255.250/3702
    cat /tmp/ws_discovery_probe.xml >&3
    timeout 3 cat <&3
" 2>/dev/null || echo "")

if echo "$DISCOVERY_RESPONSE" | grep -q 'XAddrs'; then
    log_pass "收到设备响应"
    DISCOVERED_XADDR=$(echo "$DISCOVERY_RESPONSE" | grep -oP 'XAddrs[^<]*' | head -1 | sed 's/XAddrs>//' || echo "")
    if [ -n "$DISCOVERED_XADDR" ]; then
        log_info "✓ 发现的真实 XAddr: $DISCOVERED_XADDR"
        log_info "建议: 使用此地址进行 ONVIF 连接"
        # 如果之前没有推荐端点，使用发现的端点
        if [ -z "$RECOMMENDED_ENDPOINT" ]; then
            RECOMMENDED_ENDPOINT="$DISCOVERED_XADDR"
        fi
    fi
else
    log_fail "未收到响应或超时"
    log_warn "可能原因: 1) 设备不支持WS-Discovery 2) 网络多播被阻止 3) 设备在不同网段"
fi

# 第5部分: API 服务器测试
log_section "API 服务器连通性测试"

log_test "服务器健康检查"
health_response=$(test_http_get "$SERVER_URL/api/health" || echo "")
if echo "$health_response" | grep -q '"status"'; then
    log_pass ""
    SERVER_AVAILABLE=true
else
    log_fail "服务器未响应"
    log_warn "API 服务器未启动，跳过 API 相关测试"
    SERVER_AVAILABLE=false
fi

# 第6部分: ONVIF 设备管理 API 测试
if [ "$SERVER_AVAILABLE" = "true" ]; then
    log_section "ONVIF 设备管理 API"

    log_test "获取 ONVIF 设备列表"
devices_response=$(test_http_get "$SERVER_URL/api/onvif/devices")
if echo "$devices_response" | grep -q '\['; then
    log_pass ""
    device_count=$(echo "$devices_response" | grep -o '"id"' | wc -l || echo "0")
    log_info "当前设备数: $device_count"
else
    log_fail ""
fi

log_test "触发 ONVIF 设备发现"
discover_response=$(test_http_post "$SERVER_URL/api/onvif/discover" "{}")
if echo "$discover_response" | grep -q 'success'; then
    log_pass ""
else
    log_fail ""
fi

sleep 3

# 添加设备
if [ -n "$RECOMMENDED_ENDPOINT" ]; then
    log_test "添加 ONVIF 设备"
    add_device_json=$(cat <<EOF
{
    "xaddr": "$RECOMMENDED_ENDPOINT",
    "username": "$USERNAME",
    "password": "$PASSWORD",
    "name": "测试设备",
    "auto_connect": true
}
EOF
)
    add_response=$(test_http_post "$SERVER_URL/api/onvif/devices" "$add_device_json")
    if echo "$add_response" | grep -q '"id"'; then
        log_pass ""
        DEVICE_ID=$(echo "$add_response" | grep -oP '"id":\s*"\K[^"]+' | head -1)
        log_info "设备ID: $DEVICE_ID"
    else
        log_fail ""
        log_warn "响应: $add_response"
    fi
fi

fi

# 第7部分: ONVIF 设备操作测试
if [ "$SERVER_AVAILABLE" = "true" ] && [ -n "$DEVICE_ID" ]; then
    log_section "ONVIF 设备操作"
    
    log_test "获取设备详情"
    device_info=$(test_http_get "$SERVER_URL/api/onvif/devices/$DEVICE_ID")
    if echo "$device_info" | grep -q '"id"'; then
        log_pass ""
        manufacturer=$(echo "$device_info" | grep -oP '"manufacturer":\s*"\K[^"]+' || echo "N/A")
        model=$(echo "$device_info" | grep -oP '"model":\s*"\K[^"]+' || echo "N/A")
        log_info "制造商: $manufacturer"
        log_info "型号: $model"
    else
        log_fail ""
    fi
    
    log_test "获取媒体配置"
    profiles_response=$(test_http_get "$SERVER_URL/api/onvif/devices/$DEVICE_ID/profiles")
    if echo "$profiles_response" | grep -q '\['; then
        log_pass ""
        profile_count=$(echo "$profiles_response" | grep -o '"token"' | wc -l || echo "0")
        log_info "配置数量: $profile_count"
        
        PROFILE_TOKEN=$(echo "$profiles_response" | grep -oP '"token":\s*"\K[^"]+' | head -1)
        if [ -n "$PROFILE_TOKEN" ]; then
            log_info "配置Token: $PROFILE_TOKEN"
        fi
    else
        log_fail ""
    fi
    
    if [ -n "$PROFILE_TOKEN" ]; then
        log_test "获取流地址"
        stream_uri_response=$(test_http_get "$SERVER_URL/api/onvif/devices/$DEVICE_ID/stream_uri?profile=$PROFILE_TOKEN")
        if echo "$stream_uri_response" | grep -q '"uri"'; then
            log_pass ""
            stream_uri=$(echo "$stream_uri_response" | grep -oP '"uri":\s*"\K[^"]+' || echo "")
            if [ -n "$stream_uri" ]; then
                log_info "流地址: $stream_uri"
            fi
        else
            log_fail ""
        fi
    fi
    
    log_test "获取设备快照"
    snapshot_response=$(test_http "$SERVER_URL/api/onvif/devices/$DEVICE_ID/snapshot")
    if [[ $snapshot_response -eq 200 ]]; then
        log_pass ""
    else
        log_fail "HTTP $snapshot_response"
    fi
    
    log_test "获取 PTZ 能力"
    capabilities_response=$(test_http_get "$SERVER_URL/api/onvif/devices/$DEVICE_ID/capabilities")
    if echo "$capabilities_response" | grep -q '"ptz"'; then
        log_pass ""
        has_ptz=$(echo "$capabilities_response" | grep -oP '"ptz":\s*\K(true|false)' || echo "false")
        log_info "PTZ 支持: $has_ptz"
    else
        log_fail ""
    fi
fi

# 第8部分: 流媒体服务测试
if [ "$SERVER_AVAILABLE" = "true" ]; then
    log_section "流媒体服务测试"

        log_test "获取 ZLMediaKit 信息"
    zlm_info=$(test_http_get "$SERVER_URL/api/zlm/info")
    if echo "$zlm_info" | grep -q '"status"'; then
        log_pass ""
    else
        log_fail ""
    fi
fi

# 第9部分: 清理
if [ "$SERVER_AVAILABLE" = "true" ] && [ -n "$DEVICE_ID" ]; then
    log_section "清理测试设备"
    
    log_test "删除测试设备"
    delete_response=$(curl -s -X DELETE "$SERVER_URL/api/onvif/devices/$DEVICE_ID" 2>/dev/null || echo "")
    if echo "$delete_response" | grep -q 'success'; then
        log_pass ""
    else
        log_fail ""
    fi
fi

# 测试总结
log_header "测试总结"

echo "测试统计:"
echo "  总计: $TOTAL_TESTS"
echo -e "  通过: ${GREEN}$PASSED_TESTS${NC}"
echo -e "  失败: ${RED}$FAILED_TESTS${NC}"
echo ""

echo "关键发现:"
if [ -n "$RECOMMENDED_ENDPOINT" ]; then
    echo -e "  ${GREEN}✓${NC} 推荐 ONVIF 端点: $RECOMMENDED_ENDPOINT"
else
    echo -e "  ${RED}✗${NC} 未找到可用的 ONVIF 端点"
fi

if [ "$SERVER_AVAILABLE" != "true" ]; then
    echo -e "  ${YELLOW}⚠${NC} API 服务器未运行 ($SERVER_URL)"
    echo -e "      提示: 启动服务器后可测试完整的 API 功能"
fi

if [ $FAILED_TESTS -eq 0 ]; then
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}✓ 所有测试通过!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}✗ 部分测试失败${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
fi
