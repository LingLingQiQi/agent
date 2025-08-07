package storage

import (
	"glata-backend/internal/model"
	"glata-backend/pkg/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type DiskStorage struct {
	dataDir   string
	mu        sync.RWMutex
	cache     map[string]*model.Session
	cacheSize int
}

type SessionIndex struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewDiskStorage(dataDir string, cacheSize int) *DiskStorage {
	return &DiskStorage{
		dataDir:   dataDir,
		cache:     make(map[string]*model.Session),
		cacheSize: cacheSize,
	}
}

func (d *DiskStorage) Init() error {
	if err := d.createDirectories(); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageInit, err)
	}
	
	if err := d.loadSessions(); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageInit, err)
	}
	
	logger.Info("Disk storage initialized successfully")
	return nil
}

func (d *DiskStorage) createDirectories() error {
	dirs := []string{
		d.dataDir,
		filepath.Join(d.dataDir, "sessions"),
		filepath.Join(d.dataDir, "messages"),
		filepath.Join(d.dataDir, "backup"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	
	return nil
}

func (d *DiskStorage) loadSessions() error {
	indexPath := filepath.Join(d.dataDir, "sessions.json")
	
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return d.saveSessionIndex([]*SessionIndex{})
	}
	
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	
	var indexes []*SessionIndex
	if err := json.Unmarshal(data, &indexes); err != nil {
		return err
	}
	
	for _, index := range indexes {
		if len(d.cache) >= d.cacheSize {
			break
		}
		
		session, err := d.loadSessionFromFile(index.ID)
		if err != nil {
			logger.Errorf("Failed to load session %s: %v", index.ID, err)
			continue
		}
		
		d.cache[index.ID] = session
	}
	
	return nil
}

func (d *DiskStorage) loadSessionFromFile(sessionID string) (*model.Session, error) {
	sessionPath := filepath.Join(d.dataDir, "sessions", sessionID+".json")
	
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}
	
	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	
	messages, err := d.loadMessagesFromFile(sessionID)
	if err != nil {
		logger.Errorf("Failed to load messages for session %s: %v", sessionID, err)
		messages = []model.Message{}
	}
	
	session.Messages = messages
	return &session, nil
}

func (d *DiskStorage) loadMessagesFromFile(sessionID string) ([]model.Message, error) {
	messagesPath := filepath.Join(d.dataDir, "messages", sessionID+".json")
	
	if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
		return []model.Message{}, nil
	}
	
	data, err := os.ReadFile(messagesPath)
	if err != nil {
		return nil, err
	}
	
	var messages []model.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	
	return messages, nil
}

