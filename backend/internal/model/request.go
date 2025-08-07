package model

type ChatRequest struct {
	Message        string `json:"message" binding:"required"`
	SessionID      string `json:"session_id"`
	BackgroundMode bool   `json:"background_mode"` // ✅ 约束3：是否为后台模式
}

type CreateSessionRequest struct {
	Title string `json:"title"`
}

// 渲染更新请求
type RenderUpdateRequest struct {
	SessionID   string `json:"session_id" binding:"required"`   // ✅ 约束2：强制会话ID验证
	HTMLContent string `json:"html_content" binding:"required"`
	RenderTime  int64  `json:"render_time_ms"`
}

// 批量渲染更新请求
type BatchRenderRequest struct {
	Renders []RenderUpdate `json:"renders" binding:"required"`
}

type RenderUpdate struct {
	MessageID   string `json:"message_id" binding:"required"`
	HTMLContent string `json:"html_content" binding:"required"`
	RenderTime  int64  `json:"render_time_ms"`
}