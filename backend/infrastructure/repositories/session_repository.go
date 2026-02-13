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
		Id:           uuid.New().String(),
		DocumentId:   documentID,
		Document:     document,
		Documents:    []*proto.Document{document}, // Initialize with first document
		Messages:     []*proto.ChatMessage{},
		CreatedAt:    time.Now().Unix(),
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

// AddDocument adds a document to an existing session
func (r *SessionRepository) AddDocument(sessionID string, document *proto.Document) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if document already exists
	for _, doc := range session.Documents {
		if doc.Id == document.Id {
			return fmt.Errorf("document already exists in session: %s", document.Id)
		}
	}

	session.Documents = append(session.Documents, document)
	session.LastActivity = time.Now().Unix()

	return nil
}

// GetDocuments returns all documents in a session
func (r *SessionRepository) GetDocuments(sessionID string) ([]*proto.Document, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session.Documents, nil
}

// RemoveDocument removes a document from a session
func (r *SessionRepository) RemoveDocument(sessionID string, documentID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Find and remove document
	for i, doc := range session.Documents {
		if doc.Id == documentID {
			session.Documents = append(session.Documents[:i], session.Documents[i+1:]...)
			session.LastActivity = time.Now().Unix()
			return nil
		}
	}

	return fmt.Errorf("document not found in session: %s", documentID)
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