func (d *DiskStorage) saveSessionIndex(indexes []*SessionIndex) error {
	indexPath := filepath.Join(d.dataDir, "sessions.json")
	tempPath := indexPath + ".tmp"
	
	data, err := json.MarshalIndent(indexes, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	
	return os.Rename(tempPath, indexPath)
}

func (d *DiskStorage) saveSessionToFile(session *model.Session) error {
	sessionPath := filepath.Join(d.dataDir, "sessions", session.ID+".json")
	tempPath := sessionPath + ".tmp"
	
	sessionData := *session
	sessionData.Messages = nil
	
	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	
	return os.Rename(tempPath, sessionPath)
}

func (d *DiskStorage) saveMessagesToFile(sessionID string, messages []model.Message) error {
	messagesPath := filepath.Join(d.dataDir, "messages", sessionID+".json")
	tempPath := messagesPath + ".tmp"
	
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	
	return os.Rename(tempPath, messagesPath)
}

func (d *DiskStorage) CreateSession(session *model.Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if err := d.saveSessionToFile(session); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.saveMessagesToFile(session.ID, session.Messages); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.updateSessionIndex(); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	d.cache[session.ID] = session
	d.evictCache()
	
	return nil
}

func (d *DiskStorage) GetSession(sessionID string) (*model.Session, error) {
	d.mu.RLock()
	if session, exists := d.cache[sessionID]; exists {
		d.mu.RUnlock()
		return session, nil
	}
	d.mu.RUnlock()
	
	session, err := d.loadSessionFromFile(sessionID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	d.mu.Lock()
	d.cache[sessionID] = session
	d.evictCache()
	d.mu.Unlock()
	
	return session, nil
}

func (d *DiskStorage) UpdateSession(session *model.Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if _, err := d.loadSessionFromFile(session.ID); err != nil {
		if os.IsNotExist(err) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.saveSessionToFile(session); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.saveMessagesToFile(session.ID, session.Messages); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.updateSessionIndex(); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	d.cache[session.ID] = session
	
	return nil
}

func (d *DiskStorage) DeleteSession(sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	sessionPath := filepath.Join(d.dataDir, "sessions", sessionID+".json")
	messagesPath := filepath.Join(d.dataDir, "messages", sessionID+".json")
	
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return ErrSessionNotFound
	}
	
	if err := os.Remove(sessionPath); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if _, err := os.Stat(messagesPath); err == nil {
		if err := os.Remove(messagesPath); err != nil {
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
	}
	
	delete(d.cache, sessionID)
	
	return d.updateSessionIndex()
}

func (d *DiskStorage) ListSessions() ([]*model.Session, error) {
	indexPath := filepath.Join(d.dataDir, "sessions.json")
	
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	var indexes []*SessionIndex
	if err := json.Unmarshal(data, &indexes); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidData, err)
	}
	
	sessions := make([]*model.Session, 0, len(indexes))
	for _, index := range indexes {
		session := &model.Session{
			ID:        index.ID,
			Title:     index.Title,
			CreatedAt: index.CreatedAt,
			UpdatedAt: index.UpdatedAt,
		}
		sessions = append(sessions, session)
	}
	
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	
	return sessions, nil
}

func (d *DiskStorage) AddMessage(sessionID string, message *model.Message) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	session, exists := d.cache[sessionID]
	if !exists {
		var err error
		session, err = d.loadSessionFromFile(sessionID)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrSessionNotFound
			}
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
		d.cache[sessionID] = session
	}
	
	session.Messages = append(session.Messages, *message)
	session.UpdatedAt = time.Now()
	
	if err := d.saveMessagesToFile(sessionID, session.Messages); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	if err := d.saveSessionToFile(session); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	return d.updateSessionIndex()
}

func (d *DiskStorage) GetMessages(sessionID string) ([]*model.Message, error) {
	session, err := d.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	
	messages := make([]*model.Message, len(session.Messages))
	for i, msg := range session.Messages {
		messages[i] = &msg
	}
	
	return messages, nil
}

func (d *DiskStorage) updateSessionIndex() error {
	sessionsDir := filepath.Join(d.dataDir, "sessions")
	
	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return err
	}
	
	var indexes []*SessionIndex
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		sessionID := file.Name()[:len(file.Name())-5]
		session, err := d.loadSessionFromFile(sessionID)
		if err != nil {
			logger.Errorf("Failed to load session %s for index update: %v", sessionID, err)
			continue
		}
		
		index := &SessionIndex{
			ID:        session.ID,
			Title:     session.Title,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
		}
		indexes = append(indexes, index)
	}
	
	return d.saveSessionIndex(indexes)
}

func (d *DiskStorage) evictCache() {
	if len(d.cache) <= d.cacheSize {
		return
	}
	
	type cacheEntry struct {
		id        string
		updatedAt time.Time
	}
	
	var entries []cacheEntry
	for id, session := range d.cache {
		entries = append(entries, cacheEntry{
			id:        id,
			updatedAt: session.UpdatedAt,
		})
	}
	
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].updatedAt.Before(entries[j].updatedAt)
	})
	
	toEvict := len(d.cache) - d.cacheSize
	for i := 0; i < toEvict; i++ {
		delete(d.cache, entries[i].id)
	}
}

func (d *DiskStorage) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.cache = make(map[string]*model.Session)
	return nil
}

