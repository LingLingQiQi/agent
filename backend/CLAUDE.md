# CLAUDE.md

这个文件为 Claude Code (claude.ai/code) 在处理后端代码时提供指导。

## 后端架构概述

这是一个基于 Go 和 Eino 框架实现的现代化 AI 智能体后端系统。采用任务拆解、工具调用的设计理念，支持多种大模型提供商，提供完整的智能体对话和任务执行能力。

### 核心特性

- **多模型支持**: 豆包 (ARK)、OpenAI、千问 (Qwen) 三种大模型提供商统一接口
- **智能体引擎**: 基于 Eino 框架的任务分解、执行、汇总完整流程
- **工具生态**: 支持 MCP (Model Context Protocol) 和自定义 HTTP 工具
- **会话管理**: 磁盘持久化存储，支持多会话并发和自动清理
- **流式传输**: SSE (Server-Sent Events) 实时流式响应
- **企业级特性**: 完整的日志、监控、限流、CORS 配置

## 技术栈详细信息

### 核心框架和库
- **Go 1.23.6** - 现代 Go 语言版本
- **Gin 1.10.1** - 高性能 HTTP Web 框架
- **Eino 0.4.0** - 字节跳动开源的 AI 智能体开发框架
- **Eino-ext** - Eino 框架的模型和工具扩展包

### 大模型集成
- **ARK (豆包)** - 火山引擎豆包大模型，通过 `eino-ext/components/model/ark` 集成
- **OpenAI** - GPT-4o 模型，通过 `sashabaranov/go-openai` 客户端集成  
- **Qwen** - 阿里云千问大模型，通过 `eino-ext/components/model/qwen` 集成

### 工具和服务
- **MCP 工具**: `eino-ext/components/tool/mcp` 支持 Model Context Protocol
- **UUID 生成**: `google/uuid` 用于会话 ID 生成
- **日志系统**: `sirupsen/logrus` 结构化日志
- **配置管理**: `spf13/viper` 支持多种配置格式
- **CORS 支持**: `gin-contrib/cors` 跨域请求处理

## 开发命令

### 基本开发流程
```bash
cd backend
go mod tidy                    # 整理依赖
go run cmd/main.go            # 启动开发服务器（端口 8443）
```

### 测试和验证
```bash
# 测试基本聊天接口
curl -X POST http://localhost:8443/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'

# 获取会话列表
curl -X POST http://localhost:8443/api/chat/session/list

# 创建新会话
curl -X POST http://localhost:8443/api/chat/session
```

## 详细模块说明

### 配置系统 (`configs/config.yaml`)

**服务器配置**:
- 端口: 8443 (HTTPS 端口，支持本地开发)
- 超时: 读写均为 30 分钟，适应长时间的智能体任务执行

**模型配置**:
- 支持 `doubao`、`openai`、`qwen` 三种提供商
- 统一的模型参数配置 (max_tokens、temperature、timeout)
- 每个模型独立的 API 密钥和端点配置

**智能体配置**:
- `system_prompt`: 智能体基础系统提示词
- `plan_prompt`: 任务规划阶段的专门提示词，支持 TODO 列表生成
- `execute_prompt`: 任务执行阶段的详细策略和验证规则
- `update_todo_list_prompt`: TODO 列表更新的智能判断逻辑
- `summary_prompt`: 任务总结阶段的格式化输出要求

**存储配置**:
- 类型: `disk` (磁盘持久化存储)
- 数据目录: `./data` 
- 会话 TTL: 24 小时自动清理
- 缓存大小: 1000 个会话的内存缓存
- 备份间隔: 24 小时自动备份

### 目录结构详解

#### 入口层 (`cmd/`)
```go
// cmd/main.go - 应用程序启动入口
// 负责配置加载、服务初始化、路由注册、服务启动
```

#### 处理器层 (`internal/handler/`)
```go
// chat.go - HTTP 请求处理器
// - StreamChat: 流式聊天处理 (POST /api/chat/stream)
// - CreateSession: 会话创建 (POST /api/chat/session)  
// - GetSessionList: 会话列表 (POST /api/chat/session/list)
// - GetSession: 获取会话详情 (GET /api/chat/session/:id)
// - GetMessages: 获取消息历史 (GET /api/chat/messages/:id)
// - UpdateSession: 更新会话标题 (PUT /api/chat/session/:id)
// - DeleteSession: 删除会话 (GET /api/chat/session/del/:id)
// - ClearSessions: 清空所有会话 (POST /api/chat/session/clear)
```

