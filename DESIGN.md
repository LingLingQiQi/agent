## 📋 需求与约束
1. **后端存储**：按会话ID隔离存储，会话和历史信息分开存储
2. **前端后台渲染**：用户切换会话后，前端在后台渲染非活跃会话的消息
3. **HTML持久化**：前端渲染完成后，调用后端接口保存HTML格式

### 🚨 关键约束条件
1. **前端样式保护**：不能修改任何前端样式，保持现有UI界面不变
2. **会话消息隔离**：点击会话标题后，聊天窗口必须准确展示该会话内的消息，不能展示其他会话的消息
3. **流式渲染隔离**：当发出消息且AI正在流式回复时，切换到其他会话窗口，流式生成和渲染动作必须切换到前端后台静默执行，确保新消息不会在无关会话中实时渲染和展示

## 🔄 架构设计

### 1. 后端存储结构调整

#### 简化后的存储结构
```
backend/data/
├── sessions/          # 会话元数据（保持不变）
├── messages/          # 消息内容（扩展支持HTML）
└── sessions.json      # 会话索引（保持不变）
```

### 2. 修正后的数据模型

#### 扩展消息结构
```go
type Message struct {
    ID          string    `json:"id"`
    SessionID   string    `json:"session_id"`
    Role        string    `json:"role"`
    Content     string    `json:"content"`      // 原始Markdown内容
    HTMLContent string    `json:"html_content"` // 渲染后的HTML
    IsRendered  bool      `json:"is_rendered"`  // 是否已渲染
    RenderTime  int64     `json:"render_time_ms"` // 渲染耗时(毫秒)
    Timestamp   time.Time `json:"timestamp"`
}

// 渲染状态跟踪（仅用于前端状态管理）
type RenderStatus struct {
    SessionID   string            `json:"session_id"`
    MessageID   string            `json:"message_id"`
    IsRendered  bool              `json:"is_rendered"`
    RenderTime  int64             `json:"render_time_ms"`
    LastUpdate  time.Time         `json:"last_update"`
}
```

### 3. 前端架构设计 - 严格会话隔离

