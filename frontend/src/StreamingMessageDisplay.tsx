import { useRef, useImperativeHandle, forwardRef, useEffect, useState } from 'react'
import { MarkdownCMD } from 'ds-markdown'
import 'ds-markdown/style.css'

// 定义 MarkdownCMD 的 ref 类型
interface MarkdownCMDRef {
  push: (content: string, answerType: 'thinking' | 'answer') => void
  clear: () => void
  start: () => void
  stop: () => void
  resume: () => void
  restart: () => void
  triggerWholeEnd: () => void
}

interface StreamingMessageDisplayProps {
  messageId: string
  isStreaming: boolean
  initialContent?: string
  onComplete?: () => void
  onChunkAdded?: (chunk: string) => void
}

export interface StreamingMessageDisplayRef {
  pushChunk: (chunk: string) => void
  clear: () => void
  start: () => void
  stop: () => void
  restart: () => void
  getContent: () => string
}

/**
 * 流式消息显示组件
 * 使用 MarkdownCMD 和命令式 API 进行高性能流式渲染
 */
export const StreamingMessageDisplay = forwardRef<StreamingMessageDisplayRef, StreamingMessageDisplayProps>(
  ({ messageId, isStreaming, initialContent, onComplete, onChunkAdded }, ref) => {
    const markdownRef = useRef<MarkdownCMDRef>(null)
    const [totalContent, setTotalContent] = useState('')

    // 暴露给父组件的方法
    useImperativeHandle(ref, () => ({
      pushChunk: (chunk: string) => {
        console.log(`📝 StreamingMessageDisplay.pushChunk: 消息${messageId}, 内容块: "${chunk.substring(0, 50)}..."`)
        
        // 使用 MarkdownCMD 的 push 方法增量添加内容
        markdownRef.current?.push(chunk, 'answer')
        
        // 更新总内容记录
        setTotalContent(prev => prev + chunk)
        
        // 触发回调
        onChunkAdded?.(chunk)
      },
      
      clear: () => {
        console.log(`🔄 StreamingMessageDisplay.clear: 消息${messageId}`)
        markdownRef.current?.clear()
        setTotalContent('')
      },
      
      start: () => {
        console.log(`▶️ StreamingMessageDisplay.start: 消息${messageId}`)
        markdownRef.current?.start()
      },
      
      stop: () => {
        console.log(`⏸️ StreamingMessageDisplay.stop: 消息${messageId}`)
        markdownRef.current?.stop()
      },
      
      restart: () => {
        console.log(`🔄 StreamingMessageDisplay.restart: 消息${messageId}`)
        markdownRef.current?.restart()
      },
      
      getContent: () => {
        return totalContent
      }
    }), [messageId, totalContent, onChunkAdded])

    // 初始化内容
    useEffect(() => {
      if (initialContent && markdownRef.current) {
        console.log(`🎯 StreamingMessageDisplay 初始化内容: 消息${messageId}, 长度${initialContent.length}`)
        markdownRef.current.clear()
        markdownRef.current.push(initialContent, 'answer')
        setTotalContent(initialContent)
      }
    }, [initialContent, messageId])

    // 处理打字完成事件
    const handleEnd = (data: any) => {
      console.log(`✅ StreamingMessageDisplay 打字完成: 消息${messageId}`, data)
      onComplete?.()
    }

    // 处理打字开始事件
    const handleStart = (data: any) => {
      console.log(`🚀 StreamingMessageDisplay 开始打字: 消息${messageId}`, data)
    }

    return (
      <div className="streaming-message-display" data-message-id={messageId}>
        <MarkdownCMD
          ref={markdownRef}
          timerType="requestAnimationFrame"  // 高性能模式
          interval={5}                      // 最佳体验间隔 (15-30ms)
          autoStartTyping={true}             // 自动开始打字
          onStart={handleStart}              // 开始回调
          onEnd={handleEnd}                  // 完成回调
          onTypedChar={(data) => {
            // 可选：添加打字进度追踪
            console.log(`⌨️ 打字进度: ${Math.round(data.percent)}% - "${data.currentChar}"`)
          }}
        />
      </div>
    )
  }
)

StreamingMessageDisplay.displayName = 'StreamingMessageDisplay'

export default StreamingMessageDisplay