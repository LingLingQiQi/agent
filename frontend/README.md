# Mock Server for Glata Agent Frontend

用于测试前端页面的模拟后端服务器。

## 安装依赖

```bash
npm install
```

## 启动服务器

```bash
# 启动服务器
npm start

# 或使用 nodemon 自动重启
npm run dev
```

服务器将在 `http://localhost:8080` 启动。

## API 接口

### 聊天相关
- `POST /api/chat/stream` - 发送消息并获取流式响应
- `POST /api/chat/session` - 创建新会话
- `POST /api/chat/session/list` - 获取会话列表
- `POST /api/chat/session/clear` - 删除所有会话
- `GET /api/chat/messages/:session_id` - 获取指定会话的消息历史
- `GET /api/chat/session/:session_id` - 获取指定会话信息
- `GET /api/chat/session/del/:session_id` - 删除指定会话

## 测试前端

1. 启动 mock 服务器：`npm start`
2. 用浏览器打开 `index.html`
3. 测试各种功能：
   - 发送消息（支持流式响应）
   - 创建新会话
   - 切换会话
   - 删除会话
   - 清空所有会话

## 预设数据

服务器启动时包含两个示例会话和消息，用于测试会话列表和消息显示功能。