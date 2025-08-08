import { useRef, useImperativeHandle, forwardRef, useEffect, useState, useCallback, memo } from 'react'
import { MarkdownCMD } from 'ds-markdown'
import debounce from 'lodash.debounce'
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
 * 流式消息显示组件 - 双重状态管理优化版
 * 使用 MarkdownCMD 和命令式 API 进行高性能流式渲染
 * 通过防抖和缓冲区优化减少重渲染，同时保护HTML持久化机制
 */
export const StreamingMessageDisplay = forwardRef<StreamingMessageDisplayRef, StreamingMessageDisplayProps>(
  ({ messageId, initialContent, onComplete, onChunkAdded }, ref) => {
    const markdownRef = useRef<MarkdownCMDRef>(null)
    
    // ✅ 保持现有状态，确保getContent()和HTML持久化正常工作
    const [totalContent, setTotalContent] = useState('')
    
    // 🔑 新增：缓冲区和性能优化状态  
    const contentBufferRef = useRef<string>('')
    const [renderCount, setRenderCount] = useState(0)
    const isInitializedRef = useRef<boolean>(false)  // 🔑 标记是否已初始化，防止重复初始化

    // 🔑 防抖批量推送到ds-markdown，减少重渲染频率
    const debouncedPush = useCallback(
      debounce(() => {
        if (markdownRef.current && contentBufferRef.current) {
          const bufferContent = contentBufferRef.current
          contentBufferRef.current = '' // 清空缓冲区
          
          // 推送到ds-markdown
          markdownRef.current.push(bufferContent, 'answer')
          setRenderCount(prev => prev + 1)
          
          console.log(`📝 批量推送到ds-markdown: messageId=${messageId}, 长度=${bufferContent.length}`)
        }
      }, 5), // 🔑 优化：减少防抖延迟到15ms，匹配ds-markdown间隔，提高响应速度
      [messageId]
    )

    // 暴露给父组件的方法
    useImperativeHandle(ref, () => ({
      pushChunk: (chunk: string) => {
        if (!chunk || chunk.length === 0) {
          console.warn(`StreamingMessageDisplay: 接收到空块，跳过处理: ${messageId}`)
          return
        }

        // ✅ 保持原有逻辑：立即更新totalContent，确保getContent()返回完整内容
        setTotalContent(prev => {
          const newTotal = prev + chunk
          return newTotal
        })
        
        // 🔑 性能优化：将内容添加到缓冲区，使用防抖批量处理
        contentBufferRef.current += chunk
        
        // 触发防抖批量推送
        debouncedPush()
        
        // 触发回调
        onChunkAdded?.(chunk)
      },
      
      clear: () => {
        // 🔑 清理前先推送所有缓冲区内容
        if (contentBufferRef.current && markdownRef.current) {
          markdownRef.current.push(contentBufferRef.current, 'answer')
        }
        
        // 清空所有状态和缓冲区
        markdownRef.current?.clear()
        contentBufferRef.current = ''
        setTotalContent('')
        setRenderCount(0)
        isInitializedRef.current = false  // 🔑 重置初始化标记
        debouncedPush.cancel() // 取消待执行的防抖调用
      },
      
      start: () => {
        markdownRef.current?.start()
      },
      
      stop: () => {
        markdownRef.current?.stop()
      },
      
      restart: () => {
        markdownRef.current?.restart()
      },
      
      getContent: () => {
        // 🔑 确保返回的内容包含所有缓冲区内容，避免丢失最后一段
        return totalContent + contentBufferRef.current
      }
    }), [messageId, totalContent, onChunkAdded, debouncedPush])

    // 初始化内容处理 - 🔑 只在首次挂载时处理，避免重复初始化
    useEffect(() => {
      if (initialContent && markdownRef.current && !isInitializedRef.current) {
        console.log(`🚀 首次初始化流式消息内容: ${messageId}, 长度: ${initialContent.length}`)
        markdownRef.current.clear()
        markdownRef.current.push(initialContent, 'answer')
        setTotalContent(initialContent)
        isInitializedRef.current = true  // 🔑 标记已初始化，防止重复
      }
    }, [messageId]) // 🔑 只依赖messageId，忽略initialContent变化

    // 处理打字完成事件
    const handleEnd = useCallback(() => {
      console.log(`✅ 流式渲染完成: ${messageId}, 总渲染次数: ${renderCount}, 最终内容长度: ${totalContent.length}`)
      onComplete?.()
    }, [messageId, renderCount, totalContent.length, onComplete])

    // 处理打字开始事件
    const handleStart = useCallback(() => {
      console.log(`🚀 开始流式渲染: ${messageId}`)
    }, [messageId])

    // 🔑 性能优化：使用稳定的props对象，减少MarkdownCMD重渲染
    const markdownProps = {
      timerType: "requestAnimationFrame" as const,
      interval: 5,  // 适中的间隔：平衡流畅度和性能
      autoStartTyping: true,
      onStart: handleStart,
      onEnd: handleEnd,
      onTypedChar: () => {
        // 可选：添加打字进度追踪
      }
    }

    return (
      <div className="streaming-message-display" data-message-id={messageId}>
        <MarkdownCMD
          ref={markdownRef}
          {...markdownProps}
        />
        {/* 开发模式下的调试信息 */}
        {process.env.NODE_ENV === 'development' && (
          <div className="text-xs text-gray-500 mt-1">
            渲染次数: {renderCount} | 内容长度: {totalContent.length}
          </div>
        )}
      </div>
    )
  }
)

// ✅ 简化React.memo - 主要关注messageId，避免同一消息的组件重复挂载
const MemoizedStreamingMessageDisplay = memo(StreamingMessageDisplay, (prevProps, nextProps) => {
  // 🔑 简化比较逻辑：相同messageId的组件不应该重新挂载
  return prevProps.messageId === nextProps.messageId;
});

MemoizedStreamingMessageDisplay.displayName = 'StreamingMessageDisplay'

export default MemoizedStreamingMessageDisplay