func (d *DiskStorage) Backup() error {
	backupDir := filepath.Join(d.dataDir, "backup", fmt.Sprintf("backup_%d", time.Now().Unix()))
	
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	sourceDirs := []string{"sessions", "messages"}
	for _, dir := range sourceDirs {
		srcDir := filepath.Join(d.dataDir, dir)
		dstDir := filepath.Join(backupDir, dir)
		
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
		
		if err := d.copyDir(srcDir, dstDir); err != nil {
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
	}
	
	indexSrc := filepath.Join(d.dataDir, "sessions.json")
	indexDst := filepath.Join(backupDir, "sessions.json")
	if err := d.copyFile(indexSrc, indexDst); err != nil {
		return fmt.Errorf("%w: %v", ErrFileOperation, err)
	}
	
	logger.Infof("Backup completed: %s", backupDir)
	return nil
}

func (d *DiskStorage) copyDir(src, dst string) error {
	files, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		srcPath := filepath.Join(src, file.Name())
		dstPath := filepath.Join(dst, file.Name())
		
		if err := d.copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	
	return nil
}

func (d *DiskStorage) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	
	return os.WriteFile(dst, data, 0644)
}

// ✅ 约束2：更新单个消息渲染结果，严格验证会话ID
func (d *DiskStorage) UpdateMessageRender(sessionID, messageID, htmlContent string, renderTime int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	session, exists := d.cache[sessionID]
	if !exists {
		var err error
		session, err = d.loadSessionFromFile(sessionID)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrSessionNotFound
			}
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
		d.cache[sessionID] = session
	}
	
	// 找到并更新目标消息
	for i := range session.Messages {
		if session.Messages[i].ID == messageID {
			// ✅ 约束2：验证消息确实属于目标会话
			if session.Messages[i].SessionID != sessionID {
				return fmt.Errorf("message %s does not belong to session %s", messageID, sessionID)
			}
			
			session.Messages[i].HTMLContent = htmlContent
			session.Messages[i].IsRendered = true
			session.Messages[i].RenderTime = renderTime
			
			// 保存到文件
			if err := d.saveMessagesToFile(sessionID, session.Messages); err != nil {
				return fmt.Errorf("%w: %v", ErrFileOperation, err)
			}
			
			return nil
		}
	}
	
	return fmt.Errorf("message %s not found in session %s", messageID, sessionID)
}

// ✅ 约束2：批量更新渲染结果，按会话ID分组验证
func (d *DiskStorage) UpdateMessagesRender(sessionID string, renders []model.RenderUpdate) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	session, exists := d.cache[sessionID]
	if !exists {
		var err error
		session, err = d.loadSessionFromFile(sessionID)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrSessionNotFound
			}
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
		d.cache[sessionID] = session
	}
	
	// 创建消息ID到渲染信息的映射
	renderMap := make(map[string]model.RenderUpdate)
	for _, render := range renders {
		renderMap[render.MessageID] = render
	}
	
	// 批量更新消息
	updated := false
	for i := range session.Messages {
		if render, exists := renderMap[session.Messages[i].ID]; exists {
			// ✅ 约束2：验证消息确实属于目标会话
			if session.Messages[i].SessionID != sessionID {
				logger.Warnf("Message %s does not belong to session %s, skipping", session.Messages[i].ID, sessionID)
				continue
			}
			
			session.Messages[i].HTMLContent = render.HTMLContent
			session.Messages[i].IsRendered = true
			session.Messages[i].RenderTime = render.RenderTime
			updated = true
		}
	}
	
	if updated {
		// 保存到文件
		if err := d.saveMessagesToFile(sessionID, session.Messages); err != nil {
			return fmt.Errorf("%w: %v", ErrFileOperation, err)
		}
	}
	
	return nil
}

// ✅ 约束2：获取未渲染的消息，严格按会话ID过滤
func (d *DiskStorage) GetPendingRenders(sessionID string) ([]*model.Message, error) {
	messages, err := d.GetMessages(sessionID)
	if err != nil {
		return nil, err
	}
	
	var pendingMessages []*model.Message
	for _, msg := range messages {
		// ✅ 约束2：再次验证消息属于正确的会话
		if msg.SessionID != sessionID {
			logger.Warnf("Message %s does not belong to session %s, skipping", msg.ID, sessionID)
			continue
		}
		
		// 只返回assistant角色且未渲染的消息
		if msg.Role == "assistant" && !msg.IsRendered {
			pendingMessages = append(pendingMessages, msg)
		}
	}
	
	return pendingMessages, nil
}