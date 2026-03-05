package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

// EmbeddingService generates text embeddings using Gemini's text-embedding-004 model.
// This is the foundation of the vector search half of hybrid retrieval.
type EmbeddingService struct {
	apiKey     string
	model      string
	httpClient *http.Client
	// Rate limiting: Gemini free tier = 1500 RPM for embeddings
	rateLimiter chan struct{}
}

// EmbeddingResponse represents the Gemini embedding API response
type embeddingAPIResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

type batchEmbeddingAPIResponse struct {
	Embeddings []struct {
		Values []float64 `json:"values"`
	} `json:"embeddings"`
}

type embeddingRequest struct {
	Model   string              `json:"model"`
	Content embeddingReqContent `json:"content"`
}

type embeddingReqContent struct {
	Parts []embeddingReqPart `json:"parts"`
}

type embeddingReqPart struct {
	Text string `json:"text"`
}

type batchEmbeddingRequest struct {
	Requests []embeddingRequest `json:"requests"`
}

const (
	// Gemini text-embedding-004 produces 768-dimensional vectors
	EmbeddingDimension = 768
	// Max texts per batch request
	EmbeddingBatchSize = 100
	// Max retries on rate limit
	EmbeddingMaxRetries = 3
)

// NewEmbeddingService creates a new embedding service using Gemini
func NewEmbeddingService(apiKey string) *EmbeddingService {
	rateLimiter := make(chan struct{}, 50) // Max 50 concurrent requests
	for i := 0; i < 50; i++ {
		rateLimiter <- struct{}{}
	}

	return &EmbeddingService{
		apiKey: apiKey,
		model:  "text-embedding-004",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: rateLimiter,
	}
}

// EmbedText generates an embedding for a single text string.
// Returns a normalized float32 vector of dimension 768.
func (es *EmbeddingService) EmbedText(text string) ([]float32, error) {
	if text == "" {
		return make([]float32, EmbeddingDimension), nil
	}

	// Truncate very long texts (embedding model has ~2048 token limit)
	if len(text) > 8000 {
		text = text[:8000]
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
		es.model, es.apiKey,
	)

	reqBody := map[string]interface{}{
		"model": fmt.Sprintf("models/%s", es.model),
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var result embeddingAPIResponse
	err = es.doRequestWithRetry(url, bodyBytes, &result)
	if err != nil {
		return nil, err
	}

	// Convert float64 → float32 and normalize
	return normalizeVector(toFloat32(result.Embedding.Values)), nil
}

// EmbedBatch generates embeddings for multiple texts efficiently.
// Uses batching to minimize API calls.
func (es *EmbeddingService) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	results := make([][]float32, len(texts))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	// Process in batches
	for batchStart := 0; batchStart < len(texts); batchStart += EmbeddingBatchSize {
		batchEnd := batchStart + EmbeddingBatchSize
		if batchEnd > len(texts) {
			batchEnd = len(texts)
		}
		batch := texts[batchStart:batchEnd]
		offset := batchStart

		wg.Add(1)
		go func(batch []string, offset int) {
			defer wg.Done()

			// Rate limit
			<-es.rateLimiter
			defer func() { es.rateLimiter <- struct{}{} }()

			embeddings, err := es.embedBatchRequest(batch)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			for i, emb := range embeddings {
				results[offset+i] = emb
			}
		}(batch, offset)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

func (es *EmbeddingService) embedBatchRequest(texts []string) ([][]float32, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:batchEmbedContents?key=%s",
		es.model, es.apiKey,
	)

	requests := make([]map[string]interface{}, len(texts))
	for i, text := range texts {
		if len(text) > 8000 {
			text = text[:8000]
		}
		requests[i] = map[string]interface{}{
			"model": fmt.Sprintf("models/%s", es.model),
			"content": map[string]interface{}{
				"parts": []map[string]string{
					{"text": text},
				},
			},
		}
	}

	reqBody := map[string]interface{}{
		"requests": requests,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	var result batchEmbeddingAPIResponse
	err = es.doRequestWithRetry(url, bodyBytes, &result)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = normalizeVector(toFloat32(emb.Values))
	}

	return embeddings, nil
}

func (es *EmbeddingService) doRequestWithRetry(url string, body []byte, result interface{}) error {
	for attempt := 0; attempt < EmbeddingMaxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := es.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("embedding API request failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 429 {
			// Rate limited — exponential backoff
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			log.Printf("Embedding API rate limited, retrying in %v (attempt %d/%d)", backoff, attempt+1, EmbeddingMaxRetries)
			time.Sleep(backoff)
			continue
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse embedding response: %w", err)
		}

		return nil
	}

	return fmt.Errorf("embedding API failed after %d retries", EmbeddingMaxRetries)
}

// toFloat32 converts a float64 slice to float32
func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

// normalizeVector L2-normalizes a vector (required for cosine similarity via dot product)
func normalizeVector(v []float32) []float32 {
	var norm float64
	for _, val := range v {
		norm += float64(val) * float64(val)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = float32(float64(val) / norm)
	}
	return result
}
