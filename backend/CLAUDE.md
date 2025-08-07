# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目架构

这是一个基于 Go 和 eino 开发框架实现的 Agent 应用， 用户可以输入需求, Agent 通过任务拆解,循环调用各种工具来完成任务执行,并通过 SSE 将结果流式输出给用户。

### 核心组件
- **聊天接口**: 实时流式AI聊天
- **会话管理**: 内存存储的多会话聊天历史

## 开发命令

### Go后端开发
```bash
cd backend
go mod tidy                    # 整理依赖
go run cmd/main.go            # 启动开发服务器（端口8443）
./scripts/build.sh            # 构建生产二进制文件
```

# 测试API
```bash
curl -X POST http://localhost:8443/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'
```

## 技术栈

### 后端技术
- **Go 1.23** 与 Gin 框架
- **内存会话存储** (基于map，带清理机制)
- **Server-Sent Events** 流式响应
- **豆包AI模型** 集成（火山引擎）
- **结构化日志** 使用logrus
- **Viper** 配置管理

## API接口
- `POST /api/chat/stream` - 流式聊天
- `POST /api/chat/session` - 创建新会话
- `POST /api/chat/session/list` - 获取会话列表
- `GET /api/chat/session/:session_id` - 获取会话信息
- `GET /api/chat/messages/:session_id` - 获取会话消息历史


### 配置说明
后端配置文件 `backend/configs/config.yaml`:
- 服务器设置（端口8443，超时配置）
- 豆包API设置（密钥，基础URL，模型ID）
- 日志设置

## 文件结构

### Go后端目录
- `backend/` - 后端根目录
  - `cmd/` - 应用程序入口点 (main.go)
  - `internal/handler/` - HTTP处理器 (chat.go, health.go)
  - `internal/service/` - 业务逻辑 (agent_service.go, agent_service.go)
  - `internal/model/` - 大模型封装,通过eino-ext/compontents/model/ark 的NewChatModel创建大模型节点 
  - `internal/config/` - 配置管理
  - `internal/utils/` - 工具函数 (http_client.go, sse.go)
  - `pkg/logger/` - 日志包
  - `configs/` - 配置文件
  - `scripts/` - 构建脚本

## 开发说明

### 会话管理
- 会话存储在本地磁盘中（重启后持久化）
- 自动清理旧会话（24小时TTL）
- 会话ID使用时间戳生成

### 流式实现
- 使用Server-Sent Events (SSE)进行实时流式传输
- 后端直接从豆包API流式传输响应到客户端

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