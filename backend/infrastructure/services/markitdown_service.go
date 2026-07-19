package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// MarkItDownService is an HTTP client for the Python MarkItDown sidecar.
// It converts documents (PDF/DOCX/etc.) to structured Markdown that preserves
// headings, tables, and list structure — richer than the basic text extractor
// used by the Standard pipeline.
//
// The service is optional: when MARKITDOWN_URL is unset, no instance is
// constructed and uploads fall back to the basic parser for all pipelines.
type MarkItDownService struct {
	baseURL    string
	httpClient *http.Client
}

// MarkItDownResult holds the converted markdown and any metadata the sidecar returned.
type MarkItDownResult struct {
	Markdown string
	Title    string
	Filename string
}

type markitdownResponse struct {
	Markdown string `json:"markdown"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
}

type markitdownErrorResponse struct {
	Error string `json:"error"`
}

// NewMarkItDownService creates a client pointing at the sidecar at baseURL.
// baseURL should not have a trailing slash, e.g. "http://markitdown:8000".
func NewMarkItDownService(baseURL string) *MarkItDownService {
	return &MarkItDownService{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // large PDFs can take a while
		},
	}
}

// IsAvailable pings the sidecar /health endpoint. Returns false on any error
// so callers can decide whether to attempt a conversion.
func (m *MarkItDownService) IsAvailable() bool {
	if m == nil || m.baseURL == "" {
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(m.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ConvertFile sends the file at filePath to the sidecar and returns its Markdown.
// Any error (sidecar down, non-200, parse failure) is returned to the caller,
// which is expected to log and fall back to the basic parser.
func (m *MarkItDownService) ConvertFile(filePath string) (*MarkItDownResult, error) {
	if m == nil || m.baseURL == "" {
		return nil, fmt.Errorf("markitdown service not configured")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for markitdown: %w", err)
	}
	defer file.Close()

	// Build multipart form: field name "file" matches app.py's UploadFile param.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	filename := filepath.Base(filePath)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart form: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file into multipart form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", m.baseURL+"/convert", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to build markitdown request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("markitdown request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Try to surface the sidecar's error message if it sent JSON
		var errResp markitdownErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("markitdown error (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("markitdown error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var parsed markitdownResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse markitdown response: %w", err)
	}

	if parsed.Markdown == "" {
		return nil, fmt.Errorf("markitdown returned empty markdown")
	}

	return &MarkItDownResult{
		Markdown: parsed.Markdown,
		Title:    parsed.Title,
		Filename: parsed.Filename,
	}, nil
}
