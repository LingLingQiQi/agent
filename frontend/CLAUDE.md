# CLAUDE.md

这个文件为 Claude Code (claude.ai/code) 在处理前端代码时提供指导。

## 前端架构概述

这是一个基于 React + TypeScript + Vite 构建的现代化 AI 智能体前端界面。采用组件化设计，支持实时流式消息渲染、会话隔离管理和响应式布局，为用户提供流畅的智能体对话体验。

### 核心特性

- **现代技术栈**: React 19.1.0 + TypeScript 5.8.3 + Vite 5.4.10
- **实时流式渲染**: 基于 ds-markdown 的流式 Markdown 内容渲染
- **会话隔离系统**: 独立的会话渲染管理和样式保护机制
- **响应式设计**: Tailwind CSS 4.1.11 原子化 CSS 框架
- **组件化架构**: 高度模块化的 React 组件设计
- **类型安全**: 完整的 TypeScript 类型定义和检查

## 技术栈详细信息

### 核心框架和库
- **React 19.1.0** - 最新的 React 框架，支持并发特性和自动批处理
- **TypeScript 5.8.3** - 强类型语言，提供完整的类型安全
- **Vite 5.4.10** - 现代前端构建工具，支持热模块替换 (HMR)

### UI 和样式
- **Tailwind CSS 4.1.11** - 原子化 CSS 框架，支持最新的 CSS 特性
- **@tailwindcss/typography 0.5.16** - Markdown 内容的专业排版样式
- **@heroicons/react 2.2.0** - 高质量的 SVG 图标库

### Markdown 渲染
- **ds-markdown 0.1.8** - 专业的流式 Markdown 渲染组件
- 支持实时流式内容渲染
- 内置语法高亮和代码块格式化
- 自动链接识别和处理

### 开发工具
- **ESLint 9.30.1** - 代码质量检查和风格统一
- **@vitejs/plugin-react 4.6.0** - Vite 的 React 插件支持
- **typescript-eslint 8.35.1** - TypeScript 专用的 ESLint 规则

## 开发命令

### 开发环境
```bash
cd frontend
npm install                   # 安装所有依赖包
npm run dev                   # 启动开发服务器 (端口 8080)
```

### 生产构建
```bash
npm run build                 # 构建生产版本到 dist/ 目录
npm run preview               # 预览生产构建结果
```

### 代码质量
```bash
npm run lint                  # 运行 ESLint 代码检查
npm run lint --fix            # 自动修复可修复的 lint 问题
```

## 详细模块说明

### 应用入口 (`src/main.tsx`)
```typescript
// React 18+ StrictMode 包装
// 应用根组件挂载到 DOM
// 启用开发模式的双重渲染检查
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
```

### 主应用组件 (`src/App.tsx`)

**核心状态管理**:
```typescript
// 会话状态
const [currentSession, setCurrentSession] = useState<string | null>(null)
const [sessions, setSessions] = useState<Session[]>([])

// 消息状态
const [messages, setMessages] = useState<Message[]>([])
const [inputMessage, setInputMessage] = useState('')
const [isLoading, setIsLoading] = useState(false)

// 组件引用管理
const messagesEndRef = useRef<HTMLDivElement>(null)
const markdownRefs = useRef<Map<string, HTMLDivElement>>(new Map())
const streamingRefs = useRef<Map<string, StreamingMessageDisplayRef>>(new Map())
```

**主要功能**:
- 会话生命周期管理 (创建、切换、删除)
- 消息发送和接收处理
- SSE 流式数据接收和处理
- HTML 内容提取和后端同步
- 自动滚动到最新消息

### 消息显示组件 (`src/MessageDisplay.tsx`)

**功能特性**:
```typescript
// 消息组件接口
interface MessageDisplayProps {
  message: Message
  onHTMLExtracted?: (messageId: string, html: string) => void
}

// 核心功能
// - 静态消息的 Markdown 渲染
// - HTML 内容提取和回调
// - 消息类型的样式差异化
// - 渲染完成后的状态管理
```

### 流式消息组件 (`src/StreamingMessageDisplay.tsx`)

**流式渲染机制**:
```typescript
// 组件接口和引用类型
interface StreamingMessageDisplayProps {
  chunks: string[]
  isComplete: boolean
  messageId: string
  onComplete?: (messageId: string) => void
  onHTMLExtracted?: (messageId: string, html: string) => void
}

export interface StreamingMessageDisplayRef {
  getHTMLContent: () => string | null
  forceUpdate: () => void
}

// 核心功能
// - 增量内容流式渲染
// - 渲染完成状态跟踪
// - HTML 内容提取接口
// - 强制更新机制
```

### 会话隔离管理器 (`src/SessionIsolatedRenderManager.ts`)