#### 业务逻辑层 (`internal/service/`)

**智能体服务** (`agent.go`):
```go
// AgentService - 核心智能体服务类
// - RunAgent: 执行完整的智能体对话流程
// - 支持三个主要阶段:
//   1. Plan 阶段: 分析用户意图，生成执行计划
//   2. Execute 阶段: 逐步执行任务，调用工具
//   3. Summary 阶段: 总结执行结果，生成报告

// ProgressEvent - 执行进度事件结构
// 支持 node_start、node_complete、node_error 等事件类型
// 用于实时向前端推送执行进度

// MessageCleaner - 消息清理器  
// 自动过滤无效消息 (空 role 或 content)
// 确保消息格式符合模型要求
```

**聊天服务** (`chat_service.go`):
```go
// ChatService - 聊天会话管理服务
// - 会话生命周期管理 (创建、更新、删除)
// - 消息历史存储和检索  
// - 与存储层的协调交互
```

#### 模型适配层 (`internal/model/`)

**模型工厂** (`model.go`):
```go
// CreateModel - 模型工厂方法
// 根据配置动态创建不同的模型实例:
// - ARK (豆包): eino-ext/components/model/ark
// - OpenAI: 通过 OpenAI 适配器  
// - Qwen: eino-ext/components/model/qwen

// 统一的模型接口，屏蔽不同模型的实现差异
```

**OpenAI 适配器** (`openai_adapter.go`):
```go
// OpenAIAdapter - OpenAI 模型的 Eino 适配器
// 将 OpenAI 的 API 调用适配到 Eino 框架接口
// 支持流式和非流式两种调用模式
// 处理 OpenAI 特有的消息格式和参数
```

#### 存储抽象层 (`internal/storage/`)

**存储接口** (`interface.go`):
```go
// Storage - 统一存储接口
// 定义会话、消息、TODO 列表的 CRUD 操作
// 支持多种存储后端 (当前实现磁盘存储)
```

**磁盘存储实现** (`disk.go`):
```go
// DiskStorage - 磁盘持久化存储实现
// - 会话存储: ./data/sessions.json + ./data/sessions/*.json
// - 消息存储: ./data/messages/*.json  
// - TODO 列表: ./data/todolists/*.md
// - 内存缓存: 提升读取性能
// - 自动备份: 定期备份到 ./data/backup/
// - 文件锁: 确保并发安全
```

#### 工具系统 (`internal/tools/`)

**工具通用框架** (`common.go`):
```go
// 统一的工具调用框架
// BaseRequest/BaseResponse - 工具调用的标准格式
// makeToolHTTPRequest - HTTP 工具调用的通用方法
// 工具调用超时控制和错误处理
```

**MCP 工具** (`desktop_commander_mcp_tool.go`):
```go
// DesktopCommanderMCPTool - Desktop Commander MCP 工具
// 支持桌面环境的自动化操作
// 基于 MCP (Model Context Protocol) 协议
// 提供文件操作、应用控制、系统信息等功能
```

**业务工具**:
```go
// allocate_device_tool.go - 设备分配工具
// assign_2_agent_tool.go - 任务分配工具  
// diagnose_meeting_room_tool.go - 会议室诊断工具
// edit_ticket_tool.go - 工单编辑工具
// fill_ticket_tool.go - 工单填写工具
// gaode_map_mcp_tool.go - 高德地图 MCP 工具
// hand_over_helpdesk_tool.go - 服务台移交工具
// repair_meeting_room_tool.go - 会议室维修工具
// return_device_tool.go - 设备归还工具

// 所有工具都遵循 Eino tool.BaseTool 接口
// 支持参数验证、执行结果返回、错误处理
```

#### 工具函数层 (`internal/utils/`)

**HTTP 客户端** (`http_client.go`):
```go
// 统一的 HTTP 客户端封装
// 支持超时控制、重试机制、错误处理
// 用于工具调用外部 API
```

**SSE 流式传输** (`sse.go`):
```go
// SSE (Server-Sent Events) 实现
// 支持实时流式数据推送到前端
// 消息格式化、连接管理、错误恢复
```

#### 日志系统 (`pkg/logger/`)
```go
// 基于 logrus 的结构化日志系统
// 支持不同日志级别 (debug、info、warn、error)
// JSON 格式输出，便于日志分析
// 上下文日志，包含会话 ID、用户 ID 等信息
```