#### 会话隔离渲染管理器
```javascript
class SessionIsolatedRenderManager {
    constructor() {
        this.renderQueue = new Map();        // 按会话ID隔离的渲染队列
        this.renderCache = new Map();        // 按会话ID隔离的渲染缓存
        this.activeSession = null;           // 当前活跃会话ID
        this.streamingSession = null;        // 当前流式输出的会话ID
        this.backgroundRenderer = null;      // 后台渲染器
        this.sessionWorkers = new Map();     // 每个会话的独立Worker
        this.isRendering = new Map();        // 按会话追踪渲染状态
    }

    // 严格的会话切换处理
    async switchSession(targetSessionId) {
        const previousSession = this.activeSession;
        
        // ✅ 约束2：确保会话消息严格隔离
        if (previousSession === targetSessionId) {
            return; // 相同会话，无需切换
        }
        
        // 1. 立即停止前一个会话的所有前台渲染
        if (previousSession) {
            this.pauseForegroundRender(previousSession);
        }
        
        // 2. 设置新的活跃会话
        this.activeSession = targetSessionId;
        
        // ✅ 约束3：流式渲染隔离处理
        if (this.streamingSession && this.streamingSession !== targetSessionId) {
            this.moveStreamingToBackground(this.streamingSession);
        }
        
        // 3. 加载目标会话的消息（确保只展示该会话的消息）
        await this.loadSessionMessages(targetSessionId);
        
        // 4. 将之前会话移至后台渲染
        if (previousSession && previousSession !== targetSessionId) {
            this.scheduleBackgroundRender(previousSession);
        }
    }

    // ✅ 约束3：流式渲染状态管理
    startStreamingForSession(sessionId) {
        this.streamingSession = sessionId;
        
        // 如果流式输出的会话不是当前活跃会话，立即转为后台模式
        if (sessionId !== this.activeSession) {
            this.moveStreamingToBackground(sessionId);
        }
    }

    // 将流式渲染转移到后台
    moveStreamingToBackground(sessionId) {
        if (!this.sessionWorkers.has(sessionId)) {
            this.createSessionWorker(sessionId);
        }
        
        // 通知Worker接管该会话的流式渲染
        const worker = this.sessionWorkers.get(sessionId);
        worker.postMessage({
            type: 'TAKE_OVER_STREAMING',
            sessionId: sessionId,
            isBackground: true
        });
    }

    // 创建会话专用的Worker
    createSessionWorker(sessionId) {
        const worker = new Worker('/js/session-render-worker.js');
        
        worker.postMessage({
            type: 'INIT_SESSION',
            sessionId: sessionId
        });
        
        worker.onmessage = (e) => {
            const { type, sessionId: msgSessionId, messageId, html, renderTime } = e.data;
            
            // ✅ 约束2：确保渲染结果只应用于正确的会话
            if (type === 'RENDER_COMPLETE' && msgSessionId) {
                this.saveRenderedMessage(msgSessionId, messageId, html, renderTime);
                
                // 只有当前活跃会话才更新UI
                if (msgSessionId === this.activeSession) {
                    this.updateActiveSessionUI(messageId, html);
                }
            }
        };
        
        this.sessionWorkers.set(sessionId, worker);
    }

    // 严格的消息加载 - 确保只加载目标会话的消息
    async loadSessionMessages(sessionId) {
        try {
            const response = await fetch(`/api/chat/messages/${sessionId}`);
            const messages = await response.json();
            
            // ✅ 约束2：验证消息确实属于目标会话
            const validMessages = messages.filter(msg => msg.session_id === sessionId);
            
            // 清空当前聊天窗口
            this.clearChatWindow();
            
            // 只渲染属于目标会话的消息
            validMessages.forEach(message => {
                this.renderMessageToActiveSession(message);
            });
            
            return validMessages;
        } catch (error) {
            console.error(`加载会话 ${sessionId} 消息失败:`, error);
            return [];
        }
    }

    // 清空聊天窗口
    clearChatWindow() {
        const chatMessages = document.querySelector('#chat-messages');
        if (chatMessages) {
            chatMessages.innerHTML = '';
        }
    }

    // 只向当前活跃会话渲染消息
    renderMessageToActiveSession(message) {
        if (message.session_id !== this.activeSession) {
            console.warn(`消息 ${message.id} 不属于当前活跃会话 ${this.activeSession}`);
            return;
        }
        
        // 正常渲染到UI
        this.appendMessageToUI(message);
    }
}
```

