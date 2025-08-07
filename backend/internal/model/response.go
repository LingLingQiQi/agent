package model

import "time"

type ChatResponse struct {
	SessionID       string `json:"session_id"`
	MessageID       string `json:"message_id"`
	Content         string `json:"content"`
	Role            string `json:"role"`
	Timestamp       int64  `json:"timestamp"`
	Type            string `json:"type,omitempty"`              // message, todo_update, todo_list
	IsBackground    bool   `json:"is_background"`               // ✅ 约束3：标识是否为后台模式
	IsProgress      bool   `json:"is_progress,omitempty"`       // 是否为进度消息
	ContentType     string `json:"content_type,omitempty"`      // "progress", "content", "mixed"
	Phase           string `json:"phase,omitempty"`             // "progress" | "result_start" | "result" | "completed"
	Mode            string `json:"mode,omitempty"`              // "DIRECT_REPLY" | "TODO_LIST" - 解决前端渲染截断问题
	ContentStage    string `json:"content_stage,omitempty"`     // "thinking" | "answer" - 内容阶段标识
	StreamType      string `json:"stream_type,omitempty"`       // "real" | "fake" - 流式类型标识
}

type SessionResponse struct {
	SessionID    string    `json:"session_id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type Message struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`                      // 最终正式内容 (Markdown)
	ProgressContent string    `json:"progress_content,omitempty"`   // 进度内容 (纯文本)
	ContentType     string    `json:"content_type"`                 // "progress", "content", "mixed"
	HTMLContent     string    `json:"html_content,omitempty"`       // 渲染后的HTML内容
	IsRendered      bool      `json:"is_rendered"`                  // 是否已渲染
	RenderTimeMs    int       `json:"render_time_ms,omitempty"`     // 渲染时间(毫秒)
	Timestamp       time.Time `json:"timestamp"`
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

