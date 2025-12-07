#!/bin/bash

# GB28181/ONVIF 信令服务器启动脚本
# 用法: ./start.sh [start|stop|restart|status]

APP_NAME="gb28181-server"
APP_PATH="./server"
CONFIG_PATH="./configs/config.yaml"
PID_FILE="./server.pid"
LOG_FILE="./logs/server.log"

# AI检测服务配置
AI_DETECTOR_SCRIPT="./start_ai_detector.sh"
AI_DETECTOR_PID_FILE="./ai_detector.pid"
AI_DETECTOR_PORT="${AI_DETECTOR_PORT:-8001}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 确保日志目录存在
mkdir -p ./logs

# AI检测服务管理函数
start_ai_detector() {
    # 检查AI是否启用
    if ! grep -q "Enable: true" "$CONFIG_PATH" 2>/dev/null; then
        return 0
    fi
    
    if [ ! -f "$AI_DETECTOR_SCRIPT" ]; then
        echo -e "${YELLOW}[AI]${NC} AI检测脚本不存在，跳过"
        return 0
    fi
    
    echo -e "${GREEN}[AI]${NC} 启动AI检测服务..."
    export AI_DETECTOR_PORT="$AI_DETECTOR_PORT"
    "$AI_DETECTOR_SCRIPT" start
    local result=$?
    
    if [ $result -eq 0 ]; then
        echo -e "${GREEN}[AI]${NC} AI检测服务启动成功 (端口: $AI_DETECTOR_PORT)"
    else
        echo -e "${YELLOW}[AI]${NC} AI检测服务启动失败（可选服务，不影响主服务）"
        echo -e "${YELLOW}[AI]${NC} 提示: 请确保已下载模型文件到 third-party/zlm/models/yolov8s.onnx"
    fi
}

stop_ai_detector() {
    if [ ! -f "$AI_DETECTOR_SCRIPT" ]; then
        return 0
    fi
    
    echo -e "${YELLOW}[AI]${NC} 停止AI检测服务..."
    "$AI_DETECTOR_SCRIPT" stop > /dev/null 2>&1
}

check_ai_detector_status() {
    if [ ! -f "$AI_DETECTOR_PID_FILE" ]; then
        return 1
    fi
    
    local ai_pid=$(cat "$AI_DETECTOR_PID_FILE" 2>/dev/null)
    if [ -n "$ai_pid" ] && ps -p "$ai_pid" > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# 获取进程PID
get_pid() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null 2>&1; then
            echo "$pid"
            return 0
        fi
    fi
    # 尝试通过进程名查找
    pgrep -f "$APP_NAME" 2>/dev/null | head -1
}

# 启动服务
start() {
    local pid=$(get_pid)
    if [ -n "$pid" ]; then
        echo -e "${YELLOW}[警告]${NC} 服务已在运行 (PID: $pid)"
        return 1
    fi

    # 检查可执行文件
    if [ ! -f "$APP_PATH" ]; then
        echo -e "${RED}[错误]${NC} 可执行文件不存在: $APP_PATH"
        echo -e "${YELLOW}[提示]${NC} 请先运行: make build"
        return 1
    fi

    # 检查配置文件
    if [ ! -f "$CONFIG_PATH" ]; then
        echo -e "${RED}[错误]${NC} 配置文件不存在: $CONFIG_PATH"
        return 1
    fi

    # 清理可能残留的 ZLM 进程
    pkill -f "MediaServer" 2>/dev/null
    sleep 1

    echo -e "${GREEN}[启动]${NC} 正在启动 $APP_NAME ..."
    
    # 后台启动，输出到日志文件
    nohup "$APP_PATH" -config "$CONFIG_PATH" >> "$LOG_FILE" 2>&1 &
    local new_pid=$!
    
    # 等待进程启动
    sleep 2
    
    if ps -p "$new_pid" > /dev/null 2>&1; then
        echo "$new_pid" > "$PID_FILE"
        echo -e "${GREEN}[成功]${NC} 服务启动成功 (PID: $new_pid)"
        echo -e "${GREEN}[信息]${NC} 日志文件: $LOG_FILE"
        echo ""
        echo "服务端口:"
        echo "  - API:     http://0.0.0.0:9080"
        echo "  - ZLM HTTP: http://0.0.0.0:8080"
        echo "  - RTSP:    rtsp://0.0.0.0:8554"
        echo "  - RTMP:    rtmp://0.0.0.0:1935"
        echo "  - SIP:     0.0.0.0:5060"
        echo ""
        
        # 启动AI检测服务
        start_ai_detector
        
        return 0
    else
        echo -e "${RED}[失败]${NC} 服务启动失败，请查看日志: $LOG_FILE"
        return 1
    fi
}

