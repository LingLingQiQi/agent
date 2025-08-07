#!/bin/bash

# AI 智能体项目启动脚本
# 自动启动前端和后端服务

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查端口是否被占用
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0  # 端口被占用
    else
        return 1  # 端口未被占用
    fi
}

# 获取本机IP地址
get_local_ip() {
    # 获取第一个非回环地址的IP
    local ip=$(ifconfig | grep "inet " | grep -v "127.0.0.1" | head -1 | awk '{print $2}')
    echo "$ip"
}

# 停止之前的服务
cleanup() {
    log_info "正在停止服务..."
    
    # 停止后端服务 (端口 8443)
    if check_port 8443; then
        log_info "停止后端服务 (端口 8443)..."
        lsof -ti:8443 | xargs kill -9 2>/dev/null || true
    fi
    
    # 停止前端服务 (端口 8080)
    if check_port 8080; then
        log_info "停止前端服务 (端口 8080)..."
        lsof -ti:8080 | xargs kill -9 2>/dev/null || true
    fi
    
    # 等待端口释放
    sleep 2
}

# 启动后端服务
start_backend() {
    log_info "启动后端服务..."
    
    cd backend
    
    # 检查 Go 环境
    if ! command -v go &> /dev/null; then
        log_error "Go 环境未安装，请先安装 Go 1.23 或更高版本"
        exit 1
    fi
    
    # 检查配置文件中的API密钥
    if [ -f "configs/config.yaml" ]; then
        api_key=$(grep "api_key:" configs/config.yaml | cut -d'"' -f2)
        if [ "$api_key" = "YOUR_API_KEY_HERE" ] || [ -z "$api_key" ]; then
            log_warning "请在 configs/config.yaml 中设置正确的 API 密钥"
            log_warning "将 'YOUR_API_KEY_HERE' 替换为您的豆包API密钥"
        fi
    else
        log_warning "未找到配置文件 configs/config.yaml"
    fi
    
    # 整理依赖
    log_info "整理 Go 依赖..."
    go mod tidy
    
    # 启动后端服务
    log_info "启动后端服务器 (端口 8443)..."
    nohup go run cmd/main.go > ../logs/backend.log 2>&1 &
    BACKEND_PID=$!
    
    cd ..
    
    # 等待后端启动
    sleep 5
    
    # 检查后端是否启动成功
    if check_port 8443; then
        log_success "后端服务启动成功 (PID: $BACKEND_PID)"
    else
        log_error "后端服务启动失败，请检查日志: logs/backend.log"
        exit 1
    fi
}

start_frontend() {
    log_info "启动前端服务..."
    
    cd frontend
    
    # 检查 Node.js 环境
    if ! command -v npm &> /dev/null; then
        log_error "Node.js/npm 环境未安装，请先安装 Node.js"
        exit 1
    fi
    
    # 安装依赖（如果需要）
    if [ ! -d "node_modules" ]; then
        log_info "安装前端依赖..."
        npm install
    fi
    
    # 启动前端开发服务器
    log_info "启动前端开发服务器 (端口 8080)..."
    nohup npm run dev > ../logs/frontend.log 2>&1 &
    FRONTEND_PID=$!
    
    cd ..
    
    # 等待前端启动
    sleep 5
    
    # 检查前端是否启动成功
    if check_port 8080; then
        log_success "前端服务启动成功 (PID: $FRONTEND_PID)"
    else
        log_error "前端服务启动失败，请检查日志: logs/frontend.log"
        exit 1
    fi
}

# 显示访问信息
show_access_info() {
    local ip=$(get_local_ip)
    
    echo ""
    echo "=================================="
    log_success "服务启动完成！"
    echo "=================================="
    echo ""
    echo "🌐 访问地址："
    echo "   本地访问:   http://localhost:8080"
    echo "   局域网访问: http://$ip:8080"
    echo ""
    echo "🔧 服务信息："
    echo "   前端服务:   http://localhost:8080 (PID: $FRONTEND_PID)"
    echo "   后端服务:   http://localhost:8443 (PID: $BACKEND_PID)"
    echo ""
    echo "📋 管理命令："
    echo "   查看后端日志: tail -f logs/backend.log"
    echo "   查看前端日志: tail -f logs/frontend.log"
    echo "   停止所有服务: ./stop.sh 或 Ctrl+C"
    echo ""
    echo "💡 提示："
    echo "   - 请在 backend/configs/config.yaml 中设置正确的 API 密钥"
    echo "   - 局域网内其他设备可通过 $ip:8080 访问"
    echo "   - 按 Ctrl+C 可停止所有服务"
    echo ""
}

# 等待用户输入停止
wait_for_stop() {
    echo "按 Ctrl+C 停止所有服务..."
    
    # 设置信号处理
    trap 'cleanup; exit 0' INT TERM
    
    # 等待信号
    while true; do
        sleep 1
    done
}

# 主函数
main() {
    echo "========================================="
    echo "🚀 AI 智能体项目启动脚本"
    echo "========================================="
    echo ""
    
    # 创建日志目录
    mkdir -p logs
    
    # 停止之前的服务
    cleanup
    
    # 启动服务
    start_backend
    start_frontend
    
    # 显示访问信息
    show_access_info
    
    # 等待停止
    wait_for_stop
}

# 检查命令行参数
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "AI 智能体项目启动脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help    显示此帮助信息"
    echo ""
    echo "功能:"
    echo "  - 自动启动后端 Go 服务 (端口 8443)"
    echo "  - 自动启动前端 React 开发服务器 (端口 8080)"
    echo "  - 提供本地和局域网访问地址"
    echo "  - 实时日志输出和服务管理"
    echo ""
    echo "环境要求:"
    echo "  - Go 1.23 或更高版本"
    echo "  - Node.js 和 npm"
    echo "  - 在 backend/configs/config.yaml 中设置正确的 API 密钥"
    echo ""
    exit 0
fi

# 运行主函数
main