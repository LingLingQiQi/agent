#!/bin/bash

# AI 智能体项目停止脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# 停止服务
stop_services() {
    log_info "正在停止所有服务..."
    
    local stopped=false
    
    # 停止后端服务 (端口 8443)
    if check_port 8443; then
        log_info "停止后端服务 (端口 8443)..."
        lsof -ti:8443 | xargs kill -9 2>/dev/null || true
        stopped=true
    fi
    
    # 停止前端服务 (端口 8080)
    if check_port 8080; then
        log_info "停止前端服务 (端口 8080)..."
        lsof -ti:8080 | xargs kill -9 2>/dev/null || true
        stopped=true
    fi
    
    if $stopped; then
        # 等待端口释放
        sleep 2
        log_success "所有服务已停止"
    else
        log_warning "没有发现运行中的服务"
    fi
}

# 主函数
main() {
    echo "========================================="
    echo "🛑 AI 智能体项目停止脚本"
    echo "========================================="
    echo ""
    
    stop_services
    
    echo ""
    log_success "项目已完全停止"
}

# 运行主函数
main