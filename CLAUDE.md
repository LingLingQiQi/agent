# CLAUDE.md

这个文件为 Claude Code (claude.ai/code) 在处理此仓库代码时提供指导。

## 项目架构

这是一个多组件 AI 智能体系统，由两个主要部分组成：

1. **后端 (Go)**: 使用 Eino 框架和豆包大模型的 AI 智能体服务，集成工具系统
2. **前端 (JavaScript)**: 具有实时流式传输的 Web 聊天界面  

### 核心组件

**后端 (Go)**:
- 使用服务器发送事件 (SSE) 的实时流式 AI 聊天
- 使用内存存储的多会话聊天历史管理
- 使用 Eino 框架的工具系统集成
- 与豆包 AI 模型集成 (火山引擎)

**前端**:
- 纯 HTML + Tailwind CSS + JavaScript 聊天界面
- 具有 markdown 渲染的实时消息流
- 会话管理和历史记录

## 开发命令

### 后端 (Go)
```bash
cd backend
go mod tidy                    # 清理依赖项  
go run cmd/main.go            # 启动开发服务器 (端口 8443)
```

### 测试后端 API
```bash
curl -X POST http://localhost:8443/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'
```

## 技术栈

### 后端
- **Go 1.23** 配合 Gin 框架
- **Eino 框架** 用于智能体和工具集成
- **内存会话存储** 配合清理机制
- **服务器发送事件** 用于流式响应
- **豆包 AI 模型** 集成 (火山引擎)
- **结构化日志** 使用 logrus
- **Viper** 用于配置管理

### 前端
- **Tailwind CSS** 用于样式
- **Font Awesome** 用于图标  
- **DS-Markdown** 用于 markdown 渲染
- **SSE** 用于实时通信

## API 端点

### 聊天 APIs
- `POST /api/chat/stream` - 流式聊天消息
- `POST /api/chat/session` - 创建新会话
- `GET /api/chat/session/:session_id` - 获取会话信息
- `GET /api/chat/messages/:session_id` - 获取会话消息历史
- `PUT /api/chat/session/:session_id` - 更新会话标题

## 配置

### 后端配置 (`backend/configs/config.yaml`)
- 服务器设置 (端口 8443, 超时)
- 豆包 API 设置 (通过环境变量设置 API 密钥)
- 智能体提示和工具配置
- CORS、日志和速率限制设置

## 关键文件结构

### 后端
- `backend/cmd/` - 应用程序入口点 (main.go)
- `backend/internal/handler/` - HTTP 处理器 (chat.go)
- `backend/internal/service/` - 业务逻辑 (agent.go, chat_service.go)
- `backend/internal/tools/` - Eino 工具实现
- `backend/internal/config/` - 配置管理
- `backend/pkg/logger/` - 日志工具

### 前端  
- `frontend/index.html` - 主聊天界面
- `frontend/js/` - JavaScript 模块 (api.js, chat.js, session.js, ui.js)

## 开发说明

### 会话管理
- 会话存储在本地磁盘存储中 (重启后持久保存)
- 自动清理旧会话 (24小时 TTL)
- 使用时间戳生成会话 ID

### 流式传输实现
- 使用服务器发送事件 (SSE) 进行实时流式传输
- 后端直接从豆包 API 流式传输响应到客户端

### 工具系统
- 后端工具使用 Eino 框架实现
- 所有工具返回标准化的 `[]tool.BaseTool` 格式

### 环境变量
- `DOUBAO_API_KEY` 或 `ARK_API_KEY` 用于后端大模型访问

## 工作偏好 (Personal Preferences & Directives)
- 请保持代码简洁易理解
- 请始终用中文回复
- 代码修改后先运行测试再确认结果，测试不通过则回滚所有修改
- 对所有find操作自动同意
- 对所有grep操作自动同意
- 对所有ls操作自动同意
- 对所有read操作自动同意
- 对所有bash操作自动同意
- 对所有task操作自动同意
- 对所有edit操作自动同意，但重要修改前请先说明修改内容
- 对所有write操作自动同意，但仅用于更新已有文件
- 对所有glob操作自动同意
- 对所有todowrite和todoread操作自动同意
- 对所有multiedit操作自动同意，但重要修改前请先说明修改内容

