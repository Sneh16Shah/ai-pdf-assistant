package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// NvidiaEmbeddingService generates multimodal embeddings using NVIDIA NIM's
// nvidia/llama-nemotron-embed-vl-1b-v2 model.
// This model can embed both text and images in the same vector space,
// enabling true visual RAG for PDFs with diagrams and charts.
type NvidiaEmbeddingService struct {
	apiKey      string
	model       string
	baseURL     string
	httpClient  *http.Client
	rateLimiter chan struct{}
}

// NVIDIA embedding API request/response types
type nvidiaEmbedRequest struct {
	Input          interface{} `json:"input"`
	Model          string      `json:"model"`
	InputType      string      `json:"input_type"`
	EncodingFormat string      `json:"encoding_format"`
	Truncate       string      `json:"truncate"`
}

type nvidiaEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

const (
	// NvidiaEmbeddingDimension — llama-nemotron-embed-vl-1b-v2 produces 2048-dim vectors
	NvidiaEmbeddingDimension = 2048
	// NvidiaEmbeddingBatchSize — max texts per batch
	NvidiaEmbeddingBatchSize = 50
	// NvidiaEmbeddingMaxRetries — max retries on rate limit
	NvidiaEmbeddingMaxRetries = 3
)

// NewNvidiaEmbeddingService creates a new multimodal embedding service using NVIDIA NIM.
func NewNvidiaEmbeddingService(apiKey string) *NvidiaEmbeddingService {
	rateLimiter := make(chan struct{}, 10) // Max 10 concurrent requests
	for i := 0; i < 10; i++ {
		rateLimiter <- struct{}{}
	}

	return &NvidiaEmbeddingService{
		apiKey:  apiKey,
		model:   "nvidia/llama-nemotron-embed-vl-1b-v2",
		baseURL: "https://integrate.api.nvidia.com/v1/embeddings",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		rateLimiter: rateLimiter,
	}
}

// EmbedText generates an embedding for a single text string.
// Returns a normalized float32 vector.
func (es *NvidiaEmbeddingService) EmbedText(text string) ([]float32, error) {
	if text == "" {
		return make([]float32, NvidiaEmbeddingDimension), nil
	}

	// Truncate very long texts
	if len(text) > 8000 {
		text = text[:8000]
	}

	reqBody := nvidiaEmbedRequest{
		Input:          text,
		Model:          es.model,
		InputType:      "query",
		EncodingFormat: "float",
		Truncate:       "END",
	}

	var result nvidiaEmbedResponse
	if err := es.doRequest(reqBody, &result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("nvidia embedding API error: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned from NVIDIA API")
	}

	return normalizeVector(toFloat32(result.Data[0].Embedding)), nil
}

// EmbedBatch generates embeddings for multiple texts efficiently.
func (es *NvidiaEmbeddingService) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	results := make([][]float32, len(texts))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for batchStart := 0; batchStart < len(texts); batchStart += NvidiaEmbeddingBatchSize {
		batchEnd := batchStart + NvidiaEmbeddingBatchSize
		if batchEnd > len(texts) {
			batchEnd = len(texts)
		}
		batch := texts[batchStart:batchEnd]
		offset := batchStart

		wg.Add(1)
		go func(batch []string, offset int) {
			defer wg.Done()

			<-es.rateLimiter
			defer func() { es.rateLimiter <- struct{}{} }()

			// Truncate individual texts
			truncated := make([]string, len(batch))
			for i, t := range batch {
				if len(t) > 8000 {
					truncated[i] = t[:8000]
				} else {
					truncated[i] = t
				}
			}

			reqBody := nvidiaEmbedRequest{
				Input:          truncated,
				Model:          es.model,
				InputType:      "passage",
				EncodingFormat: "float",
				Truncate:       "END",
			}

			var result nvidiaEmbedResponse
			if err := es.doRequest(reqBody, &result); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, d := range result.Data {
				if d.Index < len(batch) {
					results[offset+d.Index] = normalizeVector(toFloat32(d.Embedding))
				}
			}
		}(batch, offset)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// EmbedImage generates an embedding for a page image.
// The multimodal model embeds images in the same vector space as text,
// enabling visual retrieval.
func (es *NvidiaEmbeddingService) EmbedImage(imageData []byte) ([]float32, error) {
	if len(imageData) == 0 {
		return make([]float32, NvidiaEmbeddingDimension), nil
	}

	// Encode image to base64 data URL
	b64Image := base64.StdEncoding.EncodeToString(imageData)
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64Image)

	reqBody := nvidiaEmbedRequest{
		Input:          imageURL,
		Model:          es.model,
		InputType:      "passage",
		EncodingFormat: "float",
		Truncate:       "END",
	}

	var result nvidiaEmbedResponse
	if err := es.doRequest(reqBody, &result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("nvidia image embedding API error: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no image embedding returned from NVIDIA API")
	}

	return normalizeVector(toFloat32(result.Data[0].Embedding)), nil
}

// doRequest makes an HTTP POST to the NVIDIA embedding API with retry logic.
func (es *NvidiaEmbeddingService) doRequest(reqBody interface{}, result interface{}) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal nvidia embedding request: %w", err)
	}

	for attempt := 0; attempt < NvidiaEmbeddingMaxRetries; attempt++ {
		req, err := http.NewRequest("POST", es.baseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to create nvidia embedding request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+es.apiKey)

		resp, err := es.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("nvidia embedding API request failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 429 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoff)
			continue
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("nvidia embedding API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse nvidia embedding response: %w", err)
		}

		return nil
	}

	return fmt.Errorf("nvidia embedding API failed after %d retries", NvidiaEmbeddingMaxRetries)
}
