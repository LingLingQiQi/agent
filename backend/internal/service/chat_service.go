package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"glata-backend/internal/config"
	"glata-backend/internal/model"
	"glata-backend/internal/storage"
	"glata-backend/pkg/logger"

	"github.com/google/uuid"
)

// ProgressStep 表示单个进度步骤
type ProgressStep struct {
	NodeName    string    `json:"node_name"`    // 节点名称
	Message     string    `json:"message"`      // 进度消息
	Status      string    `json:"status"`       // 状态：in_progress, completed, error
	Timestamp   time.Time `json:"timestamp"`    // 时间戳
	Emoji       string    `json:"emoji"`        // emoji图标
}

// ProgressMessageManager 管理累积式进度消息
type ProgressMessageManager struct {
	sessionID         string         `json:"session_id"`
	progressMessageID string         `json:"progress_message_id"`
	progressSteps     []ProgressStep `json:"progress_steps"`
	isCompleted       bool           `json:"is_completed"`
	mu                sync.RWMutex   `json:"-"` // 不序列化mutex
}

// NewProgressMessageManager 创建新的进度消息管理器
func NewProgressMessageManager(sessionID string) *ProgressMessageManager {
	return &ProgressMessageManager{
		sessionID:         sessionID,
		progressMessageID: "progress-" + sessionID,
		progressSteps:     make([]ProgressStep, 0),
		isCompleted:       false,
	}
}

// AddProgress 添加进度步骤
func (pm *ProgressMessageManager) AddProgress(nodeName, message, status string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// 根据状态选择合适的emoji
	emoji := pm.getEmojiForStatus(status, nodeName)
	
	// 检查是否已存在相同节点的步骤，如果存在则更新
	for i, step := range pm.progressSteps {
		if step.NodeName == nodeName {
			pm.progressSteps[i] = ProgressStep{
				NodeName:  nodeName,
				Message:   message,
				Status:    status,
				Timestamp: time.Now(),
				Emoji:     emoji,
			}
			return
		}
	}
	
	// 如果不存在，则添加新步骤
	pm.progressSteps = append(pm.progressSteps, ProgressStep{
		NodeName:  nodeName,
		Message:   message,
		Status:    status,
		Timestamp: time.Now(),
		Emoji:     emoji,
	})
}

// getEmojiForStatus 根据状态和节点名称获取合适的emoji
func (pm *ProgressMessageManager) getEmojiForStatus(status, nodeName string) string {
	if status == "completed" {
		return "✅"
	} else if status == "error" {
		return "❌"
	}
	
	// 根据节点名称选择不同的进行中emoji
	switch {
	case strings.Contains(nodeName, "UserMessageToMap"):
		return "⏳"
	case strings.Contains(nodeName, "PlanModel"):
		return "📝"
	case strings.Contains(nodeName, "WritePlan"):
		return "💾"
	case strings.Contains(nodeName, "ScanTodoList"):
		return "🔍"
	case strings.Contains(nodeName, "ExecuteModel"):
		return "⚡"
	case strings.Contains(nodeName, "ToolsNode"):
		return "🔧"
	case strings.Contains(nodeName, "UpdateTodoListModel"):
		return "🔄"
	case strings.Contains(nodeName, "SummaryModel"):
		return "📊"
	default:
		return "🔄"
	}
}

// MarkCompleted 标记进度为完成
func (pm *ProgressMessageManager) MarkCompleted() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.isCompleted = true
}

