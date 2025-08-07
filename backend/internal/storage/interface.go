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
	
	// 消息管理
	AddMessage(sessionID string, message *model.Message) error
	GetMessages(sessionID string) ([]*model.Message, error)
	
	// 存储管理
	Init() error
	Close() error
	Backup() error
}