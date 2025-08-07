package model

type ChatRequest struct {
	Message        string `json:"message" binding:"required"`
	SessionID      string `json:"session_id"`
	BackgroundMode bool   `json:"background_mode"` // ✅ 约束3：是否为后台模式
}

type CreateSessionRequest struct {
	Title string `json:"title"`
}

// RenderRequest 消息渲染请求
type RenderRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	HTMLContent  string `json:"html_content" binding:"required"`
	RenderTimeMs int    `json:"render_time_ms"`
}

