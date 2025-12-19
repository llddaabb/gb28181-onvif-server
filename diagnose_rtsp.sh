#!/bin/bash

# ONVIF RTSP 诊断脚本
# 用于诊断为什么 RTSP URL 无法访问

echo "========================================"
echo "ONVIF RTSP URL 诊断工具"
echo "========================================"
echo ""

if [ $# -lt 3 ]; then
    echo "使用方法: $0 <设备IP> <RTSP端口> <RTSP路径> [用户名] [密码]"
    echo "例如: $0 192.168.1.232 554 /Streaming/Channels/101 admin a123456789"
    echo ""
    echo "常见的 RTSP 路径："
    echo "  海康: /Streaming/Channels/101"
    echo "  大华: /live/1"
    echo "  宇视: /livestream"
    echo "  通用: /stream"
    echo ""
    exit 1
fi

DEVICE_IP=$1
RTSP_PORT=$2
RTSP_PATH=$3
USERNAME=${4:-admin}
PASSWORD=${5:-a123456789}

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[!]${NC} $1"; }

echo "诊断信息："
echo "  设备IP: $DEVICE_IP"
echo "  RTSP端口: $RTSP_PORT"
echo "  RTSP路径: $RTSP_PATH"
echo "  用户名: $USERNAME"
echo ""

# 测试 1: 基本网络连接
log_info "测试 1: 基本网络连接..."
if ping -c 1 -W 2 "$DEVICE_IP" &> /dev/null; then
    log_success "设备可以 ping 通"
else
    log_error "设备无法 ping 通，可能的原因："
    echo "  1. 设备不在线"
    echo "  2. 防火墙阻止了 ICMP"
    echo "  3. 网络连接问题"
fi
echo ""

# 测试 2: RTSP 端口连接
log_info "测试 2: RTSP 端口连接 (TCP $RTSP_PORT)..."
timeout 3 bash -c "echo > /dev/tcp/$DEVICE_IP/$RTSP_PORT" 2>/dev/null
if [ $? -eq 0 ]; then
    log_success "RTSP 端口 $RTSP_PORT 可以连接"
else
    log_error "RTSP 端口 $RTSP_PORT 无法连接，可能的原因："
    echo "  1. RTSP 服务未启动"
    echo "  2. 防火墙阻止了此端口"
    echo "  3. 设备使用了不同的 RTSP 端口"
fi
echo ""

# 测试 3: RTSP DESCRIBE 请求
log_info "测试 3: RTSP DESCRIBE 请求（不带认证）..."
RTSP_URL="rtsp://$DEVICE_IP:$RTSP_PORT$RTSP_PATH"
RESPONSE=$(curl -s -i -X DESCRIBE "$RTSP_URL" 2>&1 | head -10)

if echo "$RESPONSE" | grep -q "200\|OK"; then
    log_success "RTSP 流可访问（无认证）"
    echo "$RESPONSE" | head -5
elif echo "$RESPONSE" | grep -q "401\|Unauthorized"; then
    log_warning "需要 RTSP 认证"
    log_info "测试 4: RTSP DESCRIBE 请求（带认证）..."
    RESPONSE=$(curl -s -i -X DESCRIBE --user "$USERNAME:$PASSWORD" "$RTSP_URL" 2>&1 | head -10)
    if echo "$RESPONSE" | grep -q "200\|OK"; then
        log_success "RTSP 流通过认证可访问"
        echo "$RESPONSE" | head -5
    else
        log_error "认证失败或 RTSP 流不存在"
        echo "  响应: $(echo "$RESPONSE" | head -1)"
    fi
elif echo "$RESPONSE" | grep -q "404\|Not Found"; then
    log_error "RTSP 路径不存在 (404)"
    echo "  请尝试以下常见路径之一："
    echo "    /Streaming/Channels/101 (海康)"
    echo "    /live/1 (大华)"
    echo "    /livestream (宇视)"
    echo "    /stream (通用)"
elif echo "$RESPONSE" | grep -q "Connection refused\|timed out"; then
    log_error "连接被拒绝或超时"
    echo "  请检查 RTSP 端口是否正确，或设备是否在线"
else
    log_error "RTSP 请求失败"
    echo "  响应: $RESPONSE"
fi
echo ""

# 测试 4: 测试不同的 RTSP 端口
if [ "$RTSP_PORT" != "554" ]; then
    log_info "测试 5: 尝试标准 RTSP 端口 554..."
    timeout 3 bash -c "echo > /dev/tcp/$DEVICE_IP/554" 2>/dev/null
    if [ $? -eq 0 ]; then
        log_warning "发现 554 端口可连接，可能需要改用此端口"
    fi
    echo ""
fi

# 测试 5: 使用 ffprobe 探测视频信息
log_info "测试 6: 使用 ffprobe 探测视频编码..."
if command -v ffprobe &> /dev/null; then
    FFPROBE_RESULT=$(ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate \
        -of default=noprint_wrappers=1 "$RTSP_URL" 2>&1)
    if [ -n "$FFPROBE_RESULT" ]; then
        log_success "视频信息："
        echo "$FFPROBE_RESULT" | sed 's/^/    /'
    else
        log_warning "ffprobe 无法获取视频信息（可能需要正确的凭据）"
    fi
else
    log_warning "ffprobe 未安装，跳过视频信息探测"
    echo "  安装方法: sudo apt-get install ffmpeg"
fi
echo ""

# 总结
echo "========================================"
echo "诊断完成"
echo "========================================"
echo ""
echo "如果遇到 404 错误，请尝试以下常见路径："
echo ""
echo "设备厂商       常见 RTSP 路径"
echo "海康威视       /Streaming/Channels/101"
echo "大华           /live/1"
echo "宇视           /livestream"
echo "Axis           /axis-media/media.amp"
echo "Sony           /media/video1"
echo "通用           /stream"
echo ""
echo "如果需要更多帮助，请："
echo "  1. 查看设备网页界面中的 RTSP 配置"
echo "  2. 查看设备说明书或官方文档"
echo "  3. 尝试使用 VLC 或其他 RTSP 播放器手动连接"
echo ""
