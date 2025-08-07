package storage

import (
	"glata-backend/internal/model"
)

type Storage interface {
	// 会话管理
	CreateSession(session *model.Session) error
	GetSession(sessionID string) (*model.Session, error)
	UpdateSession(session *model.Session) error
	DeleteSession(sessionID string) error
	ListSessions() ([]*model.Session, error)
	
	// 消息管理（扩展支持HTML）
	AddMessage(sessionID string, message *model.Message) error
	GetMessages(sessionID string) ([]*model.Message, error)
	UpdateMessageRender(sessionID, messageID, htmlContent string, renderTime int64) error
	UpdateMessagesRender(sessionID string, renders []model.RenderUpdate) error
	GetPendingRenders(sessionID string) ([]*model.Message, error)
	
	// 存储管理
	Init() error
	Close() error
	Backup() error
}