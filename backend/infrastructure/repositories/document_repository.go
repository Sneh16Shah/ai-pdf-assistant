package repositories

import (
	"fmt"
	"sync"
	"time"

	"ai-pdf-assistant-backend/proto"
	"github.com/google/uuid"
)

// DocumentRepository handles document storage in memory
type DocumentRepository struct {
	documents map[string]*proto.Document
	mutex     sync.RWMutex
}

// NewDocumentRepository creates a new in-memory document repository
func NewDocumentRepository() *DocumentRepository {
	return &DocumentRepository{
		documents: make(map[string]*proto.Document),
	}
}

// Store stores a document
func (r *DocumentRepository) Store(doc *proto.Document) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if doc.Id == "" {
		doc.Id = uuid.New().String()
	}
	
	now := time.Now().Unix()
	if doc.CreatedAt == 0 {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	r.documents[doc.Id] = doc
	return nil
}

// Get retrieves a document by ID
func (r *DocumentRepository) Get(id string) (*proto.Document, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	doc, exists := r.documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	return doc, nil
}

// Delete removes a document
func (r *DocumentRepository) Delete(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.documents[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	delete(r.documents, id)
	return nil
}

// List returns all documents (for cleanup purposes)
func (r *DocumentRepository) List() []*proto.Document {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	docs := make([]*proto.Document, 0, len(r.documents))
	for _, doc := range r.documents {
		docs = append(docs, doc)
	}

	return docs
}

