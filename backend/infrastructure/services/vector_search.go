package services

import (
	"fmt"
	"strings"
	"ai-pdf-assistant-backend/proto"
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

	scored := make([]scoredChunk, len(chunks))
	for i, chunk := range chunks {
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

		scored[i] = scoredChunk{chunk: chunk, score: score}
	}

	// Simple selection: return chunks with score > 0, sorted by score
	// For MVP, we'll just return top K chunks with matches
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
		builder.WriteString(fmt.Sprintf("[Chunk %d]\n", i+1))
		builder.WriteString(chunk.Text)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

