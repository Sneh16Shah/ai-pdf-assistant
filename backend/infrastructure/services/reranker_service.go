package services

import (
	"ai-pdf-assistant-backend/proto"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"
)

// RerankerService implements cross-encoder reranking using Gemini.
// After initial hybrid retrieval returns ~20 candidates, the reranker
// scores each (query, chunk) pair for fine-grained relevance.
//
// Why rerank? Bi-encoder retrieval (BM25 + embeddings) is fast but approximate.
// Cross-encoders see query AND document together, enabling deeper understanding
// like negation, nuance, and multi-hop reasoning. This is the standard
// retrieve-then-rerank pattern used by Cohere Rerank, ColBERT, etc.
type RerankerService struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// RerankResult holds a chunk with its reranking score
type RerankResult struct {
	Chunk          *proto.Chunk
	RelevanceScore float64
	OriginalRank   int
	NewRank        int
}

// NewRerankerService creates a new cross-encoder reranker using Gemini
func NewRerankerService(apiKey string) *RerankerService {
	return &RerankerService{
		apiKey: apiKey,
		model:  "gemini-2.5-flash-lite-preview-06-17",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Rerank takes a query and candidate chunks, returns them reranked by relevance.
// Uses Gemini as a cross-encoder: sends query + each chunk and asks for a relevance score.
// For efficiency, batches all candidates in a single prompt.
func (rs *RerankerService) Rerank(query string, candidates []*HybridResult, topK int) ([]*RerankResult, error) {
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

	// Build reranking prompt — ask Gemini to score each passage
	prompt := buildRerankPrompt(query, candidates)

	scores, err := rs.callGeminiForScores(prompt)
	if err != nil {
		log.Printf("Reranking failed, falling back to original order: %v", err)
		// Fallback: return original order
		return fallbackResults(candidates, topK), nil
	}

	// Build results with new scores
	results := make([]*RerankResult, 0, len(candidates))
	for i, candidate := range candidates {
		score := candidate.Score // default
		if i < len(scores) {
			score = scores[i]
		}
		results = append(results, &RerankResult{
			Chunk:          candidate.Chunk,
			RelevanceScore: score,
			OriginalRank:   i + 1,
		})
	}

	// Sort by reranking score
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

func buildRerankPrompt(query string, candidates []*HybridResult) string {
	var buf bytes.Buffer
	buf.WriteString("You are a relevance scoring system. Given a query and a list of text passages, ")
	buf.WriteString("score each passage's relevance to the query on a scale of 0.0 to 1.0.\n\n")
	buf.WriteString("IMPORTANT: Respond ONLY with a JSON array of numbers, one score per passage.\n")
	buf.WriteString("Example response for 3 passages: [0.95, 0.23, 0.78]\n\n")
	buf.WriteString(fmt.Sprintf("Query: %s\n\n", query))

	for i, c := range candidates {
		text := c.Chunk.Text
		if len(text) > 500 {
			text = text[:500] + "..."
		}
		buf.WriteString(fmt.Sprintf("Passage %d (Page %d):\n%s\n\n", i+1, c.Chunk.PageNumber, text))
	}

	buf.WriteString("Relevance scores (JSON array):")
	return buf.String()
}

func (rs *RerankerService) callGeminiForScores(prompt string) ([]float64, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		rs.model, rs.apiKey,
	)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.0,
			"maxOutputTokens": 500,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("rerank API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse Gemini response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse rerank response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty rerank response")
	}

	// Extract JSON array from response text
	text := geminiResp.Candidates[0].Content.Parts[0].Text
	return parseScoresFromText(text)
}

func parseScoresFromText(text string) ([]float64, error) {
	// Find JSON array in the response
	start := -1
	end := -1
	for i, c := range text {
		if c == '[' {
			start = i
		}
		if c == ']' && start >= 0 {
			end = i + 1
			break
		}
	}

	if start < 0 || end < 0 {
		return nil, fmt.Errorf("no JSON array found in reranker response: %s", text)
	}

	var scores []float64
	if err := json.Unmarshal([]byte(text[start:end]), &scores); err != nil {
		return nil, fmt.Errorf("failed to parse scores: %w", err)
	}

	return scores, nil
}

func fallbackResults(candidates []*HybridResult, topK int) []*RerankResult {
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
