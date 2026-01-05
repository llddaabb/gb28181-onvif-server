#!/bin/bash

# GB28181/ONVIF 服务器启动脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查配置文件
check_config() {
    if [ ! -f "configs/config.yaml" ]; then
        print_error "配置文件不存在: configs/config.yaml"
        exit 1
    fi
    print_info "配置文件检查通过"
}

# 检查端口占用
check_ports() {
    local ports=(5060 9080 10080 554 1935)
    local occupied=false
    
    for port in "${ports[@]}"; do
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
            print_warn "端口 $port 已被占用"
            occupied=true
        fi
    done
    
    if [ "$occupied" = true ]; then
        read -p "是否继续启动？(y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
}

# 创建必要的目录
create_dirs() {
    mkdir -p logs recordings
    print_info "目录结构创建完成"
}

# 查找可执行文件
find_executable() {
    if [ -f "./gb28181-server" ]; then
        echo "./gb28181-server"
    elif [ -f "./dist/gb28181-server" ]; then
        echo "./dist/gb28181-server"
    elif [ -f "./server" ]; then
        echo "./server"
    else
        print_error "未找到可执行文件"
        exit 1
    fi
}

# 配置库路径
setup_lib_path() {
    local lib_dir="./lib"
    if [ -d "$lib_dir" ]; then
        print_info "检测到本地库目录: $lib_dir"
        export LD_LIBRARY_PATH="$lib_dir:$LD_LIBRARY_PATH"
        print_info "已设置 LD_LIBRARY_PATH"
    fi
}

# 显示启动信息
show_info() {
    local exe=$1
    print_info "=========================================="
    print_info "GB28181/ONVIF 视频监控服务器"
    print_info "=========================================="
    print_info "可执行文件: $exe"
    print_info "配置文件: configs/config.yaml"
    print_info ""
    print_info "服务地址："
    print_info "  Web 管理界面: http://localhost:9080"
    print_info "  API 服务: http://localhost:9080/api"
    print_info "  ZLM API: http://localhost:10080"
    print_info ""
    print_info "协议端口："
    print_info "  GB28181 SIP: 5060/UDP"
    print_info "  RTSP: 554/TCP"
    print_info "  RTMP: 1935/TCP"
    print_info "=========================================="
    echo ""
}

# 主函数
main() {
    print_info "正在启动 GB28181/ONVIF 服务器..."
    
    # 检查配置
    check_config
    
    # 检查端口
    check_ports
    
    # 创建目录
    create_dirs
    
    # 配置库路径
    setup_lib_path
    
    # 查找可执行文件
    EXECUTABLE=$(find_executable)
    
    # 显示启动信息
    show_info "$EXECUTABLE"
    
    # 启动服务器
    print_info "启动服务器..."
    exec "$EXECUTABLE"
}

main "$@"
