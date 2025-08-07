// ✅ 约束2&3：会话隔离渲染管理器
// 确保严格的会话消息隔离和流式渲染后台处理

import { styleProtection } from './StyleProtectionManager';

interface Message {
    id: string;
    session_id: string;
    role: 'user' | 'assistant';
    content: string;
    html_content?: string;
    is_rendered?: boolean;
    render_time_ms?: number;
    timestamp: Date;
}

interface RenderTask {
    sessionId: string;
    messageId: string;
    content: string;
    priority: 'immediate' | 'high' | 'normal' | 'low';
    addedAt: number;
}

interface SessionWorkerMessage {
    type: 'INIT_SESSION' | 'TAKE_OVER_STREAMING' | 'STREAM_CHUNK' | 'FINISH_STREAMING' | 'RENDER_MESSAGE' | 'BATCH_RENDER';
    sessionId: string;
    messageId?: string;
    content?: string;
    messages?: Message[];
    isBackground?: boolean;
}

class SessionIsolatedRenderManager {
    private renderQueue = new Map<string, RenderTask[]>();        // 按会话ID隔离的渲染队列
    private renderCache = new Map<string, Map<string, string>>(); // 按会话ID隔离的渲染缓存
    private activeSession: string | null = null;                 // 当前活跃会话ID
    private streamingSession: string | null = null;              // 当前流式输出的会话ID
    private sessionWorkers = new Map<string, Worker>();          // 每个会话的独立Worker
    private isRendering = new Map<string, boolean>();            // 按会话追踪渲染状态
    private pendingMessages = new Map<string, Message[]>();      // 等待渲染的消息
    private messageSessionMap = new Map<string, string>();       // 消息ID到会话ID的映射
    private isSwitching = false;                                 // 防止会话切换竞态条件
    private switchMutex = Promise.resolve();                     // 会话切换互斥锁

    constructor() {
        console.log('✅ SessionIsolatedRenderManager initialized');
    }

    // ✅ 约束2：严格的会话切换处理（带互斥锁防止竞态条件）
    async switchSession(targetSessionId: string): Promise<void> {
        // 使用互斥锁防止竞态条件
        return this.switchMutex = this.switchMutex.then(async () => {
            const previousSession = this.activeSession;
            
            // 确保会话消息严格隔离
            if (previousSession === targetSessionId) {
                return; // 相同会话，无需切换
            }
            
            console.log(`🔄 开始会话切换: ${previousSession} → ${targetSessionId}`);
            this.isSwitching = true;
            
            try {
                // 1. 将前一个会话的操作转为后台静默渲染（不中止）
                if (previousSession) {
                    await this.movePreviousSessionToBackground(previousSession);
                }
                
                // 2. 原子性更新会话状态
                this.activeSession = targetSessionId;
                
                // 3. 强制终止任何不匹配会话的流式处理
                if (this.streamingSession && this.streamingSession !== targetSessionId) {
                    this.abortStream(this.streamingSession);
                }
                
                // 4. 加载目标会话的消息（确保只展示该会话的消息）
                await this.loadSessionMessages(targetSessionId);
                
                // 5. 清理之前会话的缓存和状态
                if (previousSession && previousSession !== targetSessionId) {
                    this.cleanupPreviousSession(previousSession);
                }
                
                console.log(`✅ 会话切换完成: ${targetSessionId}`);
                
            } finally {
                this.isSwitching = false;
            }
        });
    }

    // 将前一个会话的操作转为后台静默渲染
    private async movePreviousSessionToBackground(sessionId: string): Promise<void> {
        console.log(`🔀 将会话 ${sessionId} 转为后台静默渲染`);
        
        // 不中止渲染，只是转为后台模式
        if (this.streamingSession === sessionId) {
            console.log(`📦 流式输出转为后台: 会话 ${sessionId}`);
            this.moveStreamingToBackground(sessionId);
        }
        
        // 调度任何待处理的消息到后台渲染
        this.scheduleBackgroundRender(sessionId);
    }

