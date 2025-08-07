package storage

import (
	"glata-backend/internal/model"
	"sync"
)

type MemoryStorage struct {
	sessions map[string]*model.Session
	mu       sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		sessions: make(map[string]*model.Session),
	}
}

func (m *MemoryStorage) Init() error {
	return nil
}

func (m *MemoryStorage) Close() error {
	return nil
}

func (m *MemoryStorage) Backup() error {
	return nil
}

func (m *MemoryStorage) CreateSession(session *model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.sessions[session.ID] = session
	return nil
}

func (m *MemoryStorage) GetSession(sessionID string) (*model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	
	return session, nil
}

func (m *MemoryStorage) UpdateSession(session *model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[session.ID]; !exists {
		return ErrSessionNotFound
	}
	
	m.sessions[session.ID] = session
	return nil
}

func (m *MemoryStorage) DeleteSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[sessionID]; !exists {
		return ErrSessionNotFound
	}
	
	delete(m.sessions, sessionID)
	return nil
}

func (m *MemoryStorage) ListSessions() ([]*model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sessions := make([]*model.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

func (m *MemoryStorage) AddMessage(sessionID string, message *model.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}
	
	session.Messages = append(session.Messages, *message)
	return nil
}

func (m *MemoryStorage) GetMessages(sessionID string) ([]*model.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	
	messages := make([]*model.Message, len(session.Messages))
	for i, msg := range session.Messages {
		messages[i] = &msg
	}
	
	return messages, nil
}

// ✅ 约束2：更新单个消息渲染结果，严格验证会话ID
func (m *MemoryStorage) UpdateMessageRender(sessionID, messageID, htmlContent string, renderTime int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}
	
	// 找到并更新目标消息
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			// ✅ 约束2：验证消息确实属于目标会话
			if session.Messages[i].SessionID != sessionID {
				return ErrInvalidData
			}
			
			session.Messages[i].HTMLContent = htmlContent
			session.Messages[i].IsRendered = true
			session.Messages[i].RenderTime = renderTime
			
			return nil
		}
	}
	
	return ErrSessionNotFound // 消息未找到
}

// ✅ 约束2：批量更新渲染结果，按会话ID分组验证
func (m *MemoryStorage) UpdateMessagesRender(sessionID string, renders []model.RenderUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}
	
	// 创建消息ID到渲染信息的映射
	renderMap := make(map[string]model.RenderUpdate)
	for _, render := range renders {
		renderMap[render.MessageID] = render
	}
	
	// 批量更新消息
	for i := range session.Messages {
		if render, exists := renderMap[session.Messages[i].ID]; exists {
			// ✅ 约束2：验证消息确实属于目标会话
			if session.Messages[i].SessionID != sessionID {
				continue // 跳过不匹配的消息
			}
			
			session.Messages[i].HTMLContent = render.HTMLContent
			session.Messages[i].IsRendered = true
			session.Messages[i].RenderTime = render.RenderTime
		}
	}
	
	return nil
}

// ✅ 约束2：获取未渲染的消息，严格按会话ID过滤
func (m *MemoryStorage) GetPendingRenders(sessionID string) ([]*model.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	
	var pendingMessages []*model.Message
	for _, msg := range session.Messages {
		// ✅ 约束2：再次验证消息属于正确的会话
		if msg.SessionID != sessionID {
			continue // 跳过不匹配的消息
		}
		
		// 只返回assistant角色且未渲染的消息
		if msg.Role == "assistant" && !msg.IsRendered {
			msgCopy := msg // 创建副本避免指针问题
			pendingMessages = append(pendingMessages, &msgCopy)
		}
	}
	
	return pendingMessages, nil
}