#### 会话隔离的Web Worker渲染器
```javascript
// session-render-worker.js - 严格会话隔离的渲染器
class SessionIsolatedRenderWorker {
    constructor() {
        this.mdRenderer = new MarkdownRenderer();
        this.sessionId = null;              // 当前Worker负责的会话ID
        this.isBackground = false;          // 是否为后台模式
        this.streamingBuffer = [];          // 流式内容缓冲区
        this.isStreamingActive = false;     // 流式渲染状态
    }

    // 初始化会话Worker
    initSession(sessionId) {
        this.sessionId = sessionId;
        console.log(`Worker初始化 - 会话ID: ${sessionId}`);
    }

    // ✅ 约束3：接管流式渲染到后台
    takeOverStreaming(sessionId, isBackground = true) {
        if (this.sessionId !== sessionId) {
            console.error(`Worker会话ID不匹配: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }
        
        this.isBackground = isBackground;
        this.isStreamingActive = true;
        
        console.log(`会话 ${sessionId} 流式渲染转为后台模式`);
    }

    // 处理流式内容块
    async handleStreamChunk(sessionId, messageId, chunk) {
        // ✅ 约束2：验证会话ID匹配
        if (this.sessionId !== sessionId) {
            console.warn(`收到错误会话的流式内容: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }

        this.streamingBuffer.push(chunk);
        
        // 在后台静默渲染，不向主线程发送UI更新
        if (this.isBackground) {
            await this.renderStreamingContent(messageId, this.streamingBuffer.join(''));
        } else {
            // 前台模式才发送UI更新
            self.postMessage({
                type: 'STREAMING_UPDATE',
                sessionId: this.sessionId,
                messageId: messageId,
                content: this.streamingBuffer.join('')
            });
        }
    }

    // 后台静默渲染流式内容
    async renderStreamingContent(messageId, content) {
        try {
            const startTime = performance.now();
            const html = await this.mdRenderer.render(content);
            const renderTime = Math.round(performance.now() - startTime);
            
            // 只保存渲染结果，不更新UI
            self.postMessage({
                type: 'BACKGROUND_RENDER_COMPLETE',
                sessionId: this.sessionId,
                messageId: messageId,
                html: html,
                renderTime: renderTime,
                isBackground: true
            });
            
        } catch (error) {
            console.error(`后台流式渲染失败 (会话: ${this.sessionId}):`, error);
        }
    }

    // 完成流式渲染
    finishStreaming(sessionId, messageId) {
        if (this.sessionId !== sessionId) return;
        
        this.isStreamingActive = false;
        const finalContent = this.streamingBuffer.join('');
        
        // 清空缓冲区
        this.streamingBuffer = [];
        
        // 最终渲染
        this.renderMessage(finalContent, messageId);
    }

    async renderMessage(content, messageId) {
        const startTime = performance.now();
        
        try {
            const html = await this.mdRenderer.render(content);
            const renderTime = Math.round(performance.now() - startTime);
            
            self.postMessage({
                type: 'RENDER_COMPLETE',
                sessionId: this.sessionId,
                messageId: messageId,
                html: html,
                renderTime: renderTime,
                isBackground: this.isBackground
            });
            
        } catch (error) {
            console.error(`渲染失败 (会话: ${this.sessionId}):`, error);
            self.postMessage({
                type: 'RENDER_ERROR',
                sessionId: this.sessionId,
                messageId: messageId,
                error: error.message,
                fallbackContent: content
            });
        }
    }

    async handleBatchRender(sessionId, messages) {
        if (this.sessionId !== sessionId) {
            console.warn(`批量渲染会话ID不匹配: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }

        for (const message of messages) {
            // ✅ 约束2：再次验证消息属于正确的会话
            if (message.session_id !== this.sessionId) {
                console.warn(`跳过不匹配的消息: ${message.id}`);
                continue;
            }

            if (message.role === 'assistant' && !message.is_rendered) {
                await this.renderMessage(message.content, message.id);
                
                // 避免阻塞主线程
                await new Promise(resolve => setTimeout(resolve, 10));
            }
        }
    }
}

// Worker消息处理
self.onmessage = async (e) => {
    if (!self.worker) {
        self.worker = new SessionIsolatedRenderWorker();
    }
    
    const { type, sessionId, messageId, content, messages, isBackground } = e.data;
    
    switch (type) {
        case 'INIT_SESSION':
            self.worker.initSession(sessionId);
            break;
            
        case 'TAKE_OVER_STREAMING':
            self.worker.takeOverStreaming(sessionId, isBackground);
            break;
            
        case 'STREAM_CHUNK':
            await self.worker.handleStreamChunk(sessionId, messageId, content);
            break;
            
        case 'FINISH_STREAMING':
            self.worker.finishStreaming(sessionId, messageId);
            break;
            
        case 'RENDER_MESSAGE':
            await self.worker.renderMessage(content, messageId);
            break;
            
        case 'BATCH_RENDER':
            await self.worker.handleBatchRender(sessionId, messages);
            break;
            
        default:
            console.warn(`未知的Worker消息类型: ${type}`);
    }
};
```

### 4. 前端样式保护策略

#### ✅ 约束1：前端样式保护实现
```javascript
class StyleProtectionManager {
    constructor() {
        // 禁止任何CSS修改
        this.PROTECTED_SELECTORS = [
            '#chat-messages',
            '.message',
            '.message-user',
            '.message-assistant',
            '.session-item',
            '.sidebar',
            // 所有现有CSS选择器
        ];
        
        // 只允许动态内容更新，不允许样式修改
        this.ALLOWED_OPERATIONS = [
            'innerHTML',    // 更新消息内容
            'textContent',  // 更新文本内容
            'appendChild',  // 添加新消息
            'removeChild',  // 移除消息
        ];
        
        // 禁止的样式操作
        this.FORBIDDEN_OPERATIONS = [
            'style',        // 直接样式修改
            'className',    // CSS类修改
            'classList',    // CSS类列表修改
            'setAttribute', // 样式属性修改
        ];
    }

