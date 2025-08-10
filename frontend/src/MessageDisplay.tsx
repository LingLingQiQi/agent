import { useState, useEffect, useRef } from 'react'
import Markdown from 'ds-markdown'
import 'ds-markdown/style.css'

interface MessageDisplayProps {
  content: string
  isStreaming: boolean
  onHTMLExtracted?: (messageId: string, html: string) => void
  messageId?: string
}

/**
 * 消息格式化工具
 * 用于清理和标准化聊天消息的显示格式
 */
class MessageFormatter {
  static cleanContent(content: string): string {
    if (!content) return '';
    
    let cleaned = content;
    // ⚡️ 修复：保护代码块内的换行符，只清理非代码块区域的过多空行
    // 先标记代码块
    const codeBlocks: string[] = [];
    let codeBlockIndex = 0;
    
    // 提取并保护代码块
    cleaned = cleaned.replace(/```[\s\S]*?```/g, (match) => {
      const placeholder = `__CODE_BLOCK_${codeBlockIndex}__`;
      codeBlocks[codeBlockIndex] = match;
      codeBlockIndex++;
      return placeholder;
    });
    
    // 只在非代码块区域清理格式
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n'); // 限制连续空行
    cleaned = cleaned.replace(/[ \t]+$/gm, ''); // 移除行尾空格
    cleaned = cleaned.replace(/```\s*\n\s*```/g, ''); // 移除空代码块
    
    // 恢复代码块
    codeBlocks.forEach((codeBlock, index) => {
      cleaned = cleaned.replace(`__CODE_BLOCK_${index}__`, codeBlock);
    });
    
    return cleaned.trim();
  }

  static separateProgress(content: string): {
    progress: string[];
    mainContent: string;
  } {
    const lines = content.split('\n');
    const progress: string[] = [];
    const mainContent: string[] = [];
    
    for (const line of lines) {
      const trimmed = line.trim();
      
      if (trimmed.includes('🔄') || 
          trimmed.includes('⏳') || 
          trimmed.includes('✅') ||
          trimmed.includes('📊') ||
          trimmed.match(/\d+\/\d+ 完成/) ||
          trimmed.match(/进度: \d+%/) ||
          trimmed.includes('正在处理') ||
          trimmed.includes('加载中')) {
        progress.push(line);
      } else {
        mainContent.push(line);
      }
    }
    
    return {
      progress: progress,
      mainContent: mainContent.join('\n').trim()
    };
  }

  static formatCodeBlocks(content: string): string {
    if (!content) return '';
    return content.replace(/```(\w*)\n/g, (_match, lang) => {
      const language = lang || 'go';
      return `\`\`\`${language}\n`;
    });
  }

  static isFinalContent(content: string): boolean {
    if (!content) return false;
    if (content.trim().length < 50) return false;
    
    const codeBlockMatches = content.match(/```/g);
    if (codeBlockMatches && codeBlockMatches.length >= 2) return true;
    
    const summaryPatterns = [
      /总结[:：]/,
      /以上就是/,
      /希望这能帮助你/,
      /如有问题请随时/
    ];
    
    return summaryPatterns.some(pattern => pattern.test(content));
  }

  // 新增：应用任务状态样式
  static applyTaskStyles(element: HTMLElement) {
    // 添加DOM元素存在性检查
    if (!element || typeof element.querySelectorAll !== 'function') {
      console.warn('⚠️ applyTaskStyles: 无效的DOM元素', element);
      return;
    }
    
    const listItems = element.querySelectorAll('li');
    
    listItems.forEach((li) => {
      const text = li.textContent || '';
      
      // 检测失败任务 [!]
      if (text.includes('[!]')) {
        li.classList.add('task-failed');
        li.style.color = '#dc2626';
        li.style.textDecoration = 'line-through';
        li.style.opacity = '0.8';
      }
      // 检测完成任务 [x]
      else if (text.includes('[x]')) {
        li.classList.add('task-completed');
        li.style.color = '#16a34a';
      }
      // 检测待执行任务 [ ]
      else if (text.includes('[ ]')) {
        li.classList.add('task-pending');
        li.style.color = '#1a1a1a';
      }
    });
  }
}

export const MessageDisplay = ({ content, isStreaming, onHTMLExtracted, messageId }: MessageDisplayProps) => {
  const [displayContent, setDisplayContent] = useState('');
  const markdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!content) return;
    setDisplayContent(content);
  }, [content, isStreaming]);

  // 在内容渲染后应用任务状态样式
  useEffect(() => {
    if (markdownRef.current && displayContent) {
      const timer = setTimeout(() => {
        if (markdownRef.current) {
          MessageFormatter.applyTaskStyles(markdownRef.current);
        }
      }, 100);
      
      return () => clearTimeout(timer);
    }
  }, [displayContent, isStreaming]);

  if (!content.trim()) {
    return null;
  }

  return (
    <div className="markdown-display" ref={markdownRef}>
      {displayContent && (
        <div>
          <Markdown 
            interval={5}
            answerType="answer"
            timerType="requestAnimationFrame"
            theme="light"
            onEnd={() => {
              // 🔑 渲染完成回调，提取HTML并保存
              console.log(`📝 MessageDisplay渲染完成: ${messageId}`);
              
              if (markdownRef.current && messageId && onHTMLExtracted) {
                // 延迟提取，确保DOM完全更新
                setTimeout(() => {
                  if (markdownRef.current) {
                    const htmlContent = markdownRef.current.innerHTML;
                    console.log(`🎯 提取HTML内容长度: ${htmlContent.length}`);
                    onHTMLExtracted(messageId, htmlContent);
                  }
                }, 200);
              }
            }}
          >
            {displayContent}
          </Markdown>
        </div>
      )}
    </div>
  );
};

export default MessageDisplay;