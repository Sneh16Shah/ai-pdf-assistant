package services

import (
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"sort"
	"strings"
)

// VectorSearch provides simple text-based similarity search
// For MVP, we use keyword matching instead of real embeddings
type VectorSearch struct {
}

// NewVectorSearch creates a new vector search service
func NewVectorSearch() *VectorSearch {
	return &VectorSearch{}
}

// FindRelevantChunks finds chunks most relevant to the query
// Uses simple keyword matching for MVP (can be upgraded to real embeddings later)
func (v *VectorSearch) FindRelevantChunks(chunks []*proto.Chunk, query string, topK int) []*proto.Chunk {
	if len(chunks) == 0 {
		return []*proto.Chunk{}
	}

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// Score each chunk based on keyword matches
	type scoredChunk struct {
		chunk *proto.Chunk
		score int
	}

	scored := make([]scoredChunk, 0, len(chunks))
	for _, chunk := range chunks {
		chunkLower := strings.ToLower(chunk.Text)
		score := 0

		// Count keyword matches
		for _, word := range queryWords {
			if strings.Contains(chunkLower, word) {
				score++
			}
		}

		// Bonus for exact phrase match
		if strings.Contains(chunkLower, queryLower) {
			score += 5
		}

		scored = append(scored, scoredChunk{chunk: chunk, score: score})
	}

	// Sort by score descending so best matches come first
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top K chunks with matches
	relevant := make([]*proto.Chunk, 0, topK)
	for _, s := range scored {
		if s.score > 0 {
			relevant = append(relevant, s.chunk)
			if len(relevant) >= topK {
				break
			}
		}
	}

	// If no matches, return first few chunks as fallback
	if len(relevant) == 0 && len(chunks) > 0 {
		maxReturn := topK
		if len(chunks) < maxReturn {
			maxReturn = len(chunks)
		}
		return chunks[:maxReturn]
	}

	return relevant
}

// BuildContext builds a context string from relevant chunks
func (v *VectorSearch) BuildContext(chunks []*proto.Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Document Context:\n\n")

	for i, chunk := range chunks {
		pageNum := chunk.PageNumber
		if pageNum == 0 {
			pageNum = 1 // Default to page 1 if not set
		}
		builder.WriteString(fmt.Sprintf("[Chunk %d - Page %d]\n", i+1, pageNum))
		builder.WriteString(chunk.Text)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// Citation represents a page reference for a chunk
type Citation struct {
	Page int32  `json:"page"`
	Text string `json:"text"`
}

// GetCitations extracts unique page citations from chunks
func (v *VectorSearch) GetCitations(chunks []*proto.Chunk) []Citation {
	if len(chunks) == 0 {
		return []Citation{}
	}

	// Use a map to collect unique pages with sample text
	pageMap := make(map[int32]string)
	for _, chunk := range chunks {
		pageNum := chunk.PageNumber
		if pageNum == 0 {
			pageNum = 1
		}
		if _, exists := pageMap[pageNum]; !exists {
			// Store a preview of the text (max 100 chars)
			text := chunk.Text
			if len(text) > 100 {
				text = text[:100] + "..."
			}
			pageMap[pageNum] = text
		}
	}

	// Convert map to slice
	citations := make([]Citation, 0, len(pageMap))
	for page, text := range pageMap {
		citations = append(citations, Citation{Page: page, Text: text})
	}

	return citations
}
