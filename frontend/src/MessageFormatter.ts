/**
 * 消息格式化工具
 * 用于清理和标准化聊天消息的显示格式
 */

export class MessageFormatter {
  /**
   * 清理消息内容，移除不必要的格式标记
   */
  static cleanContent(content: string): string {
    if (!content) return '';
    
    // 移除常见的格式混乱
    let cleaned = content;
    
    // 移除多余的空白行
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n');
    
    // 移除行尾的空白字符
    cleaned = cleaned.replace(/[ \t]+$/gm, '');
    
    // 修复代码块格式
    cleaned = cleaned.replace(/```\s*\n\s*```/g, '');
    
    // 修复列表格式
    cleaned = cleaned.replace(/^\s*[\*\-]\s*$/gm, '');
    
    return cleaned.trim();
  }

  /**
   * 分离进度信息和主要内容
   */
  static separateProgress(content: string): {
    progress: string[];
    mainContent: string;
  } {
    const lines = content.split('\n');
    const progress: string[] = [];
    const mainContent: string[] = [];
    
    let inProgressSection = false;
    
    for (const line of lines) {
      const trimmed = line.trim();
      
      // 识别进度信息
      if (trimmed.includes('🔄') || 
          trimmed.includes('⏳') || 
          trimmed.includes('✅') ||
          trimmed.includes('📊') ||
          trimmed.match(/\d+\/\d+ 完成/) ||
          trimmed.match(/进度: \d+%/) ||
          trimmed.includes('正在处理') ||
          trimmed.includes('加载中')) {
        progress.push(line);
        inProgressSection = true;
      } else if (inProgressSection && trimmed === '') {
        // 空行可能分隔进度和主要内容
        inProgressSection = false;
      } else if (!inProgressSection) {
        mainContent.push(line);
      }
    }
    
    return {
      progress: progress,
      mainContent: mainContent.join('\n').trim()
    };
  }

  /**
   * 格式化代码块
   */
  static formatCodeBlocks(content: string): string {
    if (!content) return '';
    
    // 确保代码块有正确的语言标识
    return content.replace(/```(\w*)\n/g, (_match, lang) => {
      const language = lang || 'go';
      return `\`\`\`${language}\n`;
    });
  }

  /**
   * 检测是否为最终内容（而非进度信息）
   */
  static isFinalContent(content: string): boolean {
    if (!content) return false;
    
    // 如果内容太短，可能是中间状态
    if (content.trim().length < 50) return false;
    
    // 如果包含完整的代码块，可能是最终内容
    const codeBlockMatches = content.match(/```/g);
    if (codeBlockMatches && codeBlockMatches.length >= 2) return true;
    
    // 如果以总结性语句结尾，可能是最终内容
    const summaryPatterns = [
      /总结[:：]/,
      /以上就是/,
      /希望这能帮助你/,
      /如有问题请随时/,
      /\d+\.\s*$/m
    ];
    
    return summaryPatterns.some(pattern => pattern.test(content));
  }

  /**
   * 创建优化的消息对象
   */
  static createOptimizedMessage(
    originalContent: string,
    _messageId: string,
    isStreaming: boolean = false
  ): {
    displayContent: string;
    progressMessages: string[];
    isFinal: boolean;
  } {
    const cleaned = this.cleanContent(originalContent);
    const separated = this.separateProgress(cleaned);
    const formatted = this.formatCodeBlocks(separated.mainContent);
    const isFinal = this.isFinalContent(formatted) && !isStreaming;
    
    return {
      displayContent: formatted,
      progressMessages: separated.progress,
      isFinal
    };
  }
}

/**
 * 消息状态管理器
 * 用于管理流式消息的不同显示状态
 */
export class MessageStateManager {
  private messageStates = new Map<string, {
    originalContent: string;
    displayContent: string;
    progressMessages: string[];
    isFinal: boolean;
  }>();

  /**
   * 更新消息状态
   */
  updateMessage(
    messageId: string,
    content: string,
    isStreaming: boolean = false
  ): {
    displayContent: string;
    progressMessages: string[];
    isFinal: boolean;
  } {
    const optimized = MessageFormatter.createOptimizedMessage(
      content,
      messageId,
      isStreaming
    );
    
    this.messageStates.set(messageId, {
      originalContent: content,
      displayContent: optimized.displayContent,
      progressMessages: optimized.progressMessages,
      isFinal: optimized.isFinal
    });
    
    return optimized;
  }

  /**
   * 获取消息状态
   */
  getMessageState(messageId: string) {
    return this.messageStates.get(messageId);
  }

  /**
   * 清除消息状态
   */
  clearMessage(messageId: string) {
    this.messageStates.delete(messageId);
  }

  /**
   * 清除所有消息状态
   */
  clearAll() {
    this.messageStates.clear();
  }
}

// 全局实例
export const messageStateManager = new MessageStateManager();