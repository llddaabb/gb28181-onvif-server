#!/bin/bash
# 智能服务器启动脚本 - 自动处理端口冲突

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置要检查的端口列表（根据你的配置文件）
PORTS=(
    9080    # API 端口
    5060    # GB28181 SIP 端口
    554     # RTSP 端口 (ZLMediaKit)
    1935    # RTMP 端口 (ZLMediaKit)
    8080    # HTTP-FLV 端口 (ZLMediaKit)
    80      # HTTP 端口 (ZLMediaKit)
    443     # HTTPS 端口 (ZLMediaKit)
)

# 不应该被关闭的进程名称（保护系统进程）
PROTECTED_PROCESSES=(
    "systemd"
    "init"
    "sshd"
    "bash"
    "zsh"
    "sudo"
    "dbus"
    "NetworkManager"
)

# 日志函数
log_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    printf "${CYAN}║ %-58s ║${NC}\n" "$1"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_action() {
    echo -e "${BLUE}[ACTION]${NC} $1"
}

# 检查是否为受保护的进程
is_protected_process() {
    local process_name=$1
    for protected in "${PROTECTED_PROCESSES[@]}"; do
        if [[ "$process_name" == *"$protected"* ]]; then
            return 0
        fi
    done
    return 1
}

# 检查并关闭占用端口的进程
check_and_kill_port() {
    local port=$1
    
    # 查找占用端口的进程
    local pids=$(lsof -ti:$port 2>/dev/null || true)
    
    if [ -z "$pids" ]; then
        log_info "端口 $port: ✓ 空闲"
        return 0
    fi
    
    # 遍历每个PID
    for pid in $pids; do
        # 获取进程信息
        local process_name=$(ps -p $pid -o comm= 2>/dev/null || echo "unknown")
        local process_cmd=$(ps -p $pid -o cmd= 2>/dev/null || echo "unknown")
        
        # 检查是否为受保护的进程
        if is_protected_process "$process_name"; then
            log_warn "端口 $port: 被保护进程占用 [$process_name (PID: $pid)]"
            log_warn "  命令: $process_cmd"
            log_error "  ✗ 拒绝关闭受保护的进程，请手动处理"
            return 1
        fi
        
        # 检查是否为本项目的旧实例
        if [[ "$process_cmd" == *"./server"* ]] || [[ "$process_cmd" == *"zpip"* ]]; then
            log_warn "端口 $port: 被旧的服务器实例占用 (PID: $pid)"
            log_action "  正在关闭旧实例..."
            
            # 尝试优雅关闭
            kill -TERM $pid 2>/dev/null || true
            sleep 2
            
            # 检查是否还存在
            if kill -0 $pid 2>/dev/null; then
                log_warn "  进程未响应，强制终止..."
                kill -KILL $pid 2>/dev/null || true
            fi
            
            log_info "  ✓ 旧实例已关闭"
        else
            log_warn "端口 $port: 被其他程序占用 [$process_name (PID: $pid)]"
            log_warn "  命令: $process_cmd"
            
            # 询问用户是否关闭
            if [ "${AUTO_KILL:-false}" = "true" ]; then
                log_action "  自动模式: 关闭进程..."
                kill -TERM $pid 2>/dev/null || true
                sleep 1
                
                if kill -0 $pid 2>/dev/null; then
                    kill -KILL $pid 2>/dev/null || true
                fi
                log_info "  ✓ 进程已关闭"
            else
                read -p "  是否关闭此进程? [y/N] " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    log_action "  正在关闭进程..."
                    kill -TERM $pid 2>/dev/null || true
                    sleep 1
                    
                    if kill -0 $pid 2>/dev/null; then
                        kill -KILL $pid 2>/dev/null || true
                    fi
                    log_info "  ✓ 进程已关闭"
                else
                    log_error "  用户取消，端口仍被占用"
                    return 1
                fi
            fi
        fi
    done
    
    return 0
}

# 主程序
main() {
    log_header "智能服务器启动 - 端口冲突检测与处理"
    
    # 检查是否有 lsof 命令
    if ! command -v lsof &> /dev/null; then
        log_error "未找到 lsof 命令，无法检测端口占用"
        log_info "请安装 lsof: sudo apt-get install lsof"
        exit 1
    fi
    
    # 检查服务器可执行文件
    if [ ! -f "./server" ]; then
        log_error "未找到服务器可执行文件: ./server"
        log_info "请先编译: make build 或 go build -o server ./cmd/server"
        exit 1
    fi
    
    log_info "开始检查端口占用情况..."
    echo ""
    
    local has_conflict=false
    
    # 检查所有端口
    for port in "${PORTS[@]}"; do
        if ! check_and_kill_port $port; then
            has_conflict=true
        fi
    done
    
    echo ""
    
    if [ "$has_conflict" = true ]; then
        log_error "存在端口冲突，无法启动服务器"
        log_info "请手动处理上述端口占用问题，或使用 AUTO_KILL=true 自动关闭所有占用进程"
        exit 1
    fi
    
    log_info "✓ 所有端口检查完成，没有冲突"
    echo ""
    
    # 启动服务器
    log_header "启动服务器"
    log_action "执行: ./server $@"
    echo ""
    
    # 运行服务器，传递所有参数
    exec ./server "$@"
}

# 显示帮助信息
show_help() {
    cat << EOF
使用方法: $0 [选项]

智能启动脚本 - 自动检测并处理端口冲突

选项:
  -h, --help          显示此帮助信息
  -a, --auto          自动模式，不询问直接关闭占用端口的进程
  -l, --list          仅列出端口占用情况，不启动服务器

环境变量:
  AUTO_KILL=true      启用自动关闭模式

示例:
  $0                  # 交互式启动（询问是否关闭占用进程）
  $0 -a               # 自动启动（直接关闭占用进程）
  $0 -l               # 仅检查端口占用
  AUTO_KILL=true $0   # 使用环境变量自动启动

EOF
}

# 仅列出端口占用
list_only() {
    log_header "端口占用情况"
    
    for port in "${PORTS[@]}"; do
        local pids=$(lsof -ti:$port 2>/dev/null || true)
        
        if [ -z "$pids" ]; then
            echo -e "端口 ${GREEN}$port${NC}: 空闲"
        else
            echo -e "端口 ${RED}$port${NC}: 占用"
            for pid in $pids; do
                local process_name=$(ps -p $pid -o comm= 2>/dev/null || echo "unknown")
                local process_cmd=$(ps -p $pid -o cmd= 2>/dev/null || echo "unknown")
                echo "  ├─ PID: $pid"
                echo "  ├─ 进程: $process_name"
                echo "  └─ 命令: $process_cmd"
            done
        fi
    done
    
    exit 0
}

# 解析命令行参数
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -l|--list)
        list_only
        ;;
    -a|--auto)
        export AUTO_KILL=true
        shift
        main "$@"
        ;;
    *)
        main "$@"
        ;;
esac