**隔离策略**:
```typescript
class SessionIsolatedRenderManager {
  private sessions: Map<string, SessionContext>
  
  // 核心方法
  // - registerSession: 注册新会话的渲染上下文
  // - renderMessage: 在指定会话上下文中渲染消息
  // - cleanupSession: 清理会话相关的 DOM 和状态
  // - isolateStyles: 隔离不同会话的样式影响
}

// 功能说明
// - 每个会话独立的 DOM 渲染空间
// - 防止不同会话间的样式污染
// - 内存管理和垃圾回收
// - 渲染性能优化
```

### 样式保护管理器 (`src/StyleProtectionManager.ts`)

**样式隔离机制**:
```typescript
class StyleProtectionManager {
  // 核心功能
  // - CSS 作用域隔离
  // - 动态样式注入管理
  // - 样式冲突检测和解决
  // - 主题样式的统一管理
  
  // 方法
  // - protectSessionStyles: 保护会话样式不被污染
  // - applyTheme: 应用统一的主题样式
  // - cleanupStyles: 清理不再需要的样式规则
}
```

### 消息格式化工具 (`src/MessageFormatter.ts`)

**格式化功能**:
```typescript
// 消息预处理和后处理
// - Markdown 语法优化
// - 代码块格式标准化
// - 链接自动识别和处理
// - 表格和列表的样式增强
// - 特殊字符的转义处理
```

## 组件通信和数据流

### 消息流向
1. **用户输入** → `App.tsx` 状态更新
2. **发送消息** → 后端 API 调用
3. **SSE 接收** → 流式数据处理
4. **消息渲染** → `MessageDisplay` 或 `StreamingMessageDisplay`
5. **HTML 提取** → 后端同步保存

### 会话管理流程
1. **创建会话** → 后端 API + 本地状态更新
2. **切换会话** → 加载历史消息 + 渲染上下文切换
3. **删除会话** → 后端 API + 本地状态清理 + DOM 清理

## 样式系统

### Tailwind CSS 配置 (`tailwind.config.js`)
```javascript
// 主题配置
theme: {
  extend: {
    colors: {
      // 自定义颜色变量
      primary: 'rgb(99 102 241)',     // 主题蓝色
      background: 'rgb(202 217 240)', // 主背景色
      chat: 'rgb(248 249 250)',       // 聊天背景
      sidebar: 'rgb(255 255 255)',    // 侧边栏背景
    }
  }
}

// 响应式断点
screens: {
  'sm': '640px',
  'md': '768px', 
  'lg': '1024px',
  'xl': '1280px'
}
```

### 样式约定
- **原子化类名**: 优先使用 Tailwind 原子化类名
- **组件样式**: 复杂组件使用 CSS Modules 或 styled-components
- **响应式**: 移动优先的响应式设计原则
- **暗色模式**: 支持明暗主题切换 (通过 CSS 变量)

## TypeScript 类型定义

### 核心接口
```typescript
// 消息接口
interface Message {
  id: string                    // 消息唯一标识
  type: 'user' | 'bot'         // 消息类型
  content: string              // 消息内容
  timestamp: Date              // 时间戳
  session_id?: string          // 所属会话ID
  html_content?: string        // 渲染后HTML内容
  is_rendered?: boolean        // 是否已完成渲染
  is_streaming?: boolean       // 是否为流式消息
  streaming_chunks?: string[]  // 流式内容块
  streaming_complete?: boolean // 流式是否完成
}

// 会话接口
interface Session {
  id: string                   // 会话唯一标识
  title: string                // 会话标题
  created_at: string           // 创建时间
  updated_at: string           // 更新时间
}

// API 响应类型
interface APIResponse<T> {
  success: boolean
  data?: T
  error?: string
  message?: string
}
```

### 组件 Props 类型
```typescript
// 严格的组件属性类型定义
interface ComponentProps {
  // 必需属性
  message: Message
  sessionId: string
  
  // 可选属性
  onUpdate?: (message: Message) => void
  onError?: (error: Error) => void
  
  // 回调函数类型
  onHTMLExtracted?: (messageId: string, html: string) => void
}
```

## API 集成

