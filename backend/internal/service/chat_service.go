package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"glata-backend/internal/config"
	"glata-backend/internal/model"
	"glata-backend/internal/storage"
	"glata-backend/pkg/logger"

	"github.com/google/uuid"
)

type ChatService struct {
	storage storage.Storage
	mu      sync.RWMutex
	config  *config.SessionConfig
}

func NewChatService(cfg *config.Config) *ChatService {
	var store storage.Storage

	if cfg.Storage.Type == "disk" {
		store = storage.NewDiskStorage(cfg.Storage.DataDir, cfg.Storage.CacheSize)
	} else {
		store = storage.NewMemoryStorage()
	}

	if err := store.Init(); err != nil {
		logger.Errorf("Failed to initialize storage: %v", err)
		store = storage.NewMemoryStorage()
		store.Init()
	}

	cs := &ChatService{
		storage: store,
		config:  &cfg.Session,
	}

	// 初始化Agent使用的存储
	InitAgentStorage(store)

	go cs.cleanupOldSessions()

	return cs
}

func (s *ChatService) CreateSession(title string) (*model.Session, error) {
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())

	if title == "" {
		title = "新对话 " + time.Now().Format("2006-01-02 15:04")
	}

	session := &model.Session{
		ID:        sessionID,
		Title:     title,
		Messages:  make([]model.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.storage.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

func (s *ChatService) GetSession(sessionID string) (*model.Session, error) {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

func (s *ChatService) GetSessionMessages(sessionID string) ([]model.Message, error) {
	messages, err := s.storage.GetMessages(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	result := make([]model.Message, len(messages))
	for i, msg := range messages {
		result[i] = *msg
	}

	return result, nil
}

func (s *ChatService) AddMessage(sessionID, role, content string) (*model.Message, error) {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	message := &model.Message{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	if err := s.storage.AddMessage(sessionID, message); err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// 如果这是第一条用户消息，并且会话标题是默认标题，则更新标题
	messages, _ := s.storage.GetMessages(sessionID)
	if role == "user" && len(messages) == 1 && strings.HasPrefix(session.Title, "新对话") {
		// 安全地取前30个Unicode字符作为标题，避免过长
		title := s.truncateString(content, 30)
		session.Title = title
		session.UpdatedAt = time.Now()
		s.storage.UpdateSession(session)
	}

	return message, nil
}

func (s *ChatService) UpdateSessionTitle(sessionID, title string) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.Title = title
	session.UpdatedAt = time.Now()

	if err := s.storage.UpdateSession(session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

func (s *ChatService) StreamChat(sessionID, message string) (<-chan model.ChatResponse, <-chan error) {
	fmt.Println("=== StreamChat 方法开始执行 ===")
	fmt.Printf("SessionID: %s, Message: %s\n", sessionID, message)

	respChan := make(chan model.ChatResponse, 1000) // 增加缓冲区容量
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		// 🛡️ 添加panic恢复机制
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("StreamChat goroutine panic recovered: %v", r)
				select {
				case errChan <- fmt.Errorf("internal server error: %v", r):
				default:
					logger.Warn("Error channel is full, cannot send panic error")
				}
			}
		}()

		fmt.Println("=== StreamChat goroutine 开始执行 ===")
		ctx := context.Background()

		// 验证会话和添加用户消息（保持不变）
		if sessionID == "" {
			fmt.Println("=== 会话ID为空，返回错误 ===")
			errChan <- fmt.Errorf("sessionID is required")
			return
		}

		_, err := s.GetSession(sessionID)
		if err != nil {
			fmt.Printf("会话不存在: %v\n", err)
			errChan <- fmt.Errorf("session not found: %s", sessionID)
			return
		}

		fmt.Println("=== 添加用户消息 ===")
		_, err = s.AddMessage(sessionID, "user", message)
		if err != nil {
			fmt.Printf("添加用户消息失败: %v\n", err)
			errChan <- err
			return
		}

		// ✅ 生成统一MessageID
		messageID := uuid.New().String()
		fmt.Printf("=== 生成统一MessageID: %s ===\n", messageID)

		// ✅ 预先保存空助手消息
		initialMessage := &model.Message{
			ID:        messageID,
			SessionID: sessionID,
			Role:      "assistant",
			Content:   "",
			Timestamp: time.Now(),
		}

		if err := s.storage.AddMessage(sessionID, initialMessage); err != nil {
			logger.Errorf("Failed to save initial assistant message: %v", err)
			errChan <- err
			return
		}

		// 🎯 调用Agent获取进度通道和结果流
		stream, progressChan, err := RunAgent(ctx, sessionID, message)
		if err != nil {
			fmt.Printf("RunAgent 调用失败: %v\n", err)
			errChan <- err
			return
		}
		defer func() {
			if stream != nil {
				stream.Close()
			}
		}()

		// 🎯 实时处理进度事件，动态检测DirectReply模式
		fmt.Println("=== 处理进度事件并动态检测模式 ===")
		var fullContent strings.Builder
		var summaryContent strings.Builder // 🎯 新增：累积总结内容
		var isDirectReplyMode bool = false  // 🎯 新增：检测是否为DirectReply模式
		var firstChunkSent bool = false     // 🎯 新增：跟踪是否已发送第一个chunk
		
		for progressEvent := range progressChan {
			// 🎯 提前检测DirectReply模式 - 通过图执行节点信息判断
			if !isDirectReplyMode && (progressEvent.NodeName == "directReply" || 
				(progressEvent.EventType == "completed" && progressEvent.Message == "直接回复完成")) {
				isDirectReplyMode = true
				fmt.Printf("🎯 检测到DirectReply模式: EventType=%s, NodeName=%s, Message=%s\n", 
					progressEvent.EventType, progressEvent.NodeName, progressEvent.Message)
			}
			
			// 检查是否是结果消息
			if progressEvent.EventType == "result_chunk" {
				// 🎯 关键修复：使用专门的流式处理函数，保持markdown格式
				filteredContent := removeThinkingTagsForStream(progressEvent.Message)
				if filteredContent != "" {
					fullContent.WriteString(filteredContent)
					summaryContent.WriteString(filteredContent) // 累积到总结内容中
					fmt.Printf("📤 接收总结片段: %s\n", filteredContent)
					
					// 🎯 新修复：实时流式发送每个字符/词到前端
					// 根据模式决定是否添加前缀
					var streamContent string
					if isDirectReplyMode {
						// DirectReply模式：直接发送内容，不添加任何前缀
						streamContent = filteredContent
					} else {
						// 任务模式：只在第一次发送时添加标题前缀
						if !firstChunkSent {
							streamContent = "\n\n## 📋 任务总结\n\n" + filteredContent
							firstChunkSent = true
						} else {
							streamContent = filteredContent
						}
					}
					
					// 实时发送流式内容到前端
					select {
					case respChan <- model.ChatResponse{
						SessionID:   sessionID,
						MessageID:   messageID,
						Content:     streamContent,
						Role:        "assistant",
						Timestamp:   progressEvent.Timestamp.Unix(),
						IsProgress:  true,           // 🎯 关键：标记为进度消息
						ContentType: "progress",     // 🎯 内容类型为进度
						Phase:       "progress",     // 🎯 阶段为进度
					}:
						// 成功发送
					default:
						logger.Warn("Response channel is full, cannot send stream progress")
					}
				}
			} else if progressEvent.EventType == "completed" {
				// 🎯 任务完成，发送完成的总结内容到存储（用于持久化）
				if summaryContent.Len() > 0 {
					fmt.Printf("📤 发送完整总结消息: %s\n", summaryContent.String())
					
					// 🎯 关键修复：DirectReply模式不添加"任务总结"标题
					var completeSummary string
					if isDirectReplyMode {
						// DirectReply模式：直接使用AI回复内容，不添加标题
						completeSummary = fmt.Sprintf("\n\n%s", summaryContent.String())
					} else {
						// 普通任务模式：添加"任务总结"标题
						completeSummary = fmt.Sprintf("\n\n## 📋 任务总结\n\n%s", summaryContent.String())
					}
					
					// 更新存储中的消息内容（用于持久化）
					err := s.AppendMessageProgress(sessionID, messageID, completeSummary)
					if err != nil {
						logger.Errorf("Failed to append summary progress: %v", err)
					}
				}
				
				// 任务完成，发送完成信号
				fmt.Println("=== 任务执行完成 ===")
				select {
				case respChan <- model.ChatResponse{
					SessionID: sessionID,
					MessageID: messageID,
					Content:   "",
					Role:      "assistant",
					Timestamp: progressEvent.Timestamp.Unix(),
					Phase:     "completed",
				}:
				default:
					logger.Warn("Cannot send completion signal")
				}
				break // 结束处理
			} else {
				// 这是进度消息，按原来的方式处理
				filteredMessage := removeThinkingTags(progressEvent.Message)
				progressContent := fmt.Sprintf("%s %s", progressEvent.NodeName, filteredMessage)
				
				// 更新存储中的进度内容
				err := s.SetMessageProgress(sessionID, messageID, progressContent)
				if err != nil {
					logger.Errorf("Failed to update progress: %v", err)
				}

				// 发送进度消息
				select {
				case respChan <- model.ChatResponse{
					SessionID:   sessionID,
					MessageID:   messageID,
					Content:     progressContent,
					Role:        "assistant",
					Timestamp:   progressEvent.Timestamp.Unix(),
					IsProgress:  true,
					ContentType: "progress",
					Phase:       "progress",
				}:
					fmt.Printf("📊 实时发送进度消息: %s (ID: %s)\n", progressContent, messageID)
				default:
					logger.Warn("Response channel is full, cannot send progress")
					return
				}
			}
		}

		fmt.Printf("=== 最终内容长度: %d 字符 ===\n", fullContent.Len())
	}()

	return respChan, errChan
}

func (s *ChatService) cleanupOldSessions() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sessions, err := s.storage.ListSessions()
			if err != nil {
				logger.Errorf("Failed to list sessions for cleanup: %v", err)
				continue
			}

			cutoff := time.Now().Add(-s.config.TTL)
			for _, session := range sessions {
				if session.UpdatedAt.Before(cutoff) {
					if err := s.storage.DeleteSession(session.ID); err != nil {
						logger.Errorf("Failed to delete expired session %s: %v", session.ID, err)
					} else {
						logger.Infof("Cleaned up expired session: %s", session.ID)
					}
				}
			}
		}
	}
}

