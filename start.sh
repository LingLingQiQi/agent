#!/bin/bash

# AI 智能体项目启动脚本 - 生产模式
# 构建前端生产版本，启动后端并提供静态文件服务

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
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

# 强制清理端口
force_kill_port() {
    local port=$1
    local port_name=$2
    
    log_info "检查端口 $port 占用情况..."
    
    # 获取占用端口的进程ID
    local pids=$(lsof -ti :$port 2>/dev/null)
    
    if [ -n "$pids" ]; then
        log_warning "发现端口 $port 被以下进程占用: $pids"
        log_info "正在强制终止占用端口 $port 的进程..."
        
        # 先尝试温和终止
        echo "$pids" | xargs -r kill -TERM 2>/dev/null || true
        sleep 2
        
        # 检查是否还有进程占用端口
        local remaining_pids=$(lsof -ti :$port 2>/dev/null)
        if [ -n "$remaining_pids" ]; then
            log_warning "温和终止失败，强制杀死进程: $remaining_pids"
            echo "$remaining_pids" | xargs -r kill -9 2>/dev/null || true
            sleep 1
        fi
        
        # 最终检查
        if check_port $port; then
            log_error "无法清理端口 $port，请手动检查：lsof -i :$port"
            return 1
        else
            log_success "端口 $port 已成功清理"
            return 0
        fi
    else
        log_info "端口 $port 未被占用"
        return 0
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
    
    # 强制清理后端服务 (端口 8443)
    if ! force_kill_port 8443 "后端服务"; then
        log_error "无法清理后端服务端口 8443"
        exit 1
    fi
    
    # 强制清理前端服务 (端口 8080)
    if ! force_kill_port 8080 "前端开发服务"; then
        log_warning "清理前端开发服务端口 8080 失败，继续执行..."
    fi
    
    # 强制清理静态文件服务 (端口 4173)
    if ! force_kill_port 4173 "静态文件服务"; then
        log_warning "清理静态文件服务端口 4173 失败，继续执行..."
    fi
    
    # 等待端口释放
    sleep 2
    
    log_success "服务清理完成"
}

# 构建前端
build_frontend() {
    log_info "构建前端生产版本..."
    
    cd frontend
    
    # 检查 Node.js 环境
    if ! command -v npm &> /dev/null; then
        log_error "Node.js/npm 环境未安装，请先安装 Node.js"
        exit 1
    fi
    
    # 安装依赖（如果需要）
    if [ ! -d "node_modules" ]; then
        log_info "安装前端依赖..."
        if ! npm install; then
            log_error "前端依赖安装失败，请检查 package.json 文件"
            exit 1
        fi
    fi
    
    # 构建生产版本
    log_info "构建前端资源..."
    if ! npm run build; then
        log_error "前端构建失败"
        echo ""
        echo "================= 构建错误信息 ================="
        echo "请检查前端代码是否有语法错误或依赖问题"
        echo "您可以尝试手动运行："
        echo "cd frontend && npm run build"
        echo "=============================================="
        exit 1
    fi
    
    cd ..
    
    # 检查构建结果
    if [ ! -d "frontend/dist" ] || [ ! -f "frontend/dist/index.html" ]; then
        log_error "前端构建失败，dist目录不存在或不完整"
        exit 1
    fi
    
    # 显示构建文件大小
    local dist_size=$(du -sh frontend/dist 2>/dev/null | cut -f1 || echo "unknown")
    log_success "前端构建完成，构建文件大小: $dist_size"
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
    
    # 确保端口已清理
    if check_port 8443; then
        log_error "端口 8443 仍被占用，清理失败"
        lsof -i :8443
        exit 1
    fi
    
    # 整理依赖
    log_info "整理 Go 依赖..."
    if ! go mod tidy; then
        log_error "Go 依赖整理失败，请检查 go.mod 文件"
        exit 1
    fi
    
    # 启动后端服务
    log_info "启动后端服务器 (端口 8443)..."
    
    # 启动后端并保存PID
    nohup go run cmd/main.go > /tmp/backend.log 2>&1 &
    BACKEND_PID=$!
    
    cd ..
    
    # 等待后端启动
    log_info "等待后端服务启动..."
    local max_wait=15
    local wait_count=0
    
    while [ $wait_count -lt $max_wait ]; do
        if check_port 8443; then
            log_success "后端服务启动成功 (PID: $BACKEND_PID)"
            return 0
        fi
        
        # 检查进程是否还在运行
        if ! ps -p $BACKEND_PID > /dev/null 2>&1; then
            log_error "后端进程已退出，启动失败"
            break
        fi
        
        sleep 1
        wait_count=$((wait_count + 1))
        echo -n "."
    done
    
    echo ""
    log_error "后端服务启动失败，请检查日志输出"
    echo ""
    echo "================= 后端错误日志 ================="
    if [ -f "/tmp/backend.log" ] && [ -s "/tmp/backend.log" ]; then
        cat /tmp/backend.log
    else
        log_error "后端日志文件为空或不存在，可能启动过程中出现严重错误"
        log_info "请手动运行以查看详细错误："
        echo "cd backend && go run cmd/main.go"
    fi
    echo "=============================================="
    echo ""
    
    # 如果进程仍在运行，杀掉它
    if ps -p $BACKEND_PID > /dev/null 2>&1; then
        kill $BACKEND_PID 2>/dev/null || true
    fi
    
    exit 1
}

