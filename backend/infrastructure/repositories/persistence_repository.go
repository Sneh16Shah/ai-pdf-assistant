package repositories

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"ai-pdf-assistant-backend/database"
)

// DBSession represents a session stored in the database
type DBSession struct {
	ID           string       `json:"id"`
	UserID       string       `json:"user_id"`
	Title        string       `json:"title"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActivity time.Time    `json:"last_activity"`
	Documents    []DBDocument `json:"documents,omitempty"`
	Messages     []DBMessage  `json:"messages,omitempty"`
}

// DBDocument represents a document stored in the database
type DBDocument struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path,omitempty"`
	Pages       int       `json:"pages"`
	ChunksCount int       `json:"chunks_count"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// DBMessage represents a chat message stored in the database
type DBMessage struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Citations json.RawMessage `json:"citations,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// PersistenceRepository handles database persistence for sessions, documents, and messages
type PersistenceRepository struct{}

// NewPersistenceRepository creates a new persistence repository
func NewPersistenceRepository() *PersistenceRepository {
	return &PersistenceRepository{}
}

// SaveSession saves or updates a session in the database
func (r *PersistenceRepository) SaveSession(session *DBSession) error {
	if !database.IsConnected() {
		return nil
	}

	_, err := database.DB.Exec(`
		INSERT INTO sessions (id, user_id, title, created_at, last_activity)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			last_activity = EXCLUDED.last_activity
	`, session.ID, session.UserID, session.Title, session.CreatedAt, session.LastActivity)

	return err
}

// SaveDocument saves a document record to the database
func (r *PersistenceRepository) SaveDocument(doc *DBDocument) error {
	if !database.IsConnected() {
		return nil
	}

	_, err := database.DB.Exec(`
		INSERT INTO documents (id, session_id, filename, file_path, pages, chunks_count, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, doc.ID, doc.SessionID, doc.Filename, doc.FilePath, doc.Pages, doc.ChunksCount, doc.UploadedAt)

	return err
}

// SaveMessage saves a chat message to the database
func (r *PersistenceRepository) SaveMessage(msg *DBMessage) error {
	if !database.IsConnected() {
		return nil
	}

	_, err := database.DB.Exec(`
		INSERT INTO chat_messages (id, session_id, role, content, citations, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Citations, msg.CreatedAt)

	return err
}

// GetUserSessions returns all sessions for a user, ordered by last activity
func (r *PersistenceRepository) GetUserSessions(userID string) ([]DBSession, error) {
	if !database.IsConnected() {
		return nil, nil
	}

	rows, err := database.DB.Query(`
		SELECT s.id, s.user_id, s.title, s.created_at, s.last_activity
		FROM sessions s
		WHERE s.user_id = $1
		ORDER BY s.last_activity DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []DBSession
	for rows.Next() {
		var s DBSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.LastActivity); err != nil {
			log.Printf("Error scanning session: %v", err)
			continue
		}

		// Get documents for this session
		docs, err := r.GetSessionDocuments(s.ID)
		if err == nil {
			s.Documents = docs
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// GetSessionDocuments returns all documents for a session
func (r *PersistenceRepository) GetSessionDocuments(sessionID string) ([]DBDocument, error) {
	if !database.IsConnected() {
		return nil, nil
	}

	rows, err := database.DB.Query(`
		SELECT id, session_id, filename, COALESCE(file_path, ''), pages, chunks_count, uploaded_at
		FROM documents WHERE session_id = $1
		ORDER BY uploaded_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []DBDocument
	for rows.Next() {
		var d DBDocument
		if err := rows.Scan(&d.ID, &d.SessionID, &d.Filename, &d.FilePath, &d.Pages, &d.ChunksCount, &d.UploadedAt); err != nil {
			continue
		}
		docs = append(docs, d)
	}

	return docs, nil
}

// GetSessionMessages returns all chat messages for a session
func (r *PersistenceRepository) GetSessionMessages(sessionID string) ([]DBMessage, error) {
	if !database.IsConnected() {
		return nil, nil
	}

	rows, err := database.DB.Query(`
		SELECT id, session_id, role, content, citations, created_at
		FROM chat_messages WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DBMessage
	for rows.Next() {
		var m DBMessage
		var citations sql.NullString
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &citations, &m.CreatedAt); err != nil {
			continue
		}
		if citations.Valid {
			m.Citations = json.RawMessage(citations.String)
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// DeleteSession deletes a session and all related data (cascade)
func (r *PersistenceRepository) DeleteSession(sessionID string) error {
	if !database.IsConnected() {
		return nil
	}

	_, err := database.DB.Exec(`DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

// DeleteDocument deletes a document from the database
func (r *PersistenceRepository) DeleteDocument(documentID string) error {
	if !database.IsConnected() {
		return nil
	}

	_, err := database.DB.Exec(`DELETE FROM documents WHERE id = $1`, documentID)
	return err
}