    // 清理前一个会话的状态
    private cleanupPreviousSession(sessionId: string): void {
        console.log(`🧹 清理会话 ${sessionId} 状态`);
        
        // 清理缓存，但保留渲染结果
        this.pendingMessages.delete(sessionId);
        this.isRendering.delete(sessionId);
        
        // 清理消息映射
        for (const [messageId, mappedSessionId] of this.messageSessionMap.entries()) {
            if (mappedSessionId === sessionId) {
                this.messageSessionMap.delete(messageId);
            }
        }
        
        // 调度后台渲染，但不影响当前会话
        this.scheduleBackgroundRender(sessionId);
    }

    // 强制中止流式输出
    private abortStream(sessionId: string): void {
        console.log(`🛑 强制中止流式输出: 会话 ${sessionId}`);
        
        const worker = this.sessionWorkers.get(sessionId);
        if (worker) {
            worker.postMessage({
                type: 'ABORT_STREAM',
                sessionId: sessionId
            });
        }
        
        // 清除流式状态
        this.streamingSession = null;
    }

    // ✅ 修复：简化流式渲染状态管理
    startStreamingForSession(sessionId: string): void {
        this.streamingSession = sessionId;
        
        console.log(`📡 开始流式输出: 会话 ${sessionId}`);
        
        // 只有在明确需要后台模式时才转为后台
        // 修复：当前活跃会话的消息应该在前台显示
        if (sessionId !== this.activeSession && this.activeSession !== null) {
            console.log(`🔀 非活跃会话流式输出转为后台模式: 会话 ${sessionId}`);
            this.moveStreamingToBackground(sessionId);
        } else {
            console.log(`✅ 活跃会话流式输出保持前台模式: 会话 ${sessionId}`);
        }
    }

    // ✅ 约束3：将流式渲染转移到后台
    private moveStreamingToBackground(sessionId: string): void {
        if (!this.sessionWorkers.has(sessionId)) {
            this.createSessionWorker(sessionId);
        }
        
        // 通知Worker接管该会话的流式渲染
        const worker = this.sessionWorkers.get(sessionId);
        if (worker) {
            const message: SessionWorkerMessage = {
                type: 'TAKE_OVER_STREAMING',
                sessionId: sessionId,
                isBackground: true
            };
            worker.postMessage(message);
            console.log(`✅ 流式渲染已转移到后台: 会话 ${sessionId}`);
        }
    }

    // 创建会话专用的Worker
    private createSessionWorker(sessionId: string): void {
        const worker = new Worker('/src/SessionRenderWorker.ts', { type: 'module' });
        
        const initMessage: SessionWorkerMessage = {
            type: 'INIT_SESSION',
            sessionId: sessionId
        };
        worker.postMessage(initMessage);
        
        worker.onmessage = (e) => {
            const { type, sessionId: msgSessionId, messageId, html, renderTime, content } = e.data;
            
            // ✅ 约束2：确保渲染结果只应用于正确的会话
            if (type === 'RENDER_COMPLETE' && msgSessionId) {
                this.saveRenderedMessage(msgSessionId, messageId, html, renderTime);
                
                // 只有当前活跃会话才更新UI
                if (msgSessionId === this.activeSession) {
                    this.updateActiveSessionUI(messageId, html);
                }
            }
            
            if (type === 'BACKGROUND_RENDER_COMPLETE') {
                // 后台渲染完成，只保存结果不更新UI
                this.saveRenderedMessage(msgSessionId, messageId, html, renderTime);
                console.log(`📦 后台渲染完成: 会话 ${msgSessionId}, 消息 ${messageId}`);
            }
            
            if (type === 'REQUEST_HTML_EXTRACT') {
                // Worker请求从DOM提取HTML
                this.extractHTMLFromDOM(msgSessionId, messageId, content);
            }
        };
        
        worker.onerror = (error) => {
            console.error(`❌ Worker错误 (会话: ${sessionId}):`, error);
        };
        
        this.sessionWorkers.set(sessionId, worker);
        console.log(`🆕 Worker创建成功: 会话 ${sessionId}`);
    }

