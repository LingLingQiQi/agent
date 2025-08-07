// ✅ 约束2&3：会话隔离的Web Worker渲染器
// 严格的会话ID验证和后台流式渲染处理

interface MarkdownRenderer {
    render: (content: string) => Promise<string>;
}

interface SessionWorkerMessage {
    type: 'INIT_SESSION' | 'TAKE_OVER_STREAMING' | 'STREAM_CHUNK' | 'FINISH_STREAMING' | 'RENDER_MESSAGE' | 'BATCH_RENDER' | 'EXTRACT_HTML';
    sessionId: string;
    messageId?: string;
    content?: string;
    messages?: any[];
    isBackground?: boolean;
    html?: string; // 用于接收从DOM提取的HTML
}

interface WorkerResponse {
    type: 'RENDER_COMPLETE' | 'BACKGROUND_RENDER_COMPLETE' | 'STREAMING_UPDATE' | 'RENDER_ERROR' | 'REQUEST_HTML_EXTRACT';
    sessionId: string;
    messageId?: string;
    html?: string;
    renderTime?: number;
    isBackground?: boolean;
    content?: string;
    error?: string;
    fallbackContent?: string;
}

class SessionIsolatedRenderWorker {
    private sessionId: string | null = null;              // 当前Worker负责的会话ID
    private isBackground: boolean = false;                // 是否为后台模式
    private streamingBuffer: string[] = [];               // 流式内容缓冲区
    private mdRenderer: MarkdownRenderer | null = null;   // Markdown渲染器

    constructor() {
        this.initMarkdownRenderer();
    }

