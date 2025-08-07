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
        // 推送内容块
        
        // 使用 MarkdownCMD 的 push 方法增量添加内容
        markdownRef.current?.push(chunk, 'answer')
        
        // 更新总内容记录
        setTotalContent(prev => prev + chunk)
        
        // 触发回调
        onChunkAdded?.(chunk)
      },
      
      clear: () => {
        // 清空内容
        markdownRef.current?.clear()
        setTotalContent('')
      },
      
      start: () => {
        // 开始渲染
        markdownRef.current?.start()
      },
      
      stop: () => {
        // 暂停渲染
        markdownRef.current?.stop()
      },
      
      restart: () => {
        // 重新开始渲染
        markdownRef.current?.restart()
      },
      
      getContent: () => {
        return totalContent
      }
    }), [messageId, totalContent, onChunkAdded])

    // 初始化内容
    useEffect(() => {
      if (initialContent && markdownRef.current) {
        // 初始化内容
        markdownRef.current.clear()
        markdownRef.current.push(initialContent, 'answer')
        setTotalContent(initialContent)
      }
    }, [initialContent, messageId])

    // 处理打字完成事件
    const handleEnd = (data: any) => {
      // 打字完成
      onComplete?.()
    }

    // 处理打字开始事件
    const handleStart = (data: any) => {
      // 打字开始
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
          onTypedChar={() => {
            // 可选：添加打字进度追踪
          }}
        />
      </div>
    )
  }
)

StreamingMessageDisplay.displayName = 'StreamingMessageDisplay'

export default StreamingMessageDisplay