# CLAUDE.md

这个文件为 Claude Code (claude.ai/code) 在处理此仓库代码时提供指导。

## 项目架构

这是一个现代化的 AI 智能体系统，支持多种大模型提供商，基于任务拆解和工具调用的理念，帮助用户解决 IT 领域相关问题。系统由两个主要部分组成：

1. **后端 (Go)**: 基于 Eino 框架的 AI 智能体服务，支持多种大模型和 MCP 工具集成
2. **前端 (React)**: 现代化的 TypeScript + React + Vite 聊天界面，支持实时流式渲染

### 核心组件

**后端 (Go)**:
- **多模型支持**: 豆包 (ARK)、OpenAI、千问 (Qwen) 等大模型提供商
- **智能体系统**: 基于 Eino 框架的任务拆解和执行引擎
- **工具生态**: 集成 MCP (Model Context Protocol) 和自定义工具
- **会话管理**: 基于磁盘持久化的多会话系统
- **流式传输**: 使用 SSE (Server-Sent Events) 实时流式响应

**前端 (React + TypeScript)**:
- **现代框架**: React 19.1.0 + TypeScript + Vite 构建系统
- **流式渲染**: 基于 ds-markdown 的实时 Markdown 渲染
- **会话隔离**: 独立的会话渲染和样式保护系统
- **响应式设计**: Tailwind CSS 4.x + 现代 UI 组件

## 开发命令

### 后端 (Go)
```bash
cd backend
go mod tidy                    # 清理依赖项  
go run cmd/main.go            # 启动开发服务器 (端口 8443)
```

### 前端 (React + Vite)
```bash
cd frontend
npm install                   # 安装依赖
npm run dev                   # 启动开发服务器 (端口 8080)
npm run build                 # 构建生产版本
npm run lint                  # 代码检查
npm run preview               # 预览生产构建
```

### 测试后端 API
```bash
curl -X POST http://localhost:8443/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'
```

## 技术栈

### 后端技术
- **Go 1.23.6** + Gin 1.10.1 Web 框架
- **Eino 0.4.0** AI 智能体开发框架
- **多模型支持**: 
  - 豆包 (ARK) - 火山引擎
  - OpenAI GPT-4o
  - 千问 (Qwen) - 阿里云
- **MCP 工具系统**: Desktop Commander、高德地图等
- **存储系统**: 磁盘持久化 + 内存缓存
- **流式传输**: Server-Sent Events (SSE)
- **结构化日志**: logrus
- **配置管理**: Viper

### 前端技术
- **React 19.1.0** + **TypeScript 5.8.3**
- **Vite 5.4.10** 构建工具和开发服务器
- **Tailwind CSS 4.1.11** 原子化 CSS 框架
- **ds-markdown 0.1.8** Markdown 渲染组件
- **@heroicons/react 2.2.0** 图标库
- **ESLint 9.30.1** 代码检查工具

## API 端点

### 聊天 APIs
- `POST /api/chat/stream` - 流式聊天消息
- `POST /api/chat/session` - 创建新会话  
- `POST /api/chat/session/list` - 获取会话列表
- `POST /api/chat/session/clear` - 删除所有会话
- `GET /api/chat/session/:session_id` - 获取会话信息
- `GET /api/chat/messages/:session_id` - 获取会话消息历史
- `PUT /api/chat/session/:session_id` - 更新会话标题
- `GET /api/chat/session/del/:session_id` - 删除指定会话

## 配置

### 后端配置 (`backend/configs/config.yaml`)
- **服务器设置**: 端口 8443, 30分钟超时配置
- **模型选择器**: 支持 doubao/openai/qwen 三种提供商切换
- **智能体配置**: 系统提示词、计划生成、执行策略、总结模式
- **工具系统**: MCP Desktop Commander 等工具配置
- **存储配置**: 磁盘存储、缓存、备份和同步设置
- **CORS、日志、限流**: 完整的生产环境配置

### 前端配置
- **Vite 配置**: 开发服务器端口 8080
- **TypeScript**: 严格模式配置
- **Tailwind**: 自定义主题和组件样式
- **ESLint**: React hooks 和 TypeScript 规则

## 关键文件结构

### 后端
```
backend/
├── cmd/main.go                    # 应用程序入口点
├── configs/config.yaml            # 配置文件
├── internal/
│   ├── handler/chat.go           # HTTP 处理器
│   ├── service/                  # 业务逻辑层
│   │   ├── agent.go             # 智能体服务
│   │   └── chat_service.go      # 聊天服务
│   ├── model/                    # 大模型适配层
│   │   ├── model.go             # 模型工厂
│   │   └── openai_adapter.go    # OpenAI 适配器
│   ├── storage/                  # 存储抽象层
│   │   ├── disk.go              # 磁盘存储实现
│   │   └── interface.go         # 存储接口
│   ├── tools/                    # Eino 工具实现
│   │   ├── common.go            # 工具通用方法
│   │   ├── desktop_commander_mcp_tool.go  # MCP 工具
│   │   └── *.go                 # 各种业务工具
│   └── utils/                    # 工具函数
│       ├── http_client.go       # HTTP 客户端
│       └── sse.go               # SSE 流式传输
└── pkg/logger/                   # 日志包
```

### 前端
```
frontend/
├── src/
│   ├── App.tsx                   # 主应用组件
│   ├── main.tsx                  # React 应用入口
│   ├── MessageDisplay.tsx       # 消息显示组件
│   ├── StreamingMessageDisplay.tsx  # 流式消息组件
│   ├── SessionIsolatedRenderManager.ts  # 会话隔离管理
│   ├── StyleProtectionManager.ts  # 样式保护管理
│   └── assets/                   # 静态资源
├── public/                       # 公共文件
├── package.json                  # 依赖和脚本配置
├── vite.config.ts               # Vite 构建配置
├── tsconfig.json                # TypeScript 配置
├── tailwind.config.js           # Tailwind CSS 配置
└── eslint.config.js             # ESLint 配置
```

## 开发说明

### 会话管理
- **持久化存储**: 基于磁盘的会话和消息存储，支持重启后数据恢复
- **自动清理**: 24小时 TTL 自动清理过期会话
- **会话隔离**: 前端实现独立的会话渲染和样式保护

### 流式传输实现
- **后端 SSE**: 使用 Server-Sent Events 实现实时流式传输
- **前端流式渲染**: 基于 ds-markdown 的实时 Markdown 渲染
- **消息状态管理**: 完整的流式消息状态跟踪和渲染

### 智能体系统
- **任务拆解**: 基于用户需求自动生成 TODO 列表
- **工具调用**: 支持 HTTP、MCP 等多种工具类型
- **执行引擎**: 智能的任务执行和验证机制
- **结果汇总**: 自动生成执行总结和评估

### 多模型支持
- **统一接口**: 通过 Eino 框架统一不同模型的调用接口
- **动态切换**: 支持配置文件热切换不同模型提供商
- **参数适配**: 针对不同模型的特定参数优化

### 环境变量
- `DOUBAO_API_KEY` 或 `ARK_API_KEY` - 豆包模型访问密钥
- `OPENAI_API_KEY` - OpenAI 模型访问密钥  
- `QWEN_API_KEY` - 千问模型访问密钥

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

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.