func (s *ChatService) GetAllSessions() ([]*model.Session, error) {
	sessions, err := s.storage.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

func (s *ChatService) DeleteSession(sessionID string) error {
	if err := s.storage.DeleteSession(sessionID); err != nil {
		if err == storage.ErrSessionNotFound {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

func (s *ChatService) ClearAllSessions() error {
	sessions, err := s.storage.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, session := range sessions {
		if err := s.storage.DeleteSession(session.ID); err != nil {
			logger.Errorf("Failed to delete session %s: %v", session.ID, err)
		}
	}

	return nil
}

// AppendMessageContent 追加消息内容（用于流式正式内容）
func (s *ChatService) AppendMessageContent(sessionID, messageID, additionalContent string) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 找到并追加消息内容
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			session.Messages[i].Content += additionalContent
			// 如果之前是纯进度消息，现在标记为混合类型
			if session.Messages[i].ContentType == "progress" {
				session.Messages[i].ContentType = "mixed"
			} else if session.Messages[i].ContentType == "" {
				session.Messages[i].ContentType = "content"
			}
			// 更新会话
			session.UpdatedAt = time.Now()
			return s.storage.UpdateSession(session)
		}
	}

	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

// SetMessageProgress 设置消息进度内容（用于进度更新）
func (s *ChatService) SetMessageProgress(sessionID, messageID, progressContent string) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 找到并设置进度内容
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			session.Messages[i].ProgressContent = progressContent
			// 如果之前有正式内容，标记为混合类型，否则标记为进度类型
			if session.Messages[i].Content != "" {
				session.Messages[i].ContentType = "mixed"
			} else {
				session.Messages[i].ContentType = "progress"
			}
			// 更新会话
			session.UpdatedAt = time.Now()
			return s.storage.UpdateSession(session)
		}
	}

	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