    // 安全的DOM操作包装器
    safeUpdateContent(element, content) {
        if (!element) return;
        
        // 只更新内容，不触碰样式
        element.innerHTML = content;
    }
    
    safeClearContent(element) {
        if (!element) return;
        
        // 清空内容但保持所有样式
        element.innerHTML = '';
    }
    
    safeAppendMessage(container, messageHtml) {
        if (!container) return;
        
        // 创建消息元素但不修改任何样式
        const messageDiv = document.createElement('div');
        messageDiv.innerHTML = messageHtml;
        container.appendChild(messageDiv);
    }
}

// 全局样式保护实例
const styleProtection = new StyleProtectionManager();
```

#### 实现细节
1. **纯内容更新**：只修改DOM元素的内容，绝不触碰CSS类、样式属性或任何视觉相关的属性
2. **现有UI复用**：完全复用现有的聊天界面、侧边栏、消息显示区域
3. **安全操作封装**：所有DOM操作通过安全包装器进行，确保不会意外修改样式

### 5. 约束实现总结

#### ✅ 约束1：前端样式保护
- **实现方式**：创建StyleProtectionManager，封装所有DOM操作
- **保护范围**：聊天界面、侧边栏、消息区域的所有样式
- **操作限制**：只允许innerHTML/textContent更新，禁止样式相关操作

#### ✅ 约束2：会话消息严格隔离  
- **实现方式**：SessionIsolatedRenderManager + 严格的会话ID验证
- **隔离机制**：
  - 会话切换时立即清空聊天窗口
  - 加载消息时双重验证会话ID匹配
  - Worker渲染结果验证会话ID一致性
  - 拒绝处理错误会话的消息

#### ✅ 约束3：流式渲染后台隔离
- **实现方式**：SessionIsolatedRenderWorker + 流式状态管理
- **隔离机制**：
  - 检测会话切换时自动将流式渲染转为后台模式
  - 后台Worker静默渲染，不更新UI
  - 流式内容缓冲在Worker中，不影响当前活跃会话
  - 完成后台渲染后只保存结果，不更新界面

### 6. 后端API调整

#### 新增API端点 - 支持会话隔离
```http
# 更新消息渲染结果 - 增加会话ID验证
PUT /api/chat/message/:message_id/render
{
  "session_id": "session_123",           # ✅ 约束2：强制会话ID验证
  "html_content": "<p>渲染后的HTML内容</p>",
  "render_time_ms": 150
}

# 批量更新渲染结果 - 按会话ID分组
PUT /api/chat/session/:session_id/render-batch
{
  "renders": [
    {
      "message_id": "msg_123",
      "html_content": "<p>内容1</p>",
      "render_time_ms": 100
    },
    {
      "message_id": "msg_124", 
      "html_content": "<p>内容2</p>",
      "render_time_ms": 120
    }
  ]
}

