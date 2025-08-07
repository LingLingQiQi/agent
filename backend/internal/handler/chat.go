package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"glata-backend/internal/model"
	"glata-backend/internal/service"
	"glata-backend/internal/utils"
	"glata-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService *service.ChatService
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

func (h *ChatHandler) StreamChat(c *gin.Context) {
	fmt.Println("=== ChatHandler.StreamChat 开始执行 ===")

	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("请求解析失败: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("收到聊天请求 - SessionID: %s, Message: %s, BackgroundMode: %v\n", 
		req.SessionID, req.Message, req.BackgroundMode)

	sseWriter := utils.NewSSEWriter(c.Writer)

	fmt.Println("调用 chatService.StreamChat...")
	respChan, errChan := h.chatService.StreamChat(req.SessionID, req.Message)

	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				sseWriter.Close()
				return
			}

			// ✅ 约束3：在响应中标识是否为后台模式
			resp.IsBackground = req.BackgroundMode

			data, err := json.Marshal(resp)
			if err != nil {
				logger.Errorf("Failed to marshal response: %v", err)
				continue
			}

			if err := sseWriter.Write("message", string(data)); err != nil {
				logger.Errorf("Failed to write SSE: %v", err)
				return
			}

		case err := <-errChan:
			if err != nil {
				errData, _ := json.Marshal(gin.H{"error": err.Error()})
				sseWriter.Write("error", string(errData))
				sseWriter.Close()
				return
			}

		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *ChatHandler) CreateSession(c *gin.Context) {
	var req model.CreateSessionRequest
	// 允许空的请求体，使用默认标题
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果无法解析JSON，使用默认标题
		req.Title = "新对话"
	}

	// 如果标题为空，使用默认标题
	if req.Title == "" {
		req.Title = "新对话"
	}

	session, err := h.chatService.CreateSession(req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *ChatHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	session, err := h.chatService.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.SessionResponse{
		SessionID:    session.ID,
		Title:        session.Title,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
		MessageCount: len(session.Messages),
	})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	sessionID := c.Param("session_id")

	messages, err := h.chatService.GetSessionMessages(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   messages,
	})
}

func (h *ChatHandler) GetSessionList(c *gin.Context) {
	sessions, err := h.chatService.GetAllSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
	})
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	err := h.chatService.DeleteSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted successfully"})
}

func (h *ChatHandler) ClearAllSessions(c *gin.Context) {
	err := h.chatService.ClearAllSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All sessions cleared successfully"})
}

func (h *ChatHandler) UpdateSessionTitle(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req struct {
		Title string `json:"title" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.chatService.UpdateSessionTitle(sessionID, req.Title)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Title updated successfully"})
}

// ✅ 约束2：更新消息渲染结果 - 增加会话ID验证
func (h *ChatHandler) UpdateMessageRender(c *gin.Context) {
	messageID := c.Param("message_id")

	var req model.RenderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.chatService.UpdateMessageRender(req.SessionID, messageID, req.HTMLContent, req.RenderTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Render updated successfully"})
}

// ✅ 约束2：批量更新渲染结果 - 按会话ID分组
func (h *ChatHandler) UpdateSessionRenderBatch(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req model.BatchRenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.chatService.UpdateMessagesRender(sessionID, req.Renders)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Batch render updated successfully"})
}

// ✅ 约束2：获取未渲染的消息 - 严格按会话ID过滤
func (h *ChatHandler) GetPendingRenders(c *gin.Context) {
	sessionID := c.Param("session_id")

	messages, err := h.chatService.GetPendingRenders(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 估算渲染时间（假设每条消息平均150ms）
	estimatedTime := int64(len(messages) * 150)

	response := model.PendingRendersResponse{
		SessionID:           sessionID, // ✅ 约束2：返回响应包含会话ID
		Messages:            convertMessages(messages),
		Total:               len(messages),
		EstimatedRenderTime: estimatedTime,
	}

	c.JSON(http.StatusOK, response)
}

// 转换指针切片为值切片
func convertMessages(messages []*model.Message) []model.Message {
	result := make([]model.Message, len(messages))
	for i, msg := range messages {
		if msg != nil {
			result[i] = *msg
		}
	}
	return result
}