## API 接口详细说明

### 核心聊天接口

**`POST /api/chat/stream`** - 流式聊天
```json
Request: {
  "message": "用户消息内容",
  "session_id": "会话ID (可选，不提供时创建新会话)"
}

Response: SSE 流式响应
data: {"type": "progress", "data": {"event_type": "node_start", "node_name": "plan", "message": "开始规划任务"}}
data: {"type": "content", "data": "流式返回的内容"}
data: {"type": "progress", "data": {"event_type": "node_complete", "node_name": "plan", "message": "任务规划完成"}}
```

**`POST /api/chat/session`** - 创建新会话
```json
Response: {
  "session_id": "1754534988429790000",
  "title": "新对话",
  "created_at": "2025-01-07T10:30:00Z"
}
```

**`POST /api/chat/session/list`** - 获取会话列表
```json
Response: [
  {
    "id": "1754534988429790000",
    "title": "会话标题",
    "created_at": "2025-01-07T10:30:00Z",
    "updated_at": "2025-01-07T11:00:00Z"
  }
]
```

### 会话管理接口

**`GET /api/chat/session/:session_id`** - 获取会话详情
**`GET /api/chat/messages/:session_id`** - 获取会话消息历史  
**`PUT /api/chat/session/:session_id`** - 更新会话标题
**`GET /api/chat/session/del/:session_id`** - 删除指定会话
**`POST /api/chat/session/clear`** - 清空所有会话

## 智能体执行流程

### 1. Plan 阶段 (任务规划)
- 分析用户意图和需求
- 判断是否为 IT 相关问题
- 生成 Markdown 格式的 TODO 列表
- 输出格式: `[MODE:TODO_LIST]` 或 `[MODE:DIRECT_REPLY]`

### 2. Execute 阶段 (任务执行)
- 逐步执行 TODO 列表中的任务
- 智能选择和调用合适的工具
- 实时更新任务状态 (pending → in_progress → completed/failed)
- 根据任务类型优化验证策略 (文件操作、服务启动、程序测试等)

### 3. Summary 阶段 (结果总结)
- 分析执行结果和遇到的问题
- 评估完成效果和质量
- 生成结构化的总结报告
- 格式: 完成情况 + 遇到问题 + 结果评估

## 数据存储结构

### 会话数据 (`./data/sessions.json`)
```json
{
  "1754534988429790000": {
    "id": "1754534988429790000",
    "title": "会话标题",  
    "created_at": "2025-01-07T10:30:00Z",
    "updated_at": "2025-01-07T11:00:00Z"
  }
}
```

### 消息数据 (`./data/messages/{session_id}.json`)
```json
[
  {
    "id": "msg_001",
    "role": "user",
    "content": "用户消息内容",
    "timestamp": "2025-01-07T10:30:00Z"
  },
  {
    "id": "msg_002", 
    "role": "assistant",
    "content": "AI 回复内容",
    "timestamp": "2025-01-07T10:31:00Z"
  }
]
```

### TODO 列表 (`./data/todolists/{session_id}.md`)
```markdown
- [x] 1：已完成的任务
- [ ] 2：待执行的任务  
- [!] 3：执行失败的任务
```

## 环境变量配置

```bash
# 必需的 API 密钥 (根据配置的 model.provider 选择)
export DOUBAO_API_KEY="your_doubao_api_key"    # 豆包模型
export OPENAI_API_KEY="your_openai_api_key"    # OpenAI 模型
export QWEN_API_KEY="your_qwen_api_key"        # 千问模型

# 可选的调试配置
export LOG_LEVEL="debug"                       # 日志级别
export DATA_DIR="./custom_data"                # 自定义数据目录
```

## 开发和调试

### 日志查看
```bash
# 实时查看日志
tail -f ./logs/app.log

# 查看特定会话的日志  
grep "session_id=1754534988429790000" ./logs/app.log
```

### 性能监控
- 响应时间监控: 通过日志记录每个请求的执行时间
- 内存使用: 会话缓存和消息存储的内存占用
- 工具调用统计: 各种工具的调用频率和成功率

### 故障排查
1. **模型调用失败**: 检查 API 密钥和网络连接
2. **存储错误**: 检查磁盘空间和文件权限  
3. **工具调用超时**: 调整工具超时配置
4. **内存泄露**: 监控会话清理机制是否正常工作

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