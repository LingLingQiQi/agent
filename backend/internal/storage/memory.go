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
