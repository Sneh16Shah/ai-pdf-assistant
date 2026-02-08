// Code generated manually for MVP - matches proto/document.proto
package proto

// Document represents a PDF document
type Document struct {
	Id        string   `json:"id"`
	Filename  string   `json:"filename"`
	Text      string   `json:"text"`
	Pages     int32    `json:"pages"`
	Chunks    []*Chunk `json:"chunks"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// Chunk represents a text chunk
type Chunk struct {
	Id         string   `json:"id"`
	Text       string   `json:"text"`
	ChunkIndex int32    `json:"chunk_index"`
	PageNumber int32    `json:"page_number"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

// UploadRequest represents a PDF upload request
type UploadRequest struct {
	Filename    string `json:"filename"`
	FileContent []byte `json:"file_content"`
}

// UploadResponse represents a PDF upload response
type UploadResponse struct {
	Status    Status    `json:"status"`
	Document  *Document `json:"document,omitempty"`
	SessionId string    `json:"session_id,omitempty"`
	Error     *Error    `json:"error,omitempty"`
}

// StatusRequest represents a document status request
type StatusRequest struct {
	DocumentId string `json:"document_id"`
}

// StatusResponse represents a document status response
type StatusResponse struct {
	Status   Status    `json:"status"`
	Document *Document `json:"document,omitempty"`
	Error    *Error    `json:"error,omitempty"`
}

// SummaryRequest represents a summary request
type SummaryRequest struct {
	SessionId  string `json:"session_id"`
	DocumentId string `json:"document_id,omitempty"`
}

// SummaryResponse represents a summary response
type SummaryResponse struct {
	Status       Status   `json:"status"`
	Summary      string   `json:"summary,omitempty"`
	KeyTakeaways []string `json:"key_takeaways,omitempty"`
	MainTopics   []string `json:"main_topics,omitempty"`
	Error        *Error   `json:"error,omitempty"`
}

