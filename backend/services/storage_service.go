package services

import (
	"fmt"
	"sync"
	"time"
)

type StorageService struct {
	documents map[string]*PDFDocument
	sessions  map[string]*ChatSession
	mutex     sync.RWMutex
}

type ChatSession struct {
	ID          string        `json:"id"`
	PDFDocument *PDFDocument  `json:"pdf_document"`
	Messages    []ChatMessage `json:"messages"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

func NewStorageService() *StorageService {
	return &StorageService{
		documents: make(map[string]*PDFDocument),
		sessions:  make(map[string]*ChatSession),
	}
}

func (s *StorageService) StorePDF(doc *PDFDocument) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.documents[doc.ID] = doc
	return nil
}

func (s *StorageService) GetPDF(id string) (*PDFDocument, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	doc, exists := s.documents[id]
	if !exists {
		return nil, fmt.Errorf("PDF document not found: %s", id)
	}
	
	return doc, nil
}

func (s *StorageService) CreateSession(pdfDoc *PDFDocument) *ChatSession {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	session := &ChatSession{
		ID:          fmt.Sprintf("session_%d", time.Now().UnixNano()),
		PDFDocument: pdfDoc,
		Messages:    []ChatMessage{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	s.sessions[session.ID] = session
	return session
}

func (s *StorageService) GetSession(sessionID string) (*ChatSession, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	
	return session, nil
}

func (s *StorageService) AddMessageToSession(sessionID string, message ChatMessage) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	session.Messages = append(session.Messages, message)
	session.UpdatedAt = time.Now()
	
	return nil
}

func (s *StorageService) ClearSession(sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	session.Messages = []ChatMessage{}
	session.UpdatedAt = time.Now()
	
	return nil
}

func (s *StorageService) GetAllSessions() map[string]*ChatSession {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	// Return a copy of the sessions map
	sessions := make(map[string]*ChatSession)
	for k, v := range s.sessions {
		sessions[k] = v
	}
	
	return sessions
}