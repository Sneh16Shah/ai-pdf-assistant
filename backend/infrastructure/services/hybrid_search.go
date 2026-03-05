package services

import (
	"ai-pdf-assistant-backend/proto"
	"sort"
)

// HybridSearch combines BM25 (lexical) and Vector (semantic) retrieval
// using Reciprocal Rank Fusion (RRF). This is the standard hybrid retrieval
// pattern used by Elasticsearch, Pinecone, Weaviate, etc.
//
// Why hybrid? BM25 excels at exact keyword matches (e.g., "HIPAA compliance"),
// while vector search captures semantic meaning (e.g., "healthcare regulations").
// RRF merges their rankings without needing to normalize scores.
type HybridSearch struct {
	bm25        *BM25Search
	vectorStore *VectorStore
	embedder    *EmbeddingService
	// RRF constant — standard value is 60
	rrrK int
}

// HybridResult holds a chunk with its combined score and source info
type HybridResult struct {
	Chunk      *proto.Chunk
	Score      float64
	BM25Rank   int // 0 if not in BM25 results
	VectorRank int // 0 if not in vector results
	FromBM25   bool
	FromVector bool
}

// HybridStats tracks retrieval statistics for comparison
type HybridStats struct {
	BM25Candidates     int    `json:"bm25_candidates"`
	VectorCandidates   int    `json:"vector_candidates"`
	OverlapCount       int    `json:"overlap_count"` // Chunks found by both
	FinalCount         int    `json:"final_count"`
	VectorSearchTimeMs int64  `json:"vector_search_time_ms"`
	BM25SearchTimeMs   int64  `json:"bm25_search_time_ms"`
	FusionMethod       string `json:"fusion_method"`
}

// NewHybridSearch creates a new hybrid search combining BM25 + Vector
func NewHybridSearch(bm25 *BM25Search, vectorStore *VectorStore, embedder *EmbeddingService) *HybridSearch {
	return &HybridSearch{
		bm25:        bm25,
		vectorStore: vectorStore,
		embedder:    embedder,
		rrrK:        60, // Standard RRF constant
	}
}

// Search performs hybrid retrieval: BM25 + Vector search fused via RRF.
// Returns top-K results ranked by combined score.
func (hs *HybridSearch) Search(
	allChunks []*proto.Chunk,
	query string,
	topK int,
) ([]*HybridResult, *HybridStats) {
	stats := &HybridStats{
		FusionMethod: "reciprocal_rank_fusion",
	}

	// Build chunk lookup for fast ID → chunk resolution
	chunkMap := make(map[string]*proto.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkMap[chunk.Id] = chunk
	}

	// === BM25 Retrieval ===
	candidateK := topK * 3 // Over-retrieve for better fusion
	if candidateK < 30 {
		candidateK = 30
	}

	// Ensure BM25 is indexed
	if !hs.bm25.indexed || len(allChunks) != len(hs.bm25.chunks) {
		hs.bm25.Index(allChunks)
	}
	bm25Results := hs.bm25.Search(query, candidateK)
	stats.BM25Candidates = len(bm25Results)

	// === Vector Retrieval ===
	// Auto-index from chunk embeddings if vector store is empty
	// (handles rehydrated sessions where chunks already have embeddings)
	if hs.embedder != nil && hs.vectorStore.Size() == 0 {
		hs.tryAutoIndex(allChunks)
	}

	var vectorResults []ScoredResult
	if hs.embedder != nil && hs.vectorStore.Size() > 0 {
		queryEmbedding, err := hs.embedder.EmbedText(query)
		if err == nil && len(queryEmbedding) > 0 {
			vectorResults = hs.vectorStore.Search(queryEmbedding, candidateK)
		}
	}
	stats.VectorCandidates = len(vectorResults)

	// === Reciprocal Rank Fusion ===
	// RRF score = Σ 1 / (k + rank_i) for each retrieval system
	rrfScores := make(map[string]*HybridResult)

	// Score BM25 results
	for rank, chunk := range bm25Results {
		id := chunk.Id
		if _, exists := rrfScores[id]; !exists {
			rrfScores[id] = &HybridResult{
				Chunk: chunk,
			}
		}
		rrfScores[id].Score += 1.0 / float64(hs.rrrK+rank+1)
		rrfScores[id].BM25Rank = rank + 1
		rrfScores[id].FromBM25 = true
	}

	// Score Vector results
	for rank, vr := range vectorResults {
		id := vr.ChunkID
		if _, exists := rrfScores[id]; !exists {
			chunk, ok := chunkMap[id]
			if !ok {
				continue
			}
			rrfScores[id] = &HybridResult{
				Chunk: chunk,
			}
		}
		rrfScores[id].Score += 1.0 / float64(hs.rrrK+rank+1)
		rrfScores[id].VectorRank = rank + 1
		rrfScores[id].FromVector = true
	}

	// Count overlaps
	for _, result := range rrfScores {
		if result.FromBM25 && result.FromVector {
			stats.OverlapCount++
		}
	}

	// Sort by RRF score
	ranked := make([]*HybridResult, 0, len(rrfScores))
	for _, result := range rrfScores {
		ranked = append(ranked, result)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	// Return top K
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}
	stats.FinalCount = len(ranked)

	return ranked, stats
}

// SearchBM25Only performs BM25-only retrieval (for comparison/fallback)
func (hs *HybridSearch) SearchBM25Only(allChunks []*proto.Chunk, query string, topK int) []*HybridResult {
	if !hs.bm25.indexed || len(allChunks) != len(hs.bm25.chunks) {
		hs.bm25.Index(allChunks)
	}
	bm25Results := hs.bm25.Search(query, topK)

	results := make([]*HybridResult, len(bm25Results))
	for i, chunk := range bm25Results {
		results[i] = &HybridResult{
			Chunk:    chunk,
			Score:    1.0 / float64(hs.rrrK+i+1),
			BM25Rank: i + 1,
			FromBM25: true,
		}
	}
	return results
}

// SearchVectorOnly performs vector-only retrieval (for comparison)
func (hs *HybridSearch) SearchVectorOnly(
	allChunks []*proto.Chunk,
	query string,
	topK int,
) []*HybridResult {
	chunkMap := make(map[string]*proto.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkMap[chunk.Id] = chunk
	}

	queryEmbedding, err := hs.embedder.EmbedText(query)
	if err != nil || len(queryEmbedding) == 0 {
		return nil
	}

	vectorResults := hs.vectorStore.Search(queryEmbedding, topK)
	results := make([]*HybridResult, 0, len(vectorResults))
	for i, vr := range vectorResults {
		chunk, ok := chunkMap[vr.ChunkID]
		if !ok {
			continue
		}
		results = append(results, &HybridResult{
			Chunk:      chunk,
			Score:      vr.Score,
			VectorRank: i + 1,
			FromVector: true,
		})
	}
	return results
}

// tryAutoIndex builds the vector store from chunk embeddings already stored on chunks.
// This handles cases where chunks were loaded from rehydrated sessions or multiple documents.
func (hs *HybridSearch) tryAutoIndex(chunks []*proto.Chunk) {
	var chunksWithEmb []*proto.Chunk
	var embeddings [][]float32

	for _, chunk := range chunks {
		if len(chunk.Embedding) > 0 {
			chunksWithEmb = append(chunksWithEmb, chunk)
			embeddings = append(embeddings, chunk.Embedding)
		}
	}

	if len(chunksWithEmb) > 0 {
		hs.vectorStore.IndexChunks(chunksWithEmb, embeddings)
	}
}