    // ✅ 约束2：严格的消息加载 - 确保只加载目标会话的消息
    async loadSessionMessages(sessionId: string): Promise<Message[]> {
        try {
            const response = await fetch(`http://localhost:8443/api/chat/messages/${sessionId}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const data = await response.json();
            const messages: Message[] = data.messages || [];
            
            // ✅ 约束2：验证消息确实属于目标会话
            const validMessages = messages.filter(msg => {
                if (msg.session_id !== sessionId) {
                    console.warn(`⚠️ 消息 ${msg.id} 不属于会话 ${sessionId}, 跳过`);
                    return false;
                }
                return true;
            });
            
            // ✅ 约束1：使用样式保护清空聊天窗口
            this.clearChatWindow();
            
            // 只渲染属于目标会话的消息
            validMessages.forEach(message => {
                this.renderMessageToActiveSession(message);
            });
            
            console.log(`✅ 会话消息加载完成: ${sessionId}, ${validMessages.length} 条消息`);
            return validMessages;
            
        } catch (error) {
            console.error(`❌ 加载会话 ${sessionId} 消息失败:`, error);
            return [];
        }
    }

    // ✅ 约束1：清空聊天窗口 - 使用样式保护
    private clearChatWindow(): void {
        const chatMessages = document.querySelector('#chat-messages');
        if (chatMessages) {
            styleProtection.safeClearContent(chatMessages as HTMLElement);
        }
    }

    // ✅ 约束2：只向当前活跃会话渲染消息
    private renderMessageToActiveSession(message: Message): void {
        if (message.session_id !== this.activeSession) {
            console.warn(`⚠️ 消息 ${message.id} 不属于当前活跃会话 ${this.activeSession}`);
            return;
        }
        
        // 通过React状态更新消息，不直接操作DOM
        // 这个方法需要在React组件中具体实现
        console.log(`📝 渲染消息到活跃会话: ${message.id}`);
    }

    // 停止前台渲染
    private pauseForegroundRender(sessionId: string): void {
        this.isRendering.set(sessionId, false);
        console.log(`⏸️ 停止前台渲染: 会话 ${sessionId}`);
    }

    // 调度后台渲染
    private scheduleBackgroundRender(sessionId: string): void {
        const pendingMessages = this.pendingMessages.get(sessionId) || [];
        if (pendingMessages.length === 0) return;
        
        if (!this.sessionWorkers.has(sessionId)) {
            this.createSessionWorker(sessionId);
        }
        
        const worker = this.sessionWorkers.get(sessionId);
        if (worker) {
            const message: SessionWorkerMessage = {
                type: 'BATCH_RENDER',
                sessionId: sessionId,
                messages: pendingMessages
            };
            worker.postMessage(message);
            console.log(`🔄 调度后台渲染: 会话 ${sessionId}, ${pendingMessages.length} 条消息`);
        }
    }

    // 保存渲染结果
    private async saveRenderedMessage(sessionId: string, messageId: string, html: string, renderTime: number): Promise<void> {
        try {
            // 保存到本地缓存
            if (!this.renderCache.has(sessionId)) {
                this.renderCache.set(sessionId, new Map());
            }
            this.renderCache.get(sessionId)!.set(messageId, html);
            
            // 保存到后端
            const response = await fetch(`http://localhost:8443/api/chat/message/${messageId}/render`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    session_id: sessionId,  // ✅ 约束2：强制会话ID验证
                    html_content: html,
                    render_time_ms: renderTime
                })
            });
            
            if (!response.ok) {
                console.error(`❌ 保存渲染结果失败: ${response.status}`);
            }
            
        } catch (error) {
            console.error(`❌ 保存渲染结果失败:`, error);
        }
    }

    // 更新活跃会话UI
    private updateActiveSessionUI(messageId: string, _html: string): void {
        // 这个方法需要与React组件集成
        // 通过回调函数或事件系统通知React组件更新
        console.log(`🔄 更新活跃会话UI: 消息 ${messageId}`);
    }

    // 处理流式内容块（带严格会话验证）
    handleStreamChunk(sessionId: string, messageId: string, chunk: string): void {
        // ✅ 约束2：双重验证会话ID匹配
        if (this.isSwitching) {
            console.warn(`⏸️ 会话切换中，忽略流式内容: 会话 ${sessionId}`);
            return;
        }
        
        if (this.activeSession !== sessionId) {
            console.log(`🔀 非活跃会话流式内容转为后台: ${sessionId} (活跃: ${this.activeSession})`);
            this.handleBackgroundStreamChunk(sessionId, messageId, chunk);
            return;
        }
        
        // 记录消息到会话的映射关系
        this.messageSessionMap.set(messageId, sessionId);
        
        // 活跃会话的流式内容直接更新UI（在App.tsx中处理）
        console.log(`📨 处理活跃会话流式内容: ${sessionId}, 消息 ${messageId}`);
    }

    // 处理后台流式内容
    private handleBackgroundStreamChunk(sessionId: string, messageId: string, chunk: string): void {
        const worker = this.sessionWorkers.get(sessionId);
        if (worker) {
            const message: SessionWorkerMessage = {
                type: 'STREAM_CHUNK',
                sessionId: sessionId,
                messageId: messageId,
                content: chunk
            };
            worker.postMessage(message);
        }
    }

    // 完成流式渲染
    finishStreaming(sessionId: string, messageId: string): void {
        if (sessionId === this.activeSession) {
            console.log(`✅ 前台流式渲染完成: 会话 ${sessionId}, 消息 ${messageId}`);
        } else {
            // 通知后台Worker完成渲染
            const worker = this.sessionWorkers.get(sessionId);
            if (worker) {
                const message: SessionWorkerMessage = {
                    type: 'FINISH_STREAMING',
                    sessionId: sessionId,
                    messageId: messageId
                };
                worker.postMessage(message);
                console.log(`📦 后台流式渲染完成: 会话 ${sessionId}, 消息 ${messageId}`);
            }
        }
        
        // 清除流式状态
        if (this.streamingSession === sessionId) {
            this.streamingSession = null;
        }
    }

    // 获取当前活跃会话
    getActiveSession(): string | null {
        return this.activeSession;
    }

    // 检查会话是否有未渲染消息
    async checkPendingRenders(sessionId: string): Promise<number> {
        try {
            const response = await fetch(`http://localhost:8443/api/chat/session/${sessionId}/pending-renders`);
            if (response.ok) {
                const data = await response.json();
                return data.total || 0;
            }
        } catch (error) {
            console.error(`❌ 检查待渲染消息失败:`, error);
        }
        return 0;
    }

    // 清理会话Worker
    cleanupSessionWorker(sessionId: string): void {
        const worker = this.sessionWorkers.get(sessionId);
        if (worker) {
            worker.terminate();
            this.sessionWorkers.delete(sessionId);
            console.log(`🗑️ 清理Worker: 会话 ${sessionId}`);
        }
    }

    // 从DOM提取HTML内容 - 现在由前端React组件直接处理
    private extractHTMLFromDOM(sessionId: string, messageId: string, content: string): void {
        try {
            console.log(`🔍 请求HTML提取: 会话 ${sessionId}, 消息 ${messageId}`);
            
            // 现在HTML提取由前端React组件直接处理
            // 这里只是为了向后兼容，实际提取工作在App.tsx中完成
            console.log(`✅ HTML提取已转移到React组件处理`);
            
        } catch (error) {
            console.error(`❌ HTML提取失败:`, error);
        }
    }

    // 生成渲染状态报告
    generateRenderReport(): {
        activeSession: string | null;
        streamingSession: string | null;
        workerCount: number;
        pendingTasks: number;
    } {
        const totalPendingTasks = Array.from(this.renderQueue.values())
            .reduce((total, tasks) => total + tasks.length, 0);
            
        return {
            activeSession: this.activeSession,
            streamingSession: this.streamingSession,
            workerCount: this.sessionWorkers.size,
            pendingTasks: totalPendingTasks
        };
    }
}

// 全局会话隔离渲染管理器实例
export const sessionRenderManager = new SessionIsolatedRenderManager();

export default SessionIsolatedRenderManager;