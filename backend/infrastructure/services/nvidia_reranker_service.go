package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"
)

// NvidiaRerankerService implements cross-encoder reranking using NVIDIA NIM's
// nvidia/llama-nemotron-rerank-1b-v2 model.
// Unlike the Gemini prompt-based approach, this uses a dedicated ranking API
// that returns structured relevance scores directly.
type NvidiaRerankerService struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NVIDIA ranking API request/response types
type nvidiaRankingRequest struct {
	Model    string                `json:"model"`
	Query    nvidiaRankingQuery    `json:"query"`
	Passages []nvidiaRankingPassage `json:"passages"`
}

type nvidiaRankingQuery struct {
	Text string `json:"text"`
}

type nvidiaRankingPassage struct {
	Text string `json:"text"`
}

type nvidiaRankingResponse struct {
	Rankings []struct {
		Index    int     `json:"index"`
		Logit    float64 `json:"logit"`
	} `json:"rankings"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewNvidiaRerankerService creates a new reranker using NVIDIA NIM.
func NewNvidiaRerankerService(apiKey string) *NvidiaRerankerService {
	return &NvidiaRerankerService{
		apiKey:  apiKey,
		model:   "nvidia/llama-nemotron-rerank-1b-v2",
		baseURL: "https://integrate.api.nvidia.com/v1/ranking",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Rerank takes a query and candidate chunks, returns them reranked by relevance.
// Uses NVIDIA's dedicated ranking endpoint for structured relevance scoring.
func (rs *NvidiaRerankerService) Rerank(query string, candidates []*HybridResult, topK int) ([]*RerankResult, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// If very few candidates, skip reranking
	if len(candidates) <= 3 {
		results := make([]*RerankResult, len(candidates))
		for i, c := range candidates {
			results[i] = &RerankResult{
				Chunk:          c.Chunk,
				RelevanceScore: c.Score,
				OriginalRank:   i + 1,
				NewRank:        i + 1,
			}
		}
		return results, nil
	}

	// Build passages from candidates
	passages := make([]nvidiaRankingPassage, len(candidates))
	for i, c := range candidates {
		text := c.Chunk.Text
		if len(text) > 1000 {
			text = text[:1000] + "..."
		}
		passages[i] = nvidiaRankingPassage{Text: text}
	}

	reqBody := nvidiaRankingRequest{
		Model:    rs.model,
		Query:    nvidiaRankingQuery{Text: query},
		Passages: passages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nvidia rerank request: %w", err)
	}

	req, err := http.NewRequest("POST", rs.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create nvidia rerank request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rs.apiKey)

	resp, err := rs.httpClient.Do(req)
	if err != nil {
		log.Printf("NVIDIA reranking failed, falling back to original order: %v", err)
		return nvidiaFallbackResults(candidates, topK), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read nvidia rerank response, falling back: %v", err)
		return nvidiaFallbackResults(candidates, topK), nil
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("NVIDIA rerank API error %d: %s, falling back", resp.StatusCode, string(body))
		return nvidiaFallbackResults(candidates, topK), nil
	}

	var rankingResp nvidiaRankingResponse
	if err := json.Unmarshal(body, &rankingResp); err != nil {
		log.Printf("Failed to parse nvidia rerank response, falling back: %v", err)
		return nvidiaFallbackResults(candidates, topK), nil
	}

	if rankingResp.Error != nil {
		log.Printf("NVIDIA rerank API error: %s, falling back", rankingResp.Error.Message)
		return nvidiaFallbackResults(candidates, topK), nil
	}

	// Build results with scores from NVIDIA
	results := make([]*RerankResult, 0, len(candidates))
	scoreMap := make(map[int]float64)
	for _, r := range rankingResp.Rankings {
		scoreMap[r.Index] = r.Logit
	}

	for i, candidate := range candidates {
		score := candidate.Score // default fallback
		if s, ok := scoreMap[i]; ok {
			score = s
		}
		results = append(results, &RerankResult{
			Chunk:          candidate.Chunk,
			RelevanceScore: score,
			OriginalRank:   i + 1,
		})
	}

	// Sort by reranking score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// Assign new ranks and trim
	for i := range results {
		results[i].NewRank = i + 1
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func nvidiaFallbackResults(candidates []*HybridResult, topK int) []*RerankResult {
	results := make([]*RerankResult, 0, len(candidates))
	for i, c := range candidates {
		if i >= topK {
			break
		}
		results = append(results, &RerankResult{
			Chunk:          c.Chunk,
			RelevanceScore: c.Score,
			OriginalRank:   i + 1,
			NewRank:        i + 1,
		})
	}
	return results
}
