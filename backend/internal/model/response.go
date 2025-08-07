package model

import "time"

type ChatResponse struct {
	SessionID    string `json:"session_id"`
	MessageID    string `json:"message_id"`
	Content      string `json:"content"`
	Role         string `json:"role"`
	Timestamp    int64  `json:"timestamp"`
	Type         string `json:"type,omitempty"`         // message, todo_update, todo_list
	IsBackground bool   `json:"is_background"`          // ✅ 约束3：标识是否为后台模式
}

type SessionResponse struct {
	SessionID    string    `json:"session_id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type Message struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`      // 原始Markdown内容
	HTMLContent string    `json:"html_content"` // 渲染后的HTML
	IsRendered  bool      `json:"is_rendered"`  // 是否已渲染
	RenderTime  int64     `json:"render_time_ms"` // 渲染耗时(毫秒)
	Timestamp   time.Time `json:"timestamp"`
}

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StreamChunk struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

// 待渲染消息响应
type PendingRendersResponse struct {
	SessionID            string    `json:"session_id"`             // ✅ 约束2：返回响应包含会话ID
	Messages             []Message `json:"messages"`
	Total                int       `json:"total"`
	EstimatedRenderTime  int64     `json:"estimated_render_time_ms"`
}