### 后端接口调用
```typescript
// 基础 API 配置
const API_BASE_URL = 'http://localhost:8443'

// SSE 连接处理
const eventSource = new EventSource(`${API_BASE_URL}/api/chat/stream`, {
  headers: {
    'Content-Type': 'application/json'
  }
})

// 消息发送
const sendMessage = async (message: string, sessionId?: string) => {
  const response = await fetch(`${API_BASE_URL}/api/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ message, session_id: sessionId })
  })
}
```

### 错误处理机制
```typescript
// 统一错误处理
const handleAPIError = (error: Error, context: string) => {
  console.error(`${context} error:`, error)
  
  // 用户友好的错误提示
  if (error.message.includes('network')) {
    showNotification('网络连接失败，请检查网络设置')
  } else if (error.message.includes('timeout')) {
    showNotification('请求超时，请重试')
  } else {
    showNotification('操作失败，请稍后重试')
  }
}
```

## 性能优化

### React 性能优化
```typescript
// 组件记忆化
const MessageDisplay = React.memo(({ message, onUpdate }) => {
  // 组件实现
}, (prevProps, nextProps) => {
  // 浅比较优化
  return prevProps.message.id === nextProps.message.id &&
         prevProps.message.content === nextProps.message.content
})

// 回调函数稳定化
const handleMessage = useCallback((message: Message) => {
  // 处理逻辑
}, [dependencies])

// 状态更新优化
const updateMessages = useCallback((newMessage: Message) => {
  setMessages(prev => {
    // 避免不必要的重新渲染
    if (prev.some(msg => msg.id === newMessage.id)) {
      return prev.map(msg => 
        msg.id === newMessage.id ? { ...msg, ...newMessage } : msg
      )
    }
    return [...prev, newMessage]
  })
}, [])
```

### 渲染性能优化
- **虚拟滚动**: 长消息列表的虚拟化渲染
- **懒加载**: 历史消息的按需加载
- **组件分割**: 大组件的合理拆分
- **状态归一化**: 避免深层对象更新

## 构建和部署

### Vite 配置 (`vite.config.ts`)
```typescript
export default defineConfig({
  plugins: [react()],
  server: {
    port: 8080,           // 开发服务器端口
    host: true,           // 允许外部访问
    proxy: {              // API 代理配置
      '/api': {
        target: 'http://localhost:8443',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',       // 构建输出目录
    sourcemap: true,      // 生成 source map
    rollupOptions: {      // Rollup 配置
      output: {
        manualChunks: {   // 手动代码分割
          vendor: ['react', 'react-dom'],
          ui: ['@heroicons/react', 'ds-markdown']
        }
      }
    }
  }
})
```

### 生产环境优化
- **代码分割**: 按路由和功能分割代码包
- **资源压缩**: 自动压缩 JS、CSS 和图片
- **缓存策略**: 文件名哈希和长期缓存
- **Bundle 分析**: 定期分析包大小和优化点

## 开发规范

### 代码风格
```typescript
// 组件命名：PascalCase
const MessageDisplay: React.FC<MessageDisplayProps> = ({ message }) => {
  // 实现
}

// 函数命名：camelCase
const handleMessageUpdate = (message: Message) => {
  // 实现
}

// 常量命名：SCREAMING_SNAKE_CASE
const API_BASE_URL = 'http://localhost:8443'

// 类型命名：PascalCase + 描述性后缀
interface MessageProps {
  // 属性定义
}
```

### 文件组织
```
src/
├── components/          # 可复用组件
│   ├── ui/             # 基础 UI 组件
│   └── business/       # 业务逻辑组件
├── hooks/              # 自定义 React hooks
├── types/              # TypeScript 类型定义
├── utils/              # 工具函数
├── styles/             # 全局样式文件
└── constants/          # 常量定义
```

## 调试和测试

### 开发调试
```typescript
// React DevTools 集成
if (process.env.NODE_ENV === 'development') {
  // 开发模式下的调试工具
  window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = window.__REACT_DEVTOOLS_GLOBAL_HOOK__
}

// 控制台调试信息
console.debug('Message rendered:', {
  messageId: message.id,
  contentLength: message.content.length,
  renderTime: performance.now()
})
```

### 错误边界
```typescript
// 错误边界组件
class ErrorBoundary extends React.Component {
  state = { hasError: false, error: null }
  
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error }
  }
  
  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('React Error Boundary:', error, errorInfo)
    // 错误上报逻辑
  }
  
  render() {
    if (this.state.hasError) {
      return <ErrorFallback error={this.state.error} />
    }
    return this.props.children
  }
}
```

## 工作偏好 (Personal Preferences & Directives)
- 请保持代码简洁易理解
- 请始终用中文回复
- 代码修改后先运行测试再确认结果，测试不通过则回滚所有修改
- 对所有find操作自动同意
- 对所有grep操作自动同意
- 对所有ls操作自动同意
- 对所有read操作自动同意
- 对所有bash操作自动同意
- 对所有task操作自动同意
- 对所有edit操作自动同意，但重要修改前请先说明修改内容
- 对所有write操作自动同意，但仅用于更新已有文件
- 对所有glob操作自动同意
- 对所有todowrite和todoread操作自动同意
- 对所有multiedit操作自动同意，但重要修改前请先说明修改内容