import { useState, useEffect, useRef } from 'react'
import { Markdown } from 'ds-markdown'
import 'ds-markdown/style.css'
import './App.css'
import { sessionRenderManager } from './SessionIsolatedRenderManager'

interface Message {
  id: string
  type: 'user' | 'bot'
  content: string
  timestamp: Date
  session_id?: string  // ✅ 约束2：添加会话ID字段
  html_content?: string  // ✅ 渲染后的HTML内容
  is_rendered?: boolean  // ✅ 是否已渲染
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

  // 提取ds-markdown渲染的HTML并保存到后端
  const extractAndSaveHTML = async (messageId: string, sessionId: string) => {
    // ✅ 防止在错误的会话上下文中执行渲染保存
    if (sessionId !== currentSession) {
      console.warn(`⚠️ 会话上下文不匹配: 消息${messageId}属于会话${sessionId}, 当前会话${currentSession}, 跳过HTML提取`)
      return
    }
    
    // 防止重复提取
    const message = messages.find(msg => msg.id === messageId)
    if (message && message.is_rendered) {
      console.log(`⏭️ 消息 ${messageId} 已渲染过，跳过HTML提取`)
      return
    }
    
    // ✅ 二次验证：确保消息确实属于当前会话
    if (message && message.session_id !== currentSession) {
      console.warn(`⚠️ 消息会话ID不匹配: 消息${messageId}属于会话${message.session_id}, 当前会话${currentSession}, 跳过HTML提取`)
      return
    }
    
    // ✅ 验证消息内容是否完整（避免提取不完整的内容）
    if (message && (!message.content || message.content.trim().length < 10)) {
      console.warn(`⚠️ 消息内容不完整: 消息${messageId}长度${message.content?.length || 0}，跳过HTML提取`)
      return
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
          if (parent && parent.textContent?.includes(message.content?.substring(0, 50) || '')) {
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
        
        // ✅ 确保HTML内容完整，避免提取截断的内容
        const isComplete = htmlContent.includes('ds-markdown-answer') || htmlContent.includes('</div>') || htmlContent.includes('<pre') || htmlContent.includes('<code');
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
              if (currentSession === sessionId) {
                console.log(`🔄 重试提取HTML: 消息 ${messageId}`)
                extractAndSaveHTML(messageId, sessionId)
              }
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
        if (currentSession === sessionId) {
          console.log(`🔄 重试查找DOM元素: 消息 ${messageId}`)
          extractAndSaveHTML(messageId, sessionId)
        }
      }, 500)
    }
  }

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  // 监听消息变化，对新消息提取HTML
  useEffect(() => {
    const latestBotMessage = messages
      .filter(msg => msg.type === 'bot' && !msg.is_rendered && msg.session_id === currentSession)
      .slice(-1)[0]
    
    if (latestBotMessage && latestBotMessage.session_id && !isLoading && currentSession) {
      // 延迟提取，确保ds-markdown渲染完成
      const timeoutId = setTimeout(() => {
        // ✅ 最终验证：确保当前会话没有变化
        if (latestBotMessage.session_id === currentSession) {
          extractAndSaveHTML(latestBotMessage.id, latestBotMessage.session_id!)
        }
      }, 800) // 增加延迟确保渲染完成
      
      return () => clearTimeout(timeoutId)
    }
  }, [messages, isLoading, currentSession])

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
        const convertedMessages = (data.messages || []).map((msg: any) => {
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
        }).filter(Boolean); // 过滤掉null值
        
        // ✅ 约束1：使用样式保护更新消息
        setMessages(convertedMessages);
        
        // 延迟处理未渲染的历史消息
        setTimeout(() => {
          const unrenderedMessages = convertedMessages.filter(
            msg => msg.type === 'bot' && !msg.is_rendered && msg.content && msg.content.length > 10
          );
          
          unrenderedMessages.forEach(msg => {
            console.log(`🔄 处理未渲染的历史消息: ${msg.id}`);
            extractAndSaveHTML(msg.id, msg.session_id!);
          });
        }, 1000);
        
        // 统计渲染优化情况
        const renderedCount = convertedMessages.filter(msg => msg.is_rendered && msg.html_content).length;
        const totalAssistantMessages = convertedMessages.filter(msg => msg.type === 'bot').length;
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
          setCurrentSession(sessionId)
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

    // ✅ 约束3：确保会话管理器知道当前活跃会话，然后开始流式输出
    if (sessionId) {
      // 如果当前没有会话或者会话不匹配，先设置活跃会话
      if (currentSession !== sessionId) {
        await sessionRenderManager.switchSession(sessionId);
        setCurrentSession(sessionId);
      }
      
      sessionRenderManager.startStreamingForSession(sessionId);
    }

    try {
      const response = await fetch('http://localhost:8443/api/chat/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          message: inputMessage,
          session_id: sessionId,
          background_mode: sessionId !== currentSession  // ✅ 约束3：后台模式标识
        })
      })

      if (response.ok && response.body) {
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let botMessage = ''
        let backendMessageId: string | null = null
        
        // ✅ 只在当前活跃会话创建临时消息，避免空白消息
        let tempMessageObj: Message | null = null;
        if (sessionId === currentSession) {
          tempMessageObj = {
            id: 'temp-' + Date.now(),
            type: 'bot',
            content: '',
            timestamp: new Date(),
            session_id: sessionId || undefined  // ✅ 约束2：添加会话ID
          };
          setMessages(prev => [...prev, tempMessageObj]);
        } else {
          console.log(`⏭️ 会话${sessionId}不在前台，不创建临时消息UI`);
        }

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
                if (parsed.content && parsed.message_id) {
                  // ✅ 第一次收到数据时，获取后端返回的真实message_id
                  if (!backendMessageId) {
                    backendMessageId = parsed.message_id;
                    console.log(`📝 获取后端消息ID: ${backendMessageId}`);
                  }
                  
                  botMessage += parsed.content
                  
                  // ✅ 约束3：使用后端返回的message_id处理流式内容块
                  if (sessionId && backendMessageId) {
                    sessionRenderManager.handleStreamChunk(sessionId, backendMessageId, parsed.content);
                  }
                  
                  // ✅ 修复：放宽会话验证，允许当前活跃会话的消息渲染
                  setMessages(prev => {
                    const newMessages = [...prev];
                    
                    // 1. 尝试更新临时消息
                    if (tempMessageObj) {
                      const tempIndex = newMessages.findIndex(msg => msg.id === tempMessageObj.id);
                      if (tempIndex !== -1) {
                        newMessages[tempIndex] = {
                          ...newMessages[tempIndex],
                          id: backendMessageId,
                          content: botMessage,
                          session_id: sessionId || undefined
                        };
                        return newMessages;
                      }
                    }
                    
                    // 2. 尝试更新已存在的消息
                    const existingIndex = newMessages.findIndex(msg => msg.id === backendMessageId);
                    if (existingIndex !== -1) {
                      newMessages[existingIndex] = {
                        ...newMessages[existingIndex],
                        content: botMessage,
                        session_id: sessionId || undefined
                      };
                    } else {
                      // 3. 创建新消息（确保会话ID匹配）
                      newMessages.push({
                        id: backendMessageId,
                        type: 'bot',
                        content: botMessage,
                        timestamp: new Date(),
                        session_id: sessionId || undefined
                      });
                    }
                    
                    return newMessages;
                  });
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
          
          // ✅ 修复：确保当前活跃会话的消息完成HTML提取
          const extractHtmlWithRetry = async (retryCount = 0) => {
            // 查找DOM元素 - 使用更精确的选择器
            let markdownElement = markdownRefs.current.get(backendMessageId);
            if (!markdownElement) {
              markdownElement = document.querySelector(`[data-message-id="${backendMessageId}"]`) as HTMLDivElement;
            }
            
            if (markdownElement) {
              console.log(`✅ 找到DOM元素，开始提取HTML: ${backendMessageId}`);
              await extractAndSaveHTML(backendMessageId, sessionId);
            } else if (retryCount < 3) {
              console.log(`🔄 DOM元素未找到，${(retryCount + 1) * 500}ms后重试: ${backendMessageId}`);
              setTimeout(() => extractHtmlWithRetry(retryCount + 1), (retryCount + 1) * 500);
            } else {
              console.warn(`⚠️ 重试3次后仍未找到DOM元素: ${backendMessageId}`);
            }
          };
          
          // 确保当前会话的消息完成HTML提取
          setTimeout(() => {
            extractHtmlWithRetry();
          }, 800);
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
                className="text-blue-primary text-sm hover:text-blue-600 transition-colors bg-white"
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
                          {message.is_rendered && message.html_content ? (
                            // 使用已渲染的HTML内容
                            <div data-message-id={message.id} dangerouslySetInnerHTML={{ __html: message.html_content }} />
                          ) : (
                            // 实时渲染Markdown
                            <div 
                              data-message-id={message.id}
                              ref={(el) => {
                                if (el && message.id) {
                                  markdownRefs.current.set(message.id, el)
                                  // 对于已经完成的消息（非流式），立即提取HTML
                                  if (message.session_id && !isLoading && message.session_id === currentSession) {
                                    setTimeout(() => {
                                      // ✅ 再次验证会话上下文
                                      if (message.session_id === currentSession) {
                                        extractAndSaveHTML(message.id, message.session_id!)
                                      }
                                    }, 100) // 等待ds-markdown完成渲染
                                  }
                                }
                              }}
                            >
                              <Markdown 
                                interval={0}
                                answerType="answer"
                                theme="light"
                              >
                                {message.content}
                              </Markdown>
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