# 启动静态文件服务
start_static_server() {
    log_info "启动静态文件服务..."
    
    cd frontend
    
    # 使用vite preview提供静态文件服务
    log_info "启动静态文件服务器 (端口 4173)..."
    nohup npm run preview > /tmp/static.log 2>&1 &
    STATIC_PID=$!
    
    cd ..
    
    # 等待静态服务启动
    sleep 3
    
    # 检查静态服务是否启动成功
    if check_port 4173; then
        log_success "静态文件服务启动成功 (PID: $STATIC_PID)"
    else
        log_error "静态文件服务启动失败"
        echo ""
        echo "================= 静态服务错误日志 ================="
        if [ -f "/tmp/static.log" ] && [ -s "/tmp/static.log" ]; then
            cat /tmp/static.log
        else
            log_error "静态服务日志文件为空或不存在"
        fi
        echo "=============================================="
        
        # 如果进程仍在运行，杀掉它
        if ps -p $STATIC_PID > /dev/null 2>&1; then
            kill $STATIC_PID 2>/dev/null || true
        fi
        
        exit 1
    fi
}

# 显示访问信息
show_access_info() {
    local ip=$(get_local_ip)
    
    echo ""
    echo "=================================="
    log_success "生产模式启动完成！"
    echo "=================================="
    echo ""
    echo "🌐 访问地址："
    echo "   前端页面:   http://localhost:4173"
    echo "   局域网访问: http://$ip:4173"
    echo ""
    echo "🔧 服务信息："
    echo "   前端服务:   http://localhost:4173 (静态文件, PID: $STATIC_PID)"
    echo "   后端API:    http://localhost:8443 (API服务, PID: $BACKEND_PID)"
    echo ""
    echo "📋 特性说明："
    echo "   ✅ 前端已构建为生产版本，性能更优"
    echo "   ✅ 减少网络请求，无开发模式的chunk请求"
    echo "   ✅ 资源已压缩优化，加载速度更快"
    echo "   ❌ 无热更新功能，修改代码需重新构建"
    echo ""
    echo "📋 管理命令："
    echo "   停止所有服务: ./stop.sh 或 Ctrl+C"
    echo "   重新构建:     ./start-prod.sh"
    echo "   开发模式:     ./start-with-logs.sh"
    echo ""
    echo "📄 日志文件："
    echo "   后端日志: tail -f /tmp/backend.log"
    echo "   静态服务: tail -f /tmp/static.log"
    echo ""
    echo "💡 提示："
    echo "   - 如需修改前端代码，请使用开发模式: ./start-with-logs.sh"
    echo "   - 生产模式适合性能测试和演示展示"
    echo "   - 局域网内其他设备可通过 $ip:4173 访问"
    echo ""
}

# 主函数
main() {
    echo "========================================="
    echo "🚀 AI 智能体项目启动脚本 (生产模式)"
    echo "========================================="
    echo ""
    
    # 停止之前的服务
    cleanup
    
    # 构建前端
    build_frontend
    
    # 启动后端服务
    start_backend
    
    # 启动静态文件服务
    start_static_server
    
    # 显示访问信息
    show_access_info
    
    # 等待用户中断
    log_info "按 Ctrl+C 停止所有服务"
    trap 'cleanup; exit 0' INT TERM
    
    while true; do
        sleep 1
    done
}

# 检查命令行参数
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "AI 智能体项目启动脚本 (生产模式)"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help    显示此帮助信息"
    echo ""
    echo "功能:"
    echo "  - 构建前端生产版本 (压缩优化)"
    echo "  - 启动后端 Go 服务 (端口 8443)"
    echo "  - 启动静态文件服务器 (端口 4173)"
    echo "  - 提供本地和局域网访问地址"
    echo "  - 生产级性能优化"
    echo ""
    echo "与开发模式区别:"
    echo "  ✅ 更快的页面加载速度"
    echo "  ✅ 更少的网络请求"
    echo "  ✅ 生产级资源优化"
    echo "  ❌ 无代码热更新"
    echo "  ❌ 每次修改需重新构建"
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