# 获取未渲染的消息 - 严格按会话ID过滤
GET /api/chat/session/:session_id/pending-renders
Response:
{
  "session_id": "session_123",          # ✅ 约束2：返回响应包含会话ID
  "messages": [
    {
      "id": "msg_123",
      "session_id": "session_123",      # ✅ 约束2：每条消息都有会话ID
      "content": "原始markdown内容",
      "timestamp": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 5,
  "estimated_render_time_ms": 750
}

# ✅ 约束3：流式消息端点增强 - 支持后台模式
POST /api/chat/stream
{
  "session_id": "session_123",          # 明确指定会话ID
  "message": "用户消息",
  "background_mode": false              # 是否为后台模式
}

# 流式响应格式
data: {
  "type": "chunk",
  "session_id": "session_123",          # ✅ 约束2&3：每个响应包含会话ID
  "message_id": "msg_124",
  "content": "部分内容",
  "is_background": false                # ✅ 约束3：标识是否为后台模式
}
```

#### 修正后的后端存储接口
```go
type Storage interface {
    // 会话管理（保持不变）
    CreateSession(session *model.Session) error
    GetSession(sessionID string) (*model.Session, error)
    UpdateSession(session *model.Session) error
    DeleteSession(sessionID string) error
    ListSessions() ([]*model.Session, error)
    
    // 消息管理（扩展支持HTML）
    AddMessage(sessionID string, message *model.Message) error
    GetMessages(sessionID string) ([]*model.Message, error)
    UpdateMessageRender(sessionID, messageID, htmlContent string, renderTime int64) error
    UpdateMessagesRender(sessionID string, renders []model.RenderUpdate) error
    GetPendingRenders(sessionID string) ([]*model.Message, error)
    
    // 存储管理（保持不变）
    Init() error
    Close() error
    Backup() error
}

// 渲染更新结构
type RenderUpdate struct {
    MessageID   string `json:"message_id"`
    HTMLContent string `json:"html_content"`
    RenderTime  int64  `json:"render_time_ms"`
}
```

### 5. 前端缓存策略

#### 智能缓存管理
```javascript
class RenderCache {
    constructor(maxSize = 50) {
        this.cache = new Map();
        this.maxSize = maxSize;
        this.accessOrder = [];
    }

    get(sessionId, messageId) {
        const key = `${sessionId}:${messageId}`;
        const cached = this.cache.get(key);
        if (cached) {
            this.updateAccessOrder(key);
            return cached;
        }
        return null;
    }

    set(sessionId, messageId, html, renderTime) {
        const key = `${sessionId}:${messageId}`;
        
        if (this.cache.size >= this.maxSize) {
            this.evictLRU();
        }
        
        this.cache.set(key, {
            html,
            renderTime,
            timestamp: Date.now()
        });
        this.accessOrder.push(key);
    }

    // 会话切换时的缓存处理
    onSessionSwitch(oldSessionId, newSessionId) {
        // 保留新会话的缓存
        const newSessionKeys = Array.from(this.cache.keys())
            .filter(key => key.startsWith(`${newSessionId}:`));
        
        // 清理旧会话的长期缓存（保留最近使用的）
        const oldSessionKeys = Array.from(this.cache.keys())
            .filter(key => key.startsWith(`${oldSessionId}:`))
            .slice(0, -5); // 保留最近5条
            
        oldSessionKeys.forEach(key => this.cache.delete(key));
    }
}
```

### 6. 会话状态管理

#### 前端状态机
```javascript
class SessionState {
    constructor() {
        this.sessions = new Map();
        this.currentSession = null;
        this.renderWorker = null;
    }

    async activateSession(sessionId) {
        const previousSession = this.currentSession;
        
        // 1. 保存前一个会话的渲染状态
        if (previousSession) {
            await this.saveRenderState(previousSession);
        }
        
        // 2. 激活新会话
        this.currentSession = sessionId;
        
        // 3. 加载会话消息和渲染状态
        const messages = await this.loadSessionMessages(sessionId);
        const pendingRenders = messages.filter(m => !m.is_rendered);
        
        // 4. 如果有待渲染的消息，启动后台渲染
        if (pendingRenders.length > 0) {
            this.scheduleBackgroundRender(sessionId, pendingRenders);
        }
        
        return messages;
    }

    scheduleBackgroundRender(sessionId, messages) {
        // 使用requestIdleCallback在低优先级时渲染
        if ('requestIdleCallback' in window) {
            requestIdleCallback((deadline) => {
                this.renderMessagesInBackground(sessionId, messages, deadline);
            });
        } else {
            // 回退到setTimeout
            setTimeout(() => {
                this.renderMessagesInBackground(sessionId, messages);
            }, 1000);
        }
    }
}
```

### 7. 性能优化策略

#### 渲染优先级
```javascript
const RenderPriority = {
    IMMEDIATE: 1,    // 当前活跃会话
    HIGH: 2,        // 即将切换到的会话
    NORMAL: 3,      // 最近使用的会话
    LOW: 4          // 旧会话
};

class PriorityRenderer {
    constructor() {
        this.queue = [];
        this.isProcessing = false;
    }

    addToQueue(sessionId, messages, priority = RenderPriority.NORMAL) {
        this.queue.push({
            sessionId,
            messages,
            priority,
            addedAt: Date.now()
        });
        
        this.queue.sort((a, b) => b.priority - a.priority);
        this.processQueue();
    }

    async processQueue() {
        if (this.isProcessing || this.queue.length === 0) return;
        
        this.isProcessing = true;
        
        while (this.queue.length > 0) {
            const task = this.queue.shift();
            
            // 检查会话是否仍然是后台状态
            if (task.sessionId !== window.currentSessionId) {
                await this.renderTask(task);
            }
            
            // 让出控制权，避免阻塞主线程
            await new Promise(resolve => setTimeout(resolve, 50));
        }
        
        this.isProcessing = false;
    }
}
```

### 8. 错误处理与恢复

#### 前端错误处理
```javascript
class RenderErrorHandler {
    constructor() {
        this.failedRenders = new Map();
        this.retryPolicy = {
            maxRetries: 3,
            backoffMs: 1000,
            maxDelayMs: 5000
        };
    }

    async handleRenderError(sessionId, messageId, error) {
        const key = `${sessionId}:${messageId}`;
        const failures = this.failedRenders.get(key) || 0;
        
        if (failures < this.retryPolicy.maxRetries) {
            const delay = Math.min(
                this.retryPolicy.backoffMs * Math.pow(2, failures),
                this.retryPolicy.maxDelayMs
            );
            
            setTimeout(() => {
                this.retryRender(sessionId, messageId);
            }, delay);
            
            this.failedRenders.set(key, failures + 1);
        } else {
            // 标记为渲染失败，使用原文本
            this.markAsFailed(sessionId, messageId, error);
        }
    }
}
```

### 9. 存储优化

#### 批量更新策略
```javascript
class BatchUpdater {
    constructor() {
        this.pendingUpdates = [];
        this.batchSize = 10;
        this.flushInterval = 2000; // 2秒
        this.setupFlushTimer();
    }

    addUpdate(sessionId, messageId, html, renderTime) {
        this.pendingUpdates.push({
            sessionId,
            messageId,
            html,
            renderTime,
            timestamp: Date.now()
        });
        
        if (this.pendingUpdates.length >= this.batchSize) {
            this.flushUpdates();
        }
    }

    async flushUpdates() {
        if (this.pendingUpdates.length === 0) return;
        
        const updates = [...this.pendingUpdates];
        this.pendingUpdates = [];
        
        try {
            await fetch('/api/chat/session/render-batch', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ renders: updates })
            });
        } catch (error) {
            console.error('批量更新失败:', error);
            // 重试逻辑
            this.pendingUpdates.unshift(...updates);
        }
    }
}
```

这个修正后的设计去除了后端渲染相关的复杂性，专注于前端后台渲染和后端存储优化，并严格遵循三个关键约束条件，更符合您的实际需求。

## 🚀 实施建议

### 第一阶段：基础架构调整
1. **后端存储结构扩展**：添加HTML内容字段到消息模型
2. **API端点增强**：实现会话隔离的API端点
3. **样式保护实施**：创建StyleProtectionManager

### 第二阶段：会话隔离系统
1. **实施SessionIsolatedRenderManager**：确保严格的会话消息隔离
2. **创建会话专用Worker**：每个会话独立的渲染Worker
3. **消息加载验证**：双重验证会话ID匹配机制

### 第三阶段：流式渲染隔离
1. **流式状态管理**：实现流式渲染的后台切换
2. **Worker流式处理**：后台静默渲染流式内容
3. **UI更新控制**：确保后台渲染不影响当前界面

### 验证清单
- [ ] ✅ 约束1：前端样式完全不变，UI界面保持原状
- [ ] ✅ 约束2：会话切换后只显示目标会话的消息，无错误展示
- [ ] ✅ 约束3：流式回复时切换会话，渲染转为后台静默执行

### 性能指标
- 会话切换速度：< 200ms
- 后台渲染延迟：< 500ms  
- 内存使用优化：Worker复用，避免内存泄漏
- 样式保护：0个样式修改，100%兼容现有UI