#!/bin/bash
# ZLMediaKit 源码编译脚本
# 此脚本从 GitHub 下载 ZLMediaKit 源码并编译

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
ZLM_SOURCE_DIR="$BUILD_DIR/ZLMediaKit"
ZLM_OUTPUT_DIR="$PROJECT_ROOT/internal/zlm/embedded"
ZLM_BRANCH="${ZLM_BRANCH:-master}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_info "检查编译依赖..."
    
    local missing=()
    
    # 必需工具
    command -v git >/dev/null 2>&1 || missing+=("git")
    command -v cmake >/dev/null 2>&1 || missing+=("cmake")
    command -v make >/dev/null 2>&1 || missing+=("make (build-essential)")
    command -v gcc >/dev/null 2>&1 || missing+=("gcc")
    command -v g++ >/dev/null 2>&1 || missing+=("g++")
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "缺少以下依赖: ${missing[*]}"
        log_info "请安装依赖:"
        echo "  Ubuntu/Debian: sudo apt-get install -y git cmake build-essential"
        echo "  CentOS/RHEL:   sudo yum install -y git cmake gcc gcc-c++ make"
        exit 1
    fi
    
    log_info "✓ 基本依赖检查通过"
}

# 安装编译依赖 (可选)
install_dependencies() {
    log_info "安装 ZLMediaKit 编译依赖..."
    
    if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update
        sudo apt-get install -y \
            libssl-dev \
            libsdl2-dev \
            libavcodec-dev \
            libavformat-dev \
            libavutil-dev \
            libswscale-dev \
            libswresample-dev \
            libx264-dev \
            libfaac-dev \
            libmp3lame-dev \
            libsrtp2-dev \
            libusrsctp-dev || true
    elif command -v yum >/dev/null 2>&1; then
        sudo yum install -y \
            openssl-devel \
            SDL2-devel \
            ffmpeg-devel || true
    fi
}

# 下载 ZLMediaKit 源码
download_source() {
    log_info "下载 ZLMediaKit 源码..."
    
    mkdir -p "$BUILD_DIR"
    
    if [ -d "$ZLM_SOURCE_DIR" ]; then
        log_info "源码目录已存在，更新代码..."
        cd "$ZLM_SOURCE_DIR"
        git fetch origin
        git checkout "$ZLM_BRANCH"
        git pull origin "$ZLM_BRANCH"
        git submodule update --init --recursive
    else
        cd "$BUILD_DIR"
        git clone --depth 1 -b "$ZLM_BRANCH" https://github.com/ZLMediaKit/ZLMediaKit.git
        cd "$ZLM_SOURCE_DIR"
        git submodule update --init --recursive
    fi
    
    log_info "✓ 源码下载完成"
}

# 编译 ZLMediaKit
build_zlm() {
    log_info "编译 ZLMediaKit..."
    
    cd "$ZLM_SOURCE_DIR"
    mkdir -p build
    cd build
    
    # CMake 配置
    cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DENABLE_WEBRTC=ON \
        -DENABLE_SRT=ON \
        -DENABLE_RTPPROXY=ON \
        -DENABLE_API=ON \
        -DENABLE_TESTS=OFF \
        -DENABLE_SERVER=ON
    
    # 并行编译
    local nproc=$(nproc 2>/dev/null || echo 4)
    make -j"$nproc"
    
    log_info "✓ ZLMediaKit 编译完成"
}

# 安装到项目目录
install_to_project() {
    log_info "安装 ZLMediaKit 到项目..."
    
    local src_release="$ZLM_SOURCE_DIR/release/linux/Release"
    
    if [ ! -d "$src_release" ]; then
        log_error "编译输出目录不存在: $src_release"
        exit 1
    fi
    
    # 创建输出目录
    mkdir -p "$ZLM_OUTPUT_DIR"
    
    # 复制可执行文件
    cp "$src_release/MediaServer" "$ZLM_OUTPUT_DIR/"
    chmod +x "$ZLM_OUTPUT_DIR/MediaServer"
    
    # 复制配置文件模板
    if [ -f "$src_release/config.ini" ]; then
        cp "$src_release/config.ini" "$ZLM_OUTPUT_DIR/config.ini.template"
    elif [ -f "$ZLM_SOURCE_DIR/conf/config.ini" ]; then
        cp "$ZLM_SOURCE_DIR/conf/config.ini" "$ZLM_OUTPUT_DIR/config.ini.template"
    fi
    
    # 复制 www 目录 (Web 控制台)
    if [ -d "$src_release/www" ]; then
        cp -r "$src_release/www" "$ZLM_OUTPUT_DIR/"
    fi
    
    # 复制依赖库 (如果有)
    if [ -d "$src_release/lib" ]; then
        cp -r "$src_release/lib" "$ZLM_OUTPUT_DIR/"
    fi
    
    # 记录版本信息
    cd "$ZLM_SOURCE_DIR"
    git log -1 --format="%H %s" > "$ZLM_OUTPUT_DIR/VERSION"
    echo "Build Date: $(date)" >> "$ZLM_OUTPUT_DIR/VERSION"
    
    log_info "✓ ZLMediaKit 安装到: $ZLM_OUTPUT_DIR"
}

# 生成嵌入文件
generate_embed_files() {
    log_info "生成 Go embed 文件..."
    
    # 创建 embed.go
    cat > "$ZLM_OUTPUT_DIR/embed.go" << 'EOF'
//go:build linux
// +build linux

package embedded

import (
	"embed"
)

// MediaServerBinary 嵌入的 MediaServer 可执行文件
//go:embed MediaServer
var MediaServerBinary []byte

// ConfigTemplate 嵌入的配置文件模板
//go:embed config.ini.template
var ConfigTemplate []byte

// WWWFiles 嵌入的 Web 控制台文件
//go:embed www
var WWWFiles embed.FS

// Version 版本信息
//go:embed VERSION
var Version string
EOF

    log_info "✓ Go embed 文件生成完成"
}

# 清理编译目录
clean() {
    log_info "清理编译目录..."
    rm -rf "$BUILD_DIR"
    log_info "✓ 清理完成"
}

# 显示帮助
show_help() {
    echo "ZLMediaKit 编译脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  all          完整编译流程 (默认)"
    echo "  deps         安装编译依赖"
    echo "  download     只下载源码"
    echo "  build        只编译"
    echo "  install      只安装到项目"
    echo "  clean        清理编译目录"
    echo "  help         显示帮助"
    echo ""
    echo "环境变量:"
    echo "  ZLM_BRANCH   指定 ZLMediaKit 分支 (默认: master)"
}

# 主函数
main() {
    local cmd="${1:-all}"
    
    case "$cmd" in
        all)
            check_dependencies
            download_source
            build_zlm
            install_to_project
            generate_embed_files
            log_info "🎉 ZLMediaKit 编译安装完成!"
            ;;
        deps)
            install_dependencies
            ;;
        download)
            check_dependencies
            download_source
            ;;
        build)
            build_zlm
            ;;
        install)
            install_to_project
            generate_embed_files
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $cmd"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
