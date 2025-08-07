package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	
	// ✅ 设置连接超时和心跳机制
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Minute) // 25分钟超时
	defer cancel()
	
	// ✅ 启动心跳goroutine，防止连接因空闲而断开
	heartbeatTicker := time.NewTicker(30 * time.Second) // 每30秒发送心跳
	defer heartbeatTicker.Stop()
	
	go func() {
		for {
			select {
			case <-heartbeatTicker.C:
				// 发送心跳消息，让前端知道连接仍然活跃
				heartbeatData, _ := json.Marshal(gin.H{
					"type": "heartbeat",
					"timestamp": time.Now().Unix(),
					"message": "连接正常",
				})
				if err := sseWriter.Write("heartbeat", string(heartbeatData)); err != nil {
					logger.Warnf("心跳发送失败: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	fmt.Println("调用 chatService.StreamChat...")
	respChan, errChan := h.chatService.StreamChat(req.SessionID, req.Message)
	
	// ✅ 添加处理开始通知
	startData, _ := json.Marshal(gin.H{
		"type": "processing_start",
		"message": "开始处理您的请求...",
		"timestamp": time.Now().Unix(),
	})
	sseWriter.Write("status", string(startData))

	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				// ✅ 处理完成通知
				completeData, _ := json.Marshal(gin.H{
					"type": "processing_complete",
					"message": "处理完成",
					"timestamp": time.Now().Unix(),
				})
				sseWriter.Write("status", string(completeData))
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
				// ✅ 增强错误信息
				errorData, _ := json.Marshal(gin.H{
					"error": err.Error(),
					"type": "service_error",
					"timestamp": time.Now().Unix(),
					"suggestion": "请检查网络连接或稍后重试",
				})
				sseWriter.Write("error", string(errorData))
				sseWriter.Close()
				return
			}

		case <-ctx.Done():
			// ✅ 超时或取消处理
			if ctx.Err() == context.DeadlineExceeded {
				timeoutData, _ := json.Marshal(gin.H{
					"error": "处理超时",
					"type": "timeout",
					"timestamp": time.Now().Unix(),
					"suggestion": "请求处理时间过长，建议简化请求或稍后重试",
				})
				sseWriter.Write("error", string(timeoutData))
			}
			sseWriter.Close()
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

// UpdateMessageRender 更新消息渲染结果
func (h *ChatHandler) UpdateMessageRender(c *gin.Context) {
	messageID := c.Param("message_id")

	var req model.RenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.chatService.UpdateMessageRender(req.SessionID, messageID, req.HTMLContent, req.RenderTimeMs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Render updated successfully"})
}

// GetPendingRenders 获取待渲染的消息数量
func (h *ChatHandler) GetPendingRenders(c *gin.Context) {
	sessionID := c.Param("session_id")

	count, err := h.chatService.GetPendingRenders(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"total":      count,
	})
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
