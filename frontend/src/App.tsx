import { useState, useEffect, useRef } from 'react'
import 'ds-markdown/style.css'
import './App.css'
import { sessionRenderManager } from './SessionIsolatedRenderManager'
import { MessageDisplay } from './MessageDisplay'
import StreamingMessageDisplay, { StreamingMessageDisplayRef } from './StreamingMessageDisplay'

interface Message {
  id: string
  type: 'user' | 'bot'
  content: string
  timestamp: Date
  session_id?: string        // 会话ID字段
  html_content?: string      // 渲染后的HTML内容
  is_rendered?: boolean      // 是否已渲染
  is_streaming?: boolean     // 是否为流式消息
  streaming_chunks?: string[] // 流式内容块数组
  streaming_complete?: boolean // 流式传输是否完成
}

interface Session {
  id: string
  title: string
  created_at: string
  updated_at: string
}

function App() {
  const [currentSession, setCurrentSession] = useState<string | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [sessions, setSessions] = useState<Session[]>([])
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const markdownRefs = useRef<Map<string, HTMLDivElement>>(new Map()) // 用于跟踪Markdown组件的ref
  const streamingRefs = useRef<Map<string, StreamingMessageDisplayRef>>(new Map()) // 用于跟踪流式消息组件的ref

  // 提取ds-markdown渲染的HTML并保存到后端
  const extractAndSaveHTML = async (messageId: string, sessionId: string) => {
    console.log(`🔍 开始HTML提取: 消息${messageId}, 会话${sessionId}`)
    
    // ✅ 跳过错误消息的HTML提取
    if (messageId.startsWith('error-') || messageId.startsWith('temp-') || messageId.startsWith('suggestion-')) {
      console.log(`⏭️ 跳过临时/错误/建议消息的HTML提取: ${messageId}`)
      return
    }
    
    // 防止重复提取
    const message = messages.find(msg => msg.id === messageId)
    if (message && message.is_rendered) {
      console.log(`⏭️ 消息 ${messageId} 已渲染过，跳过HTML提取`)
      return
    }
    
    // ✅ 优化消息内容完整性判断，增强容错性
    if (message && (!message.content || message.content.trim().length === 0)) {
      console.warn(`⚠️ 消息内容为空: 消息${messageId}，跳过HTML提取`)
      return
    }
    
    // ✅ 对于短消息（如单个字符、表情等），不应视为不完整
    // 只有当消息明显不完整时才跳过（例如只有HTML标签开头）
    if (message && message.content && message.content.trim().length > 0) {
      const trimmedContent = message.content.trim()
      // 检查是否为明显不完整的内容（只有标签开头、只有空白字符等）
      const seemsIncomplete = (
        trimmedContent === '<' ||
        trimmedContent === '<think' ||
        trimmedContent === '<thinking' ||
        /^<[^>]*$/.test(trimmedContent) // 只匹配未完成的开始标签
      )
      
      if (seemsIncomplete) {
        console.warn(`⚠️ 消息内容不完整: 消息${messageId}内容"${trimmedContent}"，跳过HTML提取`)
        // 延迟重试，给更多内容累积的时间
        setTimeout(() => {
          console.log(`🔄 重试HTML提取: 消息 ${messageId}`)
          extractAndSaveHTML(messageId, sessionId)
        }, 2000) // 增加延迟时间
        return
      }
    }
    
    // 🔍 使用多种方式查找DOM元素，确保健壮性
    let markdownElement = markdownRefs.current.get(messageId)
    
    // 如果refs中没有，尝试通过DOM查询查找
    if (!markdownElement) {
      // 查找包含该消息ID的DOM元素
      const selector = `[data-message-id="${messageId}"]`
      markdownElement = document.querySelector(selector) as HTMLDivElement
      
      if (!markdownElement) {
        // 尝试查找消息内容区域
        const messageContainers = document.querySelectorAll('.markdown-content')
        for (const container of messageContainers) {
          const parent = container.closest('.flex')
          if (parent && parent.textContent?.includes(message?.content?.substring(0, 50) || '')) {
            markdownElement = container as HTMLDivElement
            break
          }
        }
      }
    }
    
    console.log(`🔍 正在提取HTML: 消息 ${messageId}, DOM元素存在: ${!!markdownElement}`)
    
    if (markdownElement) {
      try {
        const htmlContent = markdownElement.innerHTML
        console.log(`📝 提取到HTML内容: ${htmlContent.length} 字符`)
        
        // ✅ 优化HTML内容完整性判断，避免对短内容的误判
        const isComplete = (
          htmlContent.includes('ds-markdown-answer') || 
          htmlContent.includes('</div>') || 
          htmlContent.includes('<pre') || 
          htmlContent.includes('<code') ||
          // ✅ 对于短内容，如果长度合理就认为是完整的
          (htmlContent.trim().length > 0 && htmlContent.trim().length <= 100 && !htmlContent.includes('<thinking'))
        );
        
        if (htmlContent && htmlContent.trim() && isComplete) {
          // 调用后端API保存HTML内容
          const response = await fetch(`http://localhost:8443/api/chat/message/${messageId}/render`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              session_id: sessionId,
              html_content: htmlContent,
              render_time_ms: 0 // ds-markdown即时渲染
            })
          })
          
          if (response.ok) {
            console.log(`✅ HTML内容已保存: 消息 ${messageId}, ${htmlContent.length} 字符`)
            
            // 更新本地消息状态
            setMessages(prev => prev.map(msg => 
              msg.id === messageId 
                ? { ...msg, html_content: htmlContent, is_rendered: true }
                : msg
            ))
          } else {
            console.error(`❌ 保存HTML失败: ${response.status}`)
          }
        } else {
          console.warn(`⚠️ HTML内容不完整或为空: 消息 ${messageId}, 长度${htmlContent.length}`)
          
          // 如果内容不完整，延迟重试
          if (message && message.content && message.content.length > 0) {
            setTimeout(() => {
              console.log(`🔄 重试提取HTML: 消息 ${messageId}`)
              extractAndSaveHTML(messageId, sessionId)
            }, 1000)
          }
        }
      } catch (error) {
        console.error(`❌ 提取HTML失败:`, error)
      }
    } else {
      console.warn(`⚠️ 未找到DOM元素: 消息 ${messageId}, 尝试重试...`)
      
      // 如果找不到DOM元素，延迟重试
      setTimeout(() => {
        console.log(`🔄 重试查找DOM元素: 消息 ${messageId}`)
        extractAndSaveHTML(messageId, sessionId)
      }, 500)
    }
  }

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  // 监听消息变化，对新消息提取HTML
  useEffect(() => {
    const latestBotMessage = messages
      .filter((msg: Message) => msg.type === 'bot' && !msg.is_rendered && msg.session_id)
      .slice(-1)[0]
    
    if (latestBotMessage && latestBotMessage.session_id && !isLoading) {
      // 延迟提取，确保ds-markdown渲染完成
      const timeoutId = setTimeout(() => {
        console.log(`⏰ useEffect触发HTML提取: 消息${latestBotMessage.id}, 会话${latestBotMessage.session_id}`)
        extractAndSaveHTML(latestBotMessage.id, latestBotMessage.session_id!)
      }, 800) // 增加延迟确保渲染完成
      
      return () => clearTimeout(timeoutId)
    }
  }, [messages, isLoading])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  // 加载会话列表
  const loadSessions = async () => {
    try {
      const response = await fetch('http://localhost:8443/api/chat/session/list', { method: 'POST' })
      if (response.ok) {
        const data = await response.json()
        setSessions(data.sessions || [])
      }
    } catch (error) {
      console.error('加载会话列表失败:', error)
    }
  }

  // 创建新会话
  const createNewSession = async () => {
    try {
      const response = await fetch('http://localhost:8443/api/chat/session', { method: 'POST' })
      if (response.ok) {
        const data = await response.json()
        const newSessionId = data.id || data.session_id
        
        // ✅ 确保会话管理器知道新的活跃会话
        await sessionRenderManager.switchSession(newSessionId)
        setCurrentSession(newSessionId)
        setMessages([])
        await loadSessions()
        
        console.log(`✅ 新会话创建成功: ${newSessionId}`)
      }
    } catch (error) {
      console.error('创建会话失败:', error)
    }
  }

  // 清除所有会话
  const clearAllSessions = async () => {
    if (window.confirm('确定要清除所有会话吗？此操作不可撤销。')) {
      try {
        const response = await fetch('http://localhost:8443/api/chat/session/clear', { method: 'POST' })
        if (response.ok) {
          setSessions([])
          setCurrentSession(null)
          setMessages([])
        }
      } catch (error) {
        console.error('清除会话失败:', error)
      }
    }
  }

  // 删除单个会话
  const deleteSession = async (sessionId: string) => {
    if (window.confirm('确定要删除这个会话吗？')) {
      try {
        const response = await fetch(`http://localhost:8443/api/chat/session/del/${sessionId}`, { method: 'GET' })
        if (response.ok) {
          setSessions(prev => prev.filter(s => s.id !== sessionId))
          if (currentSession === sessionId) {
            setCurrentSession(null)
            setMessages([])
          }
        }
      } catch (error) {
        console.error('删除会话失败:', error)
      }
    }
  }

  // 编辑会话标题
  const editSessionTitle = async (sessionId: string, currentTitle: string) => {
    const newTitle = window.prompt('请输入新的会话标题:', currentTitle)
    if (newTitle && newTitle.trim()) {
      try {
        const response = await fetch(`http://localhost:8443/api/chat/session/${sessionId}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title: newTitle.trim() })
        })
        if (response.ok) {
          setSessions(prev => prev.map(s => s.id === sessionId ? { ...s, title: newTitle.trim() } : s))
        }
      } catch (error) {
        console.error('更新会话标题失败:', error)
      }
    }
  }

  // ✅ 约束2：切换会话 - 使用会话隔离管理器
  const switchSession = async (sessionId: string) => {
    try {
      // ✅ 清理旧会话的DOM引用，防止竞态条件
      markdownRefs.current.clear()
      
      // 使用会话隔离管理器处理切换
      await sessionRenderManager.switchSession(sessionId);
      
      setCurrentSession(sessionId)
      const response = await fetch(`http://localhost:8443/api/chat/messages/${sessionId}`)
      if (response.ok) {
        const data = await response.json()
        
        // ✅ 约束2：验证消息属于正确会话并转换格式
        const convertedMessages: Message[] = (data.messages || [])
          .map((msg: any) => {
            // 验证消息属于目标会话
            if (msg.session_id !== sessionId) {
              console.warn(`⚠️ 消息 ${msg.id} 不属于会话 ${sessionId}, 跳过`);
              return null;
            }
            
            return {
              id: msg.id,
              type: msg.role === 'assistant' ? 'bot' : 'user',
              content: msg.content,
              timestamp: new Date(msg.timestamp),
              session_id: msg.session_id,  // ✅ 约束2：保持会话ID
              html_content: msg.html_content,
              is_rendered: msg.is_rendered
            };
          })
          .filter((msg: Message | null): msg is Message => msg !== null);
        
        // ✅ 约束1：使用样式保护更新消息
        setMessages(convertedMessages);
        
        // 延迟处理未渲染的历史消息
        setTimeout(() => {
          const unrenderedMessages = convertedMessages.filter(
            (msg: Message) => msg.type === 'bot' && !msg.is_rendered && msg.content && msg.content.length > 10
          );
          
          unrenderedMessages.forEach((msg: Message) => {
            console.log(`🔄 处理未渲染的历史消息: ${msg.id}`);
            extractAndSaveHTML(msg.id, msg.session_id!);
          });
        }, 1000);
        
        // 统计渲染优化情况
        const renderedCount = convertedMessages.filter((msg: Message) => msg.is_rendered && msg.html_content).length;
        const totalAssistantMessages = convertedMessages.filter((msg: Message) => msg.type === 'bot').length;
        if (totalAssistantMessages > 0) {
          console.log(`✅ 会话切换完成: ${sessionId}, ${convertedMessages.length} 条消息 (${renderedCount}/${totalAssistantMessages} 助手消息使用缓存渲染)`);
        } else {
          console.log(`✅ 会话切换完成: ${sessionId}, ${convertedMessages.length} 条消息`);
        }
      }
    } catch (error) {
      console.error('❌ 加载会话消息失败:', error)
    }
  }

  // ✅ 约束3：发送消息 - 支持流式渲染隔离
  const sendMessage = async () => {
    if (!inputMessage.trim() || isLoading) return

    // 如果没有当前会话，先创建一个
    let sessionId = currentSession
    if (!sessionId) {
      try {
        const response = await fetch('http://localhost:8443/api/chat/session', { method: 'POST' })
        if (response.ok) {
          const data = await response.json()
          sessionId = data.id || data.session_id
          // ✅ 立即设置当前会话状态，确保后续逻辑使用正确的会话ID
          console.log(`🆕 创建新会话并设置为当前: ${sessionId}`)
          setCurrentSession(sessionId)
          await sessionRenderManager.switchSession(sessionId)
          await loadSessions()
        } else {
          console.error('创建会话失败')
          return
        }
      } catch (error) {
        console.error('创建会话失败:', error)
        return
      }
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      type: 'user',
      content: inputMessage,
      timestamp: new Date(),
      session_id: sessionId || undefined  // ✅ 约束2：添加会话ID
    }

    setMessages(prev => [...prev, userMessage])
    setInputMessage('')
    setIsLoading(true)

    // ✅ 使用刚创建或获取的sessionId，而不是依赖可能未更新的currentSession状态
    console.log(`📝 当前使用的会话ID: ${sessionId}`)
    
    // ✅ 确保会话管理器知道当前活跃会话
    if (sessionId) {
      console.log(`🔄 确保会话管理器同步: ${sessionId}`)
      await sessionRenderManager.switchSession(sessionId)
    }
    
    // ✅ 启动流式输出处理
    if (sessionId) {
      sessionRenderManager.startStreamingForSession(sessionId)
    }

    try {
      // ✅ 创建AbortController来控制请求超时
      const abortController = new AbortController();
      const timeoutId = setTimeout(() => {
        console.log('⏰ SSE请求超时，主动中断连接');
        abortController.abort();
      }, 1800000); // 30分钟超时
      
      const response = await fetch('http://localhost:8443/api/chat/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          message: inputMessage,
          session_id: sessionId,
          background_mode: sessionId !== currentSession  // ✅ 约束3：后台模式标识
        }),
        signal: abortController.signal  // ✅ 添加超时控制
      })
      
      // 清除超时定时器
      clearTimeout(timeoutId);

      if (response.ok && response.body) {
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let botMessage = ''
        let backendMessageId: string | null = null
        
        // ✅ 创建临时流式消息，使用实际的sessionId而不是currentSession
        let tempMessageObj: Message | null = null;
        tempMessageObj = {
          id: 'temp-' + Date.now(),
          type: 'bot',
          content: '',
          timestamp: new Date(),
          session_id: sessionId || undefined,  // ✅ 使用实际的sessionId
          is_streaming: true,                   // 新增：标识为流式消息
          streaming_chunks: [],                 // 新增：流式内容块数组
          streaming_complete: false             // 新增：流式传输是否完成
        };
        setMessages(prev => [...prev, tempMessageObj]);
        console.log(`📝 创建临时流式消息UI for session: ${sessionId}`);

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value)
          const lines = chunk.split('\n')
          
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              if (data === '[DONE]') break
              
              try {
                const parsed = JSON.parse(data)
                
                // ✅ 处理不同类型的服务器消息
                if (line.startsWith('data: ') && line.includes('event: heartbeat')) {
                  console.log('💓 收到心跳消息，连接正常')
                  continue
                }
                
                if (line.startsWith('data: ') && line.includes('event: status')) {
                  console.log(`📊 收到状态消息: ${parsed.message || parsed.type}`)
                  if (parsed.type === 'processing_start') {
                    console.log('🚀 服务器开始处理请求')
                  } else if (parsed.type === 'processing_complete') {
                    console.log('✅ 服务器处理完成')
                  }
                  continue
                }
                
                if (line.startsWith('data: ') && line.includes('event: error')) {
                  console.error('❌ 收到服务器错误:', parsed)
                  const errorMsg = parsed.suggestion ? 
                    `${parsed.error}\n\n💡 建议: ${parsed.suggestion}` : 
                    parsed.error
                  
                  setMessages(prev => [...prev, {
                    id: 'server-error-' + Date.now(),
                    type: 'bot',
                    content: `🔧 服务器错误: ${errorMsg}`,
                    timestamp: new Date(),
                    session_id: sessionId || undefined
                  }])
                  break
                }
                
                // 处理正常的聊天消息
                if (parsed.content !== undefined && parsed.message_id) {
                  // ✅ 第一次收到数据时，获取后端返回的真实message_id
                  if (!backendMessageId) {
                    backendMessageId = parsed.message_id;
                    console.log(`📝 获取后端消息ID: ${backendMessageId}`);
                  }
                  
                  // ✅ 增强日志：详细显示接收的数据
                  console.log(`📝 接收SSE数据详情:`, {
                    messageId: parsed.message_id,
                    content: parsed.content,
                    contentLength: parsed.content ? parsed.content.length : 0,
                    phase: parsed.phase,
                    hasContent: !!parsed.content,
                    contentPreview: parsed.content ? parsed.content.substring(0, 50) + (parsed.content.length > 50 ? '...' : '') : '[空内容]'
                  });
                  
                  // 🎯 关键修复：只有当内容不为空时才进行处理，避免空内容干扰
                  if (parsed.content && parsed.content.length > 0) {
                    // ✅ 第一次收到数据时，获取后端返回的真实message_id
                    if (!backendMessageId) {
                      backendMessageId = parsed.message_id;
                      console.log(`📝 获取后端消息ID: ${backendMessageId}`);
                      
                      // 🔑 关键修复：不更新消息ID，保持使用临时ID避免React组件重新渲染
                      // 只记录后端ID用于会话管理，UI组件继续使用临时ID
                      console.log(`🔄 保持使用临时ID作为消息标识: ${tempMessageObj?.id}`);
                    }
                    
                    // 🚀 使用StreamingMessageDisplay的push方法处理流式内容
                    // 🔑 关键修复：优先使用临时ID查找ref，降级使用后端ID
                    let streamingRef = streamingRefs.current.get(tempMessageObj?.id); // 先尝试临时ID
                    if (!streamingRef && backendMessageId) {
                      streamingRef = streamingRefs.current.get(backendMessageId); // 后备：尝试后端ID
                    }
                    
                    if (streamingRef) {
                      console.log(`📡 通过StreamingMessageDisplay处理内容块: ${parsed.content.length} 字符`);
                      streamingRef.pushChunk(parsed.content);
                    } else {
                      console.warn(`⚠️ 未找到StreamingMessageDisplay ref，尝试的ID: 临时=${tempMessageObj?.id}, 后端=${backendMessageId}`);
                      
                      // 降级处理：如果ref未找到，使用传统方式累积到content字段
                      const targetId = tempMessageObj?.id || backendMessageId;
                      setMessages(prev => prev.map(msg => 
                        msg.id === targetId
                          ? { 
                              ...msg, 
                              content: (msg.content || '') + parsed.content,
                              streaming_chunks: [...(msg.streaming_chunks || []), parsed.content]
                            }
                          : msg
                      ));
                    }
                    
                    // 使用会话渲染管理器处理（如果需要）
                    if (sessionId && backendMessageId) {
                      sessionRenderManager.handleStreamChunk(sessionId, backendMessageId, parsed.content);
                    }
                    
                    console.log(`📝 流式内容处理完成:`, {
                      messageId: backendMessageId,
                      chunkLength: parsed.content.length,
                      chunkPreview: parsed.content.substring(0, 50) + (parsed.content.length > 50 ? '...' : ''),
                      hasStreamingRef: !!streamingRefs.current.get(backendMessageId)
                    });
                  } else {
                    // 空内容，可能是完成信号
                    console.log(`📝 接收到空内容，可能是完成信号，phase: ${parsed.phase}`);
                  }
                }
              } catch (e) {
                // 忽略JSON解析错误
              }
            }
          }
        }
        
        // ✅ 约束3：完成流式渲染，使用后端返回的message_id
        if (sessionId && backendMessageId) {
          sessionRenderManager.finishStreaming(sessionId, backendMessageId);
          
          // ✅ 标记流式消息为完成状态
          setMessages(prev => prev.map(msg => 
            msg.id === backendMessageId 
              ? { 
                  ...msg, 
                  streaming_complete: true,
                  // 保留流式标识，但标记为完成状态
                  is_streaming: true
                }
              : msg
          ));
          
          // ✅ HTML提取将由StreamingMessageDisplay的onComplete回调处理
          console.log(`✅ 流式渲染完成: sessionId: ${sessionId}, messageId: ${backendMessageId}`);
        }
        
        // 如果当前会话的标题是默认标题，则更新为第一条消息
        const currentSessionData = sessions.find(s => s.id === sessionId)
        if (currentSessionData && (currentSessionData.title === '新对话' || currentSessionData.title.startsWith('新对话'))) {
          const firstLine = inputMessage.split('\n')[0]
          const newTitle = firstLine.length > 30 ? firstLine.substring(0, 30) + '...' : firstLine
          
          // 更新后端标题
          try {
            await fetch(`http://localhost:8443/api/chat/session/${sessionId}`, {
              method: 'PUT',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ title: newTitle })
            })
            // 重新加载会话列表
            await loadSessions()
          } catch (error) {
            console.error('更新会话标题失败:', error)
          }
        }
      }
    } catch (error) {
      console.error('发送消息失败:', error)
      
      // ✅ 优化错误处理和用户体验
      let errorMessage = '❌ 连接错误: 未知错误'
      let errorType = 'unknown'
      
      if (error.name === 'AbortError') {
        console.log('📡 SSE连接超时，可能是服务器处理时间过长')
        errorMessage = '⏰ 连接超时：服务器处理时间较长，请稍后重试或检查网络连接。'
        errorType = 'timeout'
      } else if (error.message?.includes('ERR_INCOMPLETE_CHUNKED_ENCODING')) {
        console.log('📡 SSE编码错误，连接中断')
        errorMessage = '📡 数据传输中断：连接已中断，请重新发送消息。'
        errorType = 'chunked_encoding'
      } else if (error.message?.includes('network error') || error.message?.includes('Failed to fetch')) {
        console.log('📡 网络连接错误')
        errorMessage = '🌐 网络连接错误：请检查网络连接后重试。'
        errorType = 'network'
      } else if (error.message?.includes('500') || error.message?.includes('502') || error.message?.includes('503')) {
        console.log('📡 服务器错误')
        errorMessage = '🔧 服务器暂时不可用：服务器正在处理中，请稍后重试。'
        errorType = 'server'
      } else {
        console.log('📡 其他错误:', error.message)
        errorMessage = `❌ 连接错误: ${error.message || '请检查网络连接'}`
        errorType = 'other'
      }
      
      // ✅ 添加带有错误类型的消息，便于用户理解和处理
      const errorMessageObj: Message = {
        id: 'error-' + Date.now(),
        type: 'bot',
        content: errorMessage,
        timestamp: new Date(),
        session_id: sessionId || undefined
      }
      
      setMessages(prev => [...prev, errorMessageObj])
      
      // ✅ 特定错误类型的用户提示和建议
      if (errorType === 'network' || errorType === 'timeout') {
        // 延迟显示重试建议
        setTimeout(() => {
          const suggestionMessage: Message = {
            id: 'suggestion-' + Date.now(),
            type: 'bot',
            content: '💡 **建议**：\n- 检查网络连接是否稳定\n- 刷新页面后重试\n- 如果问题持续，可能是服务器负载较高，请稍后再试',
            timestamp: new Date(),
            session_id: sessionId || undefined
          }
          setMessages(prev => [...prev, suggestionMessage])
        }, 1000)
      }
    } finally {
      setIsLoading(false)
    }
  }

  // 初始化加载
  useEffect(() => {
    loadSessions()
  }, [])


  return (
    <div className="flex h-full w-full overflow-hidden p-4 gap-4 bg-gray-100">
        {/* Sidebar */}
        <div className="w-96 min-w-[384px] bg-sidebar-bg rounded-2xl shadow-sm flex flex-col p-6">
          {/* Header */}
          <div className="flex items-center justify-center mb-8">
            <h1 className="text-xl font-semibold text-text-primary tracking-widest">Glata Agent</h1>
          </div>
          
          {/* New Chat Button */}
          <button 
            onClick={createNewSession}
            className="w-full bg-blue-primary text-white py-2 px-3 rounded-lg mb-6 flex items-center justify-center gap-2 hover:bg-indigo-600 transition-colors text-sm"
          >
            <i className="fas fa-plus text-xs"></i>
            New chat
          </button>
          
          {/* Conversations Section */}
          <div className="mb-6 flex-1">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-medium text-text-secondary">Your conversations</h3>
              <button 
                onClick={clearAllSessions}
                className="text-blue-primary text-sm hover:text-blue-600 transition-all duration-200 bg-white border-2 border-transparent hover:border-blue-200 rounded-md px-3 py-1.5 focus:outline-none focus:border-blue-primary active:border-blue-primary"
              >
                Clear All
              </button>
            </div>
            
            {/* Conversation List */}
            <div className="bg-gray-50 rounded-lg p-2 space-y-1 flex-1 overflow-y-auto">
              {sessions.map((session) => (
                <div 
                  key={session.id}
                  onClick={() => switchSession(session.id)}
                  className={`p-3 rounded-lg cursor-pointer group relative transition-colors ${
                    session.id === currentSession 
                      ? 'bg-blue-selected border border-blue-200' 
                      : 'hover:bg-gray-100'
                  }`}
                >
                  <div className="flex items-center gap-3">
                      <i className={`fab fa-weixin text-xs ${
                        session.id === currentSession 
                          ? 'text-blue-primary' 
                          : 'text-gray-600'
                      }`}></i>
                    <span className={`text-xs truncate pr-3 ${
                      session.id === currentSession 
                        ? 'text-blue-primary font-medium' 
                        : 'text-gray-700 font-normal'
                    }`}>
                      {session.title}
                    </span>
                  </div>
                  <div className="absolute right-[3px] top-1/2 transform -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center">
                    <button 
                      onClick={(e) => {
                        e.stopPropagation()
                        editSessionTitle(session.id, session.title)
                      }}
                      className={`w-5 h-5 rounded flex items-center justify-center ${
                        session.id === currentSession ? 'hover:bg-blue-200' : 'hover:bg-gray-200'
                      }`}
                    >
                      <i className={`fas fa-edit text-[13px] ${
                        session.id === currentSession ? 'text-blue-600' : 'text-gray-500'
                      }`}></i>
                    </button>
                    <button 
                      onClick={(e) => {
                        e.stopPropagation()
                        deleteSession(session.id)
                      }}
                      className={`w-5 h-5 rounded flex items-center justify-center ml-[-8px] ${
                        session.id === currentSession ? 'hover:bg-blue-200' : 'hover:bg-gray-200'
                      }`}
                    >
                      <i className={`fas fa-trash text-[13px] ${
                        session.id === currentSession ? 'text-blue-600' : 'text-gray-500'
                      }`}></i>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
          
          {/* Bottom Section */}
          <div className="mt-auto pt-6 border-t border-border-light">
            <div className="space-y-2">
              <div className="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-50 cursor-pointer">
                <div className="w-8 h-8 flex items-center justify-center">
                  <i className="fas fa-cog text-text-secondary"></i>
                </div>
                <span className="text-sm text-text-primary">Settings</span>
              </div>
              
              <div className="flex items-center gap-3 p-3">
                <div className="w-8 h-8 bg-blue-primary rounded-full flex items-center justify-center text-white text-sm font-medium">
                  A
                </div>
                <span className="text-sm text-text-primary">Andrew Nelson</span>
              </div>
            </div>
          </div>
        </div>
        
        {/* Main Chat Area */}
        <div className="flex-1 rounded-2xl shadow-sm flex flex-col relative bg-white">
          {/* Chat Messages */}
          <div className="flex-1 overflow-y-auto p-6 space-y-6 pb-40">
              {messages.map((message) => (
              <div key={message.id} className={`flex ${message.type === 'user' ? 'justify-end' : 'justify-start'}`}>
                {message.type === 'user' ? (
                  <div className="max-w-3xl">
                    <div className="flex items-start gap-3">
                      <div className="bg-blue-primary text-white p-4 rounded-2xl rounded-tr-md">
                        <p className="text-sm">{message.content}</p>
                      </div>
                      <div className="w-8 h-8 bg-gray-300 rounded-full flex items-center justify-center flex-shrink-0">
                        <i className="fas fa-user text-gray-600 text-sm"></i>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="max-w-4xl">
                    <div className="flex items-start gap-3">
                      <div className="w-8 h-8 bg-blue-primary rounded-full flex items-center justify-center flex-shrink-0">
                        <i className="fas fa-robot text-white text-sm"></i>
                      </div>
                      <div className="bg-gray-50 p-6 rounded-2xl rounded-tl-md shadow-sm border border-border-light max-w-4xl">
                        <div className="text-sm text-text-primary markdown-content">
                          {/* 根据消息类型选择渲染方式 */}
                          {message.is_streaming ? (
                            /* 流式消息使用 StreamingMessageDisplay */
                            <StreamingMessageDisplay
                              messageId={message.id}
                              isStreaming={isLoading && message.id.includes('temp')}
                              initialContent={message.content}
                              ref={(ref) => {
                                if (ref && message.id) {
                                  // 🔑 关键修复：确保ref始终使用最新的消息ID
                                  console.log(`🔗 注册StreamingMessageDisplay ref: ${message.id}, ref存在: ${!!ref}`)
                                  
                                  // 调试：显示当前所有ref映射
                                  const currentKeys = Array.from(streamingRefs.current.keys())
                                  console.log(`📋 当前ref映射keys: [${currentKeys.join(', ')}]`)
                                  
                                  // 如果是从临时ID更新过来的，需要先清理旧的映射
                                  if (message.id.startsWith('temp-')) {
                                    // 临时消息，直接设置映射
                                    streamingRefs.current.set(message.id, ref)
                                    console.log(`🆔 设置临时ID映射: ${message.id}`)
                                  } else {
                                    // 真实消息ID，确保映射正确
                                    const existingKeys = Array.from(streamingRefs.current.keys())
                                    const tempKeys = existingKeys.filter(key => key.startsWith('temp-'))
                                    
                                    // 清理所有临时key映射，只保留真实ID映射
                                    tempKeys.forEach(tempKey => {
                                      if (streamingRefs.current.has(tempKey)) {
                                        console.log(`🧹 清理临时ref映射: ${tempKey}`)
                                        streamingRefs.current.delete(tempKey)
                                      }
                                    })
                                    
                                    streamingRefs.current.set(message.id, ref)
                                    console.log(`✅ 设置真实ID的ref映射: ${message.id}`)
                                  }
                                  
                                  // 调试：显示更新后的ref映射
                                  const updatedKeys = Array.from(streamingRefs.current.keys())
                                  console.log(`📋 更新后ref映射keys: [${updatedKeys.join(', ')}]`)
                                } else {
                                  console.warn(`⚠️ ref回调执行但参数无效: ref=${!!ref}, messageId=${message.id}`)
                                }
                              }}
                              onComplete={() => {
                                console.log(`✅ 流式消息渲染完成: ${message.id}`)
                                // 更新消息状态
                                setMessages(prev => prev.map(msg => 
                                  msg.id === message.id 
                                    ? { ...msg, streaming_complete: true }
                                    : msg
                                ))
                                
                                // 如果需要，可以在这里触发HTML提取
                                if (message.session_id && !isLoading) {
                                  setTimeout(() => {
                                    extractAndSaveHTML(message.id, message.session_id!)
                                  }, 500)
                                }
                              }}
                              onChunkAdded={(chunk) => {
                                console.log(`📋 流式块添加: ${message.id}, 块大小: ${chunk.length}`)
                                // 可以在这里添加额外的处理逻辑
                              }}
                            />
                          ) : message.is_rendered && message.html_content ? (
                            /* 已渲染的静态消息使用缓存的HTML */
                            <div data-message-id={message.id} dangerouslySetInnerHTML={{ __html: message.html_content }} />
                          ) : (
                            /* 未渲染的静态消息使用 MessageDisplay */
                            <div 
                              data-message-id={message.id}
                              ref={(el) => {
                                if (el && message.id) {
                                  markdownRefs.current.set(message.id, el)
                                  // 只为静态消息提取HTML
                                  if (message.session_id && !isLoading && message.content && 
                                      !message.id.startsWith('error-') && !message.id.startsWith('temp-') &&
                                      !message.is_streaming) {
                                    setTimeout(() => {
                                      console.log(`📋 静态消息ref回调触发HTML提取: 消息${message.id}, 会话${message.session_id}`)
                                      extractAndSaveHTML(message.id, message.session_id!)
                                    }, 100)
                                  }
                                }
                              }}
                            >
                              {message.content && (
                                <>
                                  {console.log(`🎨 渲染静态MessageDisplay: 消息${message.id}, 内容长度${message.content.length}, 前50字符: "${message.content.substring(0, 50)}..."`)}
                                  <MessageDisplay 
                                    content={message.content} 
                                    isStreaming={false}
                                  />
                                </>
                              )}
                              
                              {/* 空状态处理 */}
                              {!message.content && (
                                <div className="text-gray-500 text-sm italic">
                                  正在生成回复...
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>
          
          {/* Input Area */}
          <div className="absolute bottom-0 left-0 w-full p-6 z-10">
          <div className="max-w-6xl mx-auto flex items-center gap-4 rounded-full pl-6 pr-2 py-3" style={{ 
            width: '800px',
            height: '50px',
            background: 'rgba(255, 255, 255, 0.8)', 
            backdropFilter: 'blur(20px)',
            WebkitBackdropFilter: 'blur(20px)',
            border: '1px solid rgba(255, 255, 255, 0.3)',
            boxShadow: '0 8px 32px rgba(0, 0, 0, 0.1)'
          }}>
            {/* Brain emoji icon with pink bottom shadow */}
            <div className="text-2xl relative">
              <span>🧠</span>
              <div 
                className="absolute top-full left-1/2 transform -translate-x-1/2"
                style={{
                  width: '44px',
                  height: '18px',
                  background: 'radial-gradient(ellipse, rgba(141, 102, 106, 0.6) 0%, rgba(255, 192, 203, 0.4) 40%, transparent 70%)',
                  filter: 'blur(3px)',
                  marginTop: '-12px'
                }}
              />
            </div>
            
            {/* Input field */}
            <div className="flex-1">
              <textarea
                value={inputMessage}
                onChange={(e) => setInputMessage(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    sendMessage();
                  }
                }}
                placeholder="What's in your mind?..."
                className="w-full px-0 py-1 border-0 focus:outline-none text-base text-gray-800 placeholder-gray-400 bg-transparent resize-none"
                disabled={isLoading}
                rows={1}
              />
            </div>
            
            {/* Send button */}
            <button
              onClick={sendMessage}
              disabled={isLoading || !inputMessage.trim()}
              className="w-10 h-10 bg-blue-500 text-white rounded-full flex items-center justify-center hover:bg-blue-600 transition-colors shadow-lg disabled:cursor-not-allowed flex-shrink-0"
            >
              {isLoading ? (
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
              ) : (
                                <i className="fas fa-paper-plane text-white"></i>
              )}
            </button>
          </div>
        </div>
        </div>
    </div>
  )
}

export default App