    // 初始化Markdown渲染器
    private async initMarkdownRenderer(): Promise<void> {
        try {
            // 简单的Markdown渲染器实现
            this.mdRenderer = {
                render: async (content: string): Promise<string> => {
                    // 基本的Markdown转HTML
                    let html = content
                        // 代码块
                        .replace(/```(\w+)?\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>')
                        // 内联代码
                        .replace(/`([^`]+)`/g, '<code>$1</code>')
                        // 粗体
                        .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
                        // 斜体
                        .replace(/\*([^*]+)\*/g, '<em>$1</em>')
                        // 链接
                        .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank">$1</a>')
                        // 标题
                        .replace(/^### (.+)$/gm, '<h3>$1</h3>')
                        .replace(/^## (.+)$/gm, '<h2>$1</h2>')
                        .replace(/^# (.+)$/gm, '<h1>$1</h1>')
                        // 列表
                        .replace(/^\- (.+)$/gm, '<li>$1</li>')
                        .replace(/(<li>.*<\/li>)/s, '<ul>$1</ul>')
                        // 换行
                        .replace(/\n/g, '<br>');
                    
                    return html;
                }
            };
            console.log('✅ Worker: Markdown渲染器初始化完成');
        } catch (error) {
            console.error('❌ Worker: Markdown渲染器初始化失败:', error);
        }
    }

    // 初始化会话Worker
    initSession(sessionId: string): void {
        this.sessionId = sessionId;
        console.log(`✅ Worker初始化 - 会话ID: ${sessionId}`);
    }

    // ✅ 约束3：接管流式渲染到后台
    takeOverStreaming(sessionId: string, isBackground: boolean = true): void {
        if (this.sessionId !== sessionId) {
            console.error(`❌ Worker会话ID不匹配: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }
        
        this.isBackground = isBackground;
        
        console.log(`🔀 会话 ${sessionId} 流式渲染转为后台模式`);
    }

    // 处理流式内容块
    async handleStreamChunk(sessionId: string, messageId: string, chunk: string): Promise<void> {
        // ✅ 约束2：验证会话ID匹配
        if (this.sessionId !== sessionId) {
            console.warn(`⚠️ 收到错误会话的流式内容: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }

        this.streamingBuffer.push(chunk);
        
        // 在后台模式下，不进行中间渲染，等待流式结束
        if (this.isBackground) {
            // 静默积累内容，等待 FINISH_STREAMING 时统一渲染
            console.log(`📦 后台积累内容: 会话 ${sessionId}, 消息块 ${chunk.length} 字符`);
        } else {
            // 前台模式才发送UI更新
            const response: WorkerResponse = {
                type: 'STREAMING_UPDATE',
                sessionId: this.sessionId,
                messageId: messageId,
                content: this.streamingBuffer.join('')
            };
            self.postMessage(response);
        }
    }

    // 后台静默渲染流式内容 (暂时未使用)
    /*
    private async renderStreamingContent(messageId: string, content: string): Promise<void> {
        try {
            const startTime = performance.now();
            const html = await this.renderMarkdown(content);
            const renderTime = Math.round(performance.now() - startTime);
            
            // 只保存渲染结果，不更新UI
            const response: WorkerResponse = {
                type: 'BACKGROUND_RENDER_COMPLETE',
                sessionId: this.sessionId!,
                messageId: messageId,
                html: html,
                renderTime: renderTime,
                isBackground: true
            };
            self.postMessage(response);
            
        } catch (error) {
            console.error(`❌ 后台流式渲染失败 (会话: ${this.sessionId}):`, error);
        }
    }
    */

    // 完成流式渲染
    async finishStreaming(sessionId: string, messageId: string): Promise<void> {
        if (this.sessionId !== sessionId) return;
        
        const finalContent = this.streamingBuffer.join('');
        
        // 清空缓冲区
        this.streamingBuffer = [];
        
        // 请求主线程从DOM提取HTML
        if (finalContent) {
            console.log(`✅ 流式结束，请求DOM HTML提取: 会话 ${sessionId}, 消息 ${messageId}`);
            
            // 发送提取HTML请求到主线程
            const extractRequest: WorkerResponse = {
                type: 'REQUEST_HTML_EXTRACT',
                sessionId: this.sessionId!,
                messageId: messageId,
                content: finalContent
            };
            self.postMessage(extractRequest);
        }
    }

    // 渲染单个消息
    async renderMessage(content: string, messageId: string): Promise<void> {
        const startTime = performance.now();
        
        try {
            const html = await this.renderMarkdown(content);
            const renderTime = Math.round(performance.now() - startTime);
            
            const response: WorkerResponse = {
                type: 'RENDER_COMPLETE',
                sessionId: this.sessionId!,
                messageId: messageId,
                html: html,
                renderTime: renderTime,
                isBackground: this.isBackground
            };
            self.postMessage(response);
            
        } catch (error) {
            console.error(`❌ 渲染失败 (会话: ${this.sessionId}):`, error);
            
            const errorResponse: WorkerResponse = {
                type: 'RENDER_ERROR',
                sessionId: this.sessionId!,
                messageId: messageId,
                error: error instanceof Error ? error.message : 'Unknown error',
                fallbackContent: content
            };
            self.postMessage(errorResponse);
        }
    }

    // 批量渲染处理
    async handleBatchRender(sessionId: string, messages: any[]): Promise<void> {
        if (this.sessionId !== sessionId) {
            console.warn(`⚠️ 批量渲染会话ID不匹配: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }

        for (const message of messages) {
            // ✅ 约束2：再次验证消息属于正确的会话
            if (message.session_id !== this.sessionId) {
                console.warn(`⚠️ 跳过不匹配的消息: ${message.id}`);
                continue;
            }

            if (message.role === 'assistant' && !message.is_rendered) {
                await this.renderMessage(message.content, message.id);
                
                // 避免阻塞主线程
                await new Promise(resolve => setTimeout(resolve, 10));
            }
        }
    }

    // Markdown渲染核心方法
    private async renderMarkdown(content: string): Promise<string> {
        if (!this.mdRenderer) {
            throw new Error('Markdown渲染器未初始化');
        }
        
        return await this.mdRenderer.render(content);
    }
    
    // 保存从主线程提取的HTML
    async saveExtractedHTML(sessionId: string, messageId: string, html: string): Promise<void> {
        if (this.sessionId !== sessionId) {
            console.warn(`⚠️ 保存HTML时会话ID不匹配: 期望 ${this.sessionId}, 实际 ${sessionId}`);
            return;
        }
        
        try {
            console.log(`✅ 保存提取的HTML: 会话 ${sessionId}, 消息 ${messageId}`);
            
            const response: WorkerResponse = {
                type: 'RENDER_COMPLETE',
                sessionId: this.sessionId!,
                messageId: messageId,
                html: html,
                renderTime: 0, // 前端提取，无渲染时间
                isBackground: this.isBackground
            };
            self.postMessage(response);
            
        } catch (error) {
            console.error(`❌ 保存提取HTML失败 (会话: ${this.sessionId}):`, error);
            
            const errorResponse: WorkerResponse = {
                type: 'RENDER_ERROR',
                sessionId: this.sessionId!,
                messageId: messageId,
                error: error instanceof Error ? error.message : 'Unknown error'
            };
            self.postMessage(errorResponse);
        }
    }
}

// Worker消息处理
let worker: SessionIsolatedRenderWorker | null = null;

self.onmessage = async (e: MessageEvent<SessionWorkerMessage>) => {
    if (!worker) {
        worker = new SessionIsolatedRenderWorker();
    }
    
    const { type, sessionId, messageId, content, messages, isBackground } = e.data;
    
    try {
        switch (type) {
            case 'INIT_SESSION':
                worker.initSession(sessionId);
                break;
                
            case 'TAKE_OVER_STREAMING':
                worker.takeOverStreaming(sessionId, isBackground);
                break;
                
            case 'STREAM_CHUNK':
                if (messageId && content !== undefined) {
                    await worker.handleStreamChunk(sessionId, messageId, content);
                }
                break;
                
            case 'FINISH_STREAMING':
                if (messageId) {
                    worker.finishStreaming(sessionId, messageId);
                }
                break;
                
            case 'RENDER_MESSAGE':
                if (messageId && content !== undefined) {
                    await worker.renderMessage(content, messageId);
                }
                break;
                
            case 'BATCH_RENDER':
                if (messages) {
                    await worker.handleBatchRender(sessionId, messages);
                }
                break;
                
            case 'EXTRACT_HTML':
                if (messageId && e.data.html) {
                    // 接收从主线程提取的HTML并保存
                    await worker.saveExtractedHTML(sessionId, messageId, e.data.html);
                }
                break;
                
            default:
                console.warn(`⚠️ 未知的Worker消息类型: ${type}`);
        }
    } catch (error) {
        console.error(`❌ Worker处理消息失败:`, error);
        
        const errorResponse: WorkerResponse = {
            type: 'RENDER_ERROR',
            sessionId: sessionId,
            messageId: messageId,
            error: error instanceof Error ? error.message : 'Unknown error'
        };
        self.postMessage(errorResponse);
    }
};

// Worker错误处理
self.onerror = (error) => {
    console.error('❌ Worker全局错误:', error);
};

self.onunhandledrejection = (event) => {
    console.error('❌ Worker未处理的Promise拒绝:', event.reason);
};

console.log('✅ SessionIsolatedRenderWorker已启动');