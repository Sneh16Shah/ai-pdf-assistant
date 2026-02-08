package repositories

import (
	"fmt"
	"sync"
	"time"

	"ai-pdf-assistant-backend/proto"
	"github.com/google/uuid"
)

// SessionRepository handles chat session storage in memory
type SessionRepository struct {
	sessions map[string]*proto.ChatSession
	mutex    sync.RWMutex
}

// NewSessionRepository creates a new in-memory session repository
func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		sessions: make(map[string]*proto.ChatSession),
	}
}

// Create creates a new chat session
func (r *SessionRepository) Create(documentID string, document *proto.Document) (*proto.ChatSession, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session := &proto.ChatSession{
		Id:        uuid.New().String(),
		DocumentId: documentID,
		Document:  document,
		Messages:  []*proto.ChatMessage{},
		CreatedAt: time.Now().Unix(),
		LastActivity: time.Now().Unix(),
	}

	r.sessions[session.Id] = session
	return session, nil
}

// Get retrieves a session by ID
func (r *SessionRepository) Get(id string) (*proto.ChatSession, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	session, exists := r.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// AddMessage adds a message to a session
func (r *SessionRepository) AddMessage(sessionID string, message *proto.ChatMessage) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if message.Id == "" {
		message.Id = uuid.New().String()
	}
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().Unix()
	}

	session.Messages = append(session.Messages, message)
	session.LastActivity = time.Now().Unix()

	return nil
}

// ClearMessages clears all messages in a session
func (r *SessionRepository) ClearMessages(sessionID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Messages = []*proto.ChatMessage{}
	session.LastActivity = time.Now().Unix()

	return nil
}

// Delete removes a session
func (r *SessionRepository) Delete(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.sessions[id]; !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(r.sessions, id)
	return nil
}

// CleanupInactive removes sessions inactive for more than specified duration
func (r *SessionRepository) CleanupInactive(duration time.Duration) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now().Unix()
	threshold := now - int64(duration.Seconds())
	count := 0

	for id, session := range r.sessions {
		if session.LastActivity < threshold {
			delete(r.sessions, id)
			count++
		}
	}

	return count
}