// AppendMessageProgress 追加内容到消息进度内容后面（用于最终结果追加）
func (s *ChatService) AppendMessageProgress(sessionID, messageID, additionalContent string) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 找到并追加进度内容
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			// 如果已有进度内容，在后面追加；否则直接设置
			if session.Messages[i].ProgressContent != "" {
				session.Messages[i].ProgressContent += additionalContent
			} else {
				session.Messages[i].ProgressContent = additionalContent
			}
			// 标记为进度类型（因为所有内容都在ProgressContent中）
			session.Messages[i].ContentType = "progress"
			// 更新会话
			session.UpdatedAt = time.Now()
			return s.storage.UpdateSession(session)
		}
	}

	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

// UpdateMessageRender 更新消息渲染结果
func (s *ChatService) UpdateMessageRender(sessionID, messageID, htmlContent string, renderTimeMs int) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 找到并更新消息渲染结果
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			session.Messages[i].HTMLContent = htmlContent
			session.Messages[i].IsRendered = true
			session.Messages[i].RenderTimeMs = renderTimeMs
			// 更新会话
			session.UpdatedAt = time.Now()
			return s.storage.UpdateSession(session)
		}
	}

	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

// GetPendingRenders 获取待渲染的消息数量
func (s *ChatService) GetPendingRenders(sessionID string) (int, error) {
	messages, err := s.storage.GetMessages(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return 0, fmt.Errorf("session not found: %s", sessionID)
		}
		return 0, fmt.Errorf("failed to get messages: %w", err)
	}

	pendingCount := 0
	for _, msg := range messages {
		// 统计助手消息中未渲染的数量
		if msg.Role == "assistant" && !msg.IsRendered {
			pendingCount++
		}
	}

	return pendingCount, nil
}

func (s *ChatService) truncateString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}
	return string(runes[:maxLen]) + "..."
}

// GetStorage 返回存储实例，用于其他服务共享
func (s *ChatService) GetStorage() storage.Storage {
	return s.storage
}