// BuildMarkdownContent 构建Markdown格式的进度内容
func (pm *ProgressMessageManager) BuildMarkdownContent() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var content strings.Builder
	
	// 添加标题
	if pm.isCompleted {
		content.WriteString("## ✅ 处理完成\n\n")
	} else {
		content.WriteString("## 🔄 正在处理中...\n\n")
	}
	
	// 添加进度步骤列表
	for _, step := range pm.progressSteps {
		content.WriteString(fmt.Sprintf("- %s **%s**: %s\n", 
			step.Emoji, step.NodeName, step.Message))
	}
	
	// 添加分割线和状态说明
	content.WriteString("\n---\n")
	if pm.isCompleted {
		content.WriteString("*所有步骤执行完毕*")
	} else {
		content.WriteString("*处理中，请稍候...*")
	}
	
	return content.String()
}

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

	respChan := make(chan model.ChatResponse, 100) // 增加缓冲区，确保进度消息实时发送
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		fmt.Println("=== StreamChat goroutine 开始执行 ===")
		ctx := context.Background()

		// 只有在 sessionID 为空时才创建新会话
		// 这应该只在前端明确没有会话时才发生
		if sessionID == "" {
			fmt.Println("=== 会话ID为空，返回错误 ===")
			errChan <- fmt.Errorf("sessionID is required")
			return
		}

		// 检查会话是否存在
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
		fmt.Println("用户消息添加成功")

		fmt.Println("=== 准备调用 RunAgentWithProgress ===")
		// 调用带进度报告的 RunAgent 方法
		stream, progressChan, err := RunAgentWithProgress(ctx, sessionID, message)
		if err != nil {
			fmt.Printf("RunAgentWithProgress 调用失败: %v\n", err)
			errChan <- fmt.Errorf("RunAgentWithProgress 调用失败: %w", err)
			return
		}
		fmt.Println("=== RunAgentWithProgress 调用成功 ===")

		// 创建进度消息管理器
		progressManager := NewProgressMessageManager(sessionID)
		
		// 启动 goroutine 处理进度事件 - 增加优先级调度
		progressDone := make(chan bool, 1)
		go func() {
			defer func() { progressDone <- true }()
			fmt.Println("=== 开始监听进度事件 ===")
			
			for progressEvent := range progressChan {
				// 确定进度状态
				status := "in_progress"
				if progressEvent.EventType == "node_complete" {
					status = "completed"
				} else if progressEvent.EventType == "node_error" {
					status = "error"
				}
				
				// 添加进度到管理器
				progressManager.AddProgress(progressEvent.NodeName, progressEvent.Message, status)
				
				// 构建Markdown内容
				markdownContent := progressManager.BuildMarkdownContent()
				
				// 发送累积的进度消息 - 使用阻塞发送确保实时性
				respChan <- model.ChatResponse{
					SessionID: sessionID,
					MessageID: progressManager.progressMessageID, // 使用固定的进度消息ID
					Content:   markdownContent,
					Role:      "assistant", // 改为assistant，作为Bot消息显示
					Timestamp: progressEvent.Timestamp.Unix(),
				}
				
				fmt.Printf("📊 发送进度更新: %s - %s\n", progressEvent.NodeName, progressEvent.Message)
				
				// 强制让出CPU时间，确保消息能被处理
				time.Sleep(1 * time.Millisecond)
			}
			
			// 进度完成后，标记管理器为完成状态并发送最终进度消息
			progressManager.MarkCompleted()
			finalMarkdownContent := progressManager.BuildMarkdownContent()
			
			respChan <- model.ChatResponse{
				SessionID: sessionID,
				MessageID: progressManager.progressMessageID,
				Content:   finalMarkdownContent,
				Role:      "assistant",
				Timestamp: time.Now().Unix(),
			}
			
			fmt.Println("✅ 发送最终进度完成消息")
			fmt.Println("=== 进度事件监听结束 ===")
		}()

		defer stream.Close()

		var fullContent strings.Builder
		messageID := uuid.New().String()
		
		// ✅ 立即保存空的助手消息，确保render API能找到消息
		fmt.Printf("=== 预先保存空助手消息，ID: %s ===\n", messageID)
		initialMessage := &model.Message{
			ID:        messageID,
			SessionID: sessionID,
			Role:      "assistant",
			Content:   "", // 空内容，稍后更新
			Timestamp: time.Now(),
		}
		
		saveErr := s.storage.AddMessage(sessionID, initialMessage)
		if saveErr != nil {
			logger.Errorf("Failed to save initial assistant message: %v", saveErr)
			fmt.Printf("保存初始助手消息失败: %v\n", saveErr)
			errChan <- saveErr
			return
		}
		fmt.Printf("初始助手消息保存成功，ID: %s\n", messageID)

		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					fmt.Printf("=== 流结束，更新完整助手消息 ===\n")
					// 流结束，更新已存在消息的内容
					if fullContent.Len() > 0 {
						fmt.Printf("更新助手消息: %s (ID: %s)\n", fullContent.String(), messageID)
						
						// 更新现有消息的内容
						updateErr := s.UpdateMessageContent(sessionID, messageID, fullContent.String())
						if updateErr != nil {
							logger.Errorf("Failed to update assistant message: %v", updateErr)
							fmt.Printf("更新助手消息失败: %v\n", updateErr)
						} else {
							fmt.Printf("助手消息更新成功，ID: %s\n", messageID)
							
							// ✅ 自动渲染HTML内容
							fmt.Printf("=== 准备启动自动渲染goroutine ===\n")
							go func() {
								fmt.Printf("=== 开始自动渲染HTML内容 ===\n")
								renderErr := s.autoRenderMessageHTML(sessionID, messageID, fullContent.String())
								if renderErr != nil {
									fmt.Printf("自动HTML渲染失败: %v\n", renderErr)
								} else {
									fmt.Printf("自动HTML渲染成功，消息ID: %s\n", messageID)
								}
							}()
						}
					} else {
						fmt.Printf("助手消息内容为空，保持空内容\n")
					}
					
					// 等待进度处理完成
					fmt.Println("=== 等待进度处理完成 ===")
					<-progressDone
					fmt.Println("=== 进度处理已完成，返回 ===")
					return
				}
				fmt.Printf("流接收错误: %v\n", err)
				errChan <- err
				return
			}

			if chunk.Content != "" {
				fullContent.WriteString(chunk.Content)
				fmt.Printf("接收到消息块: %s\n", chunk.Content)

				respChan <- model.ChatResponse{
					SessionID: sessionID,
					MessageID: messageID,
					Content:   chunk.Content,
					Role:      "assistant",
					Timestamp: time.Now().Unix(),
				}
			}
		}
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

