// Code generated manually for MVP - matches proto/chat.proto
package proto

// ChatMessage represents a chat message
type ChatMessage struct {
	Id        string `json:"id"`
	Role      string `json:"role"` // "user" or "assistant"
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// ChatSession represents a chat session
type ChatSession struct {
	Id           string         `json:"id"`
	DocumentId   string         `json:"document_id"`
	Document     *Document      `json:"document,omitempty"`
	Messages     []*ChatMessage `json:"messages"`
	CreatedAt    int64          `json:"created_at"`
	LastActivity int64          `json:"last_activity"`
}

// ChatRequest represents a chat message request
type ChatRequest struct {
	SessionId string `json:"session_id"`
	Message   string `json:"message"`
}

// ChatResponse represents a chat message response
type ChatResponse struct {
	Status         Status   `json:"status"`
	Response       string   `json:"response,omitempty"`
	SessionId      string   `json:"session_id,omitempty"`
	RelevantChunks []string `json:"relevant_chunks,omitempty"`
	AnswerFound    bool     `json:"answer_found"`
	Error          *Error   `json:"error,omitempty"`
}

// HistoryRequest represents a chat history request
type HistoryRequest struct {
	SessionId string `json:"session_id"`
}

// HistoryResponse represents a chat history response
type HistoryResponse struct {
	Status  Status       `json:"status"`
	Session *ChatSession `json:"session,omitempty"`
	Error   *Error       `json:"error,omitempty"`
}

// ClearSessionRequest represents a clear session request
type ClearSessionRequest struct {
	SessionId string `json:"session_id"`
}

// ClearSessionResponse represents a clear session response
type ClearSessionResponse struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

