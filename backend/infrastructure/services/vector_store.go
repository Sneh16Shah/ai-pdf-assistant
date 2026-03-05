package services

import (
	"ai-pdf-assistant-backend/proto"
	"math"
	"sort"
	"sync"
)

// VectorStore is an in-memory vector store for chunk embeddings.
// Supports efficient cosine similarity search (dot product on normalized vectors).
// Designed with a clean interface so it can be swapped for Pinecone/Weaviate/Qdrant later.
type VectorStore struct {
	mu         sync.RWMutex
	entries    []vectorEntry
	dimensions int
}

type vectorEntry struct {
	ChunkID    string
	PageNumber int32
	Embedding  []float32
}

// ScoredChunk represents a chunk with its similarity score
type ScoredChunk struct {
	Chunk *proto.Chunk
	Score float64
}

// NewVectorStore creates a new in-memory vector store
func NewVectorStore() *VectorStore {
	return &VectorStore{
		dimensions: EmbeddingDimension,
	}
}

// IndexChunks stores chunk embeddings in the vector store.
// Called once after embedding generation during PDF upload.
func (vs *VectorStore) IndexChunks(chunks []*proto.Chunk, embeddings [][]float32) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.entries = make([]vectorEntry, 0, len(chunks))
	for i, chunk := range chunks {
		if i >= len(embeddings) || embeddings[i] == nil {
			continue
		}
		vs.entries = append(vs.entries, vectorEntry{
			ChunkID:    chunk.Id,
			PageNumber: chunk.PageNumber,
			Embedding:  embeddings[i],
		})
	}
}

// Search finds the top-K most similar chunks to the query embedding.
// Uses dot product on normalized vectors (equivalent to cosine similarity).
func (vs *VectorStore) Search(queryEmbedding []float32, topK int) []ScoredResult {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.entries) == 0 || len(queryEmbedding) == 0 {
		return nil
	}

	type scored struct {
		index int
		score float64
	}

	scores := make([]scored, len(vs.entries))
	for i, entry := range vs.entries {
		scores[i] = scored{
			index: i,
			score: dotProduct(queryEmbedding, entry.Embedding),
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	limit := topK
	if limit > len(scores) {
		limit = len(scores)
	}

	results := make([]ScoredResult, limit)
	for i := 0; i < limit; i++ {
		entry := vs.entries[scores[i].index]
		results[i] = ScoredResult{
			ChunkID:    entry.ChunkID,
			PageNumber: entry.PageNumber,
			Score:      scores[i].score,
		}
	}

	return results
}

// ScoredResult represents a search result with chunk metadata and score
type ScoredResult struct {
	ChunkID    string
	PageNumber int32
	Score      float64
}

// Size returns the number of indexed entries
func (vs *VectorStore) Size() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.entries)
}

// Clear removes all entries
func (vs *VectorStore) Clear() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.entries = nil
}

// dotProduct computes the dot product of two float32 vectors.
// For normalized vectors, this equals cosine similarity.
func dotProduct(a, b []float32) float64 {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	var sum float64
	for i := 0; i < minLen; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// CosineSimilarity computes cosine similarity between two vectors (non-normalized)
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	var dot, normA, normB float64
	for i := 0; i < minLen; i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