// UpdateMessageContent 更新消息内容
func (s *ChatService) UpdateMessageContent(sessionID, messageID, content string) error {
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 找到并更新消息内容
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			session.Messages[i].Content = content
			// 更新会话
			session.UpdatedAt = time.Now()
			return s.storage.UpdateSession(session)
		}
	}

	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

func (s *ChatService) truncateString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}
	return string(runes[:maxLen]) + "..."
}

// ✅ 约束2：更新单个消息渲染结果，严格验证会话ID
func (s *ChatService) UpdateMessageRender(sessionID, messageID, htmlContent string, renderTime int64) error {
	return s.storage.UpdateMessageRender(sessionID, messageID, htmlContent, renderTime)
}

// ✅ 约束2：批量更新渲染结果，按会话ID分组验证
func (s *ChatService) UpdateMessagesRender(sessionID string, renders []model.RenderUpdate) error {
	return s.storage.UpdateMessagesRender(sessionID, renders)
}

// ✅ 约束2：获取未渲染的消息，严格按会话ID过滤
func (s *ChatService) GetPendingRenders(sessionID string) ([]*model.Message, error) {
	messages, err := s.storage.GetPendingRenders(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get pending renders: %w", err)
	}

	return messages, nil
}

// GetStorage 返回存储实例，用于其他服务共享
func (s *ChatService) GetStorage() storage.Storage {
	return s.storage
}

// autoRenderMessageHTML 已弃用 - 前端现在负责HTML渲染
// 保留此方法为空实现以维持兼容性
func (s *ChatService) autoRenderMessageHTML(sessionID, messageID, content string) error {
	// 前端现在负责HTML渲染和保存
	// 此方法不再执行任何操作
	fmt.Printf("=== autoRenderMessageHTML已弃用，前端负责HTML渲染 ===\n")
	return nil
}