# 停止服务
stop() {
    # 先停止AI检测服务
    stop_ai_detector
    
    local pid=$(get_pid)
    if [ -z "$pid" ]; then
        echo -e "${YELLOW}[警告]${NC} 服务未运行"
        rm -f "$PID_FILE"
        return 0
    fi

    echo -e "${YELLOW}[停止]${NC} 正在停止服务 (PID: $pid) ..."
    
    # 发送 SIGTERM 信号
    kill "$pid" 2>/dev/null
    
    # 等待进程退出
    local count=0
    while ps -p "$pid" > /dev/null 2>&1; do
        sleep 1
        count=$((count + 1))
        if [ $count -ge 10 ]; then
            echo -e "${YELLOW}[警告]${NC} 进程未响应，强制终止..."
            kill -9 "$pid" 2>/dev/null
            # 同时停止 ZLM 子进程
            pkill -9 -f "MediaServer" 2>/dev/null
            break
        fi
    done
    
    rm -f "$PID_FILE"
    echo -e "${GREEN}[成功]${NC} 服务已停止"
    return 0
}

# 重启服务
restart() {
    echo -e "${YELLOW}[重启]${NC} 正在重启服务..."
    stop
    sleep 2
    start
}

# 查看状态
status() {
    local pid=$(get_pid)
    if [ -n "$pid" ]; then
        echo -e "${GREEN}[运行中]${NC} 服务正在运行 (PID: $pid)"
        echo ""
        echo "进程信息:"
        ps -p "$pid" -o pid,ppid,user,%cpu,%mem,etime,cmd --no-headers 2>/dev/null
        echo ""
        # 检查 ZLM 子进程
        local zlm_pid=$(pgrep -f "MediaServer" | head -1)
        if [ -n "$zlm_pid" ]; then
            echo -e "${GREEN}[ZLM]${NC} ZLMediaKit 运行中 (PID: $zlm_pid)"
        else
            echo -e "${YELLOW}[ZLM]${NC} ZLMediaKit 未运行"
        fi
        
        # 检查 AI 检测服务
        if check_ai_detector_status; then
            local ai_pid=$(cat "$AI_DETECTOR_PID_FILE" 2>/dev/null)
            echo -e "${GREEN}[AI]${NC} AI检测服务运行中 (PID: $ai_pid, 端口: $AI_DETECTOR_PORT)"
        else
            echo -e "${YELLOW}[AI]${NC} AI检测服务未运行"
        fi
        
        return 0
    else
        echo -e "${RED}[已停止]${NC} 服务未运行"
        return 1
    fi
}

# 查看日志
logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo -e "${YELLOW}[警告]${NC} 日志文件不存在: $LOG_FILE"
    fi
}

# 显示帮助
usage() {
    echo "用法: $0 {start|stop|restart|status|logs}"
    echo ""
    echo "命令:"
    echo "  start   - 启动服务"
    echo "  stop    - 停止服务"
    echo "  restart - 重启服务"
    echo "  status  - 查看服务状态"
    echo "  logs    - 查看实时日志"
    echo ""
    echo "示例:"
    echo "  $0 start    # 启动服务"
    echo "  $0 stop     # 停止服务"
    echo "  $0 logs     # 查看日志"
}

# 主逻辑
case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    *)
        usage
        exit 1
        ;;
esac

exit $?
