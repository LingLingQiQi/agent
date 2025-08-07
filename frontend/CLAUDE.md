# Glata Agent Frontend

AI聊天应用的前端页面，使用纯HTML + Tailwind CSS + JavaScript实现。

## 项目结构

- `index.html` - 主页面文件，包含完整的聊天界面

## 功能特性

### 1. 右侧聊天窗口
- **消息发送**: 用户在输入框输入文字后，点击发送按钮或按回车键发送消息
- **后端接口**: 调用 `POST /api/chat/stream` 接口发送消息
- **流式响应**: 后端通过 SSE (Server-Sent Events) 流式返回结果
- **消息渲染**: 使用 DS-Markdown 组件 (`git@github.com:onshinpei/ds-markdown.git`) 流式渲染消息内容
- **交互功能**: 支持点赞、点踩、复制、分享和重新生成等操作

### 2. 左侧会话列表栏
- **创建新会话**: 点击 "New chat" 按钮调用 `POST /api/chat/session` 接口
- **会话列表**: 调用 `POST /api/chat/session/list` 获取所有历史会话
- **会话切换**: 点击会话后调用以下接口：
  - `GET /api/chat/messages/:session_id` - 获取会话历史消息
  - `GET /api/chat/session/:session_id` - 获取会话信息
- **会话管理**: 支持编辑、删除单个会话，以及清空所有会话

## API 接口

### 聊天相关
- `POST /api/chat/stream` - 发送消息并获取流式响应
- `POST /api/chat/session` - 创建新会话
- `POST /api/chat/session/list` - 获取会话列表
- `POST /api/chat/session/clear` - 删除所有会话
- `GET /api/chat/messages/:session_id` - 获取指定会话的消息历史
- `GET /api/chat/session/:session_id` - 获取指定会话信息
- `GET /api/chat/session/del/:session_id` - 删除指定会话

## 技术栈

- **UI框架**: Tailwind CSS (通过CDN引入)
- **图标库**: Font Awesome 6.0.0
- **Markdown渲染**: DS-Markdown 组件
- **通信协议**: HTTP + SSE (Server-Sent Events)

## 样式主题

使用自定义Tailwind配置：
- 主背景色: `#cad9f0`
- 聊天背景: `#f8f9fa` 
- 侧边栏背景: `#ffffff`
- 主题蓝色: `#6366f1`
- 选中状态: `#f0f4ff`

## 用户界面

- **响应式设计**: 支持不同屏幕尺寸
- **现代化界面**: 圆角设计、阴影效果、悬停状态
- **用户体验**: 流畅的动画过渡效果

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