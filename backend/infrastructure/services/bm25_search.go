package services

import (
	"ai-pdf-assistant-backend/proto"
	"math"
	"sort"
	"strings"
)

// BM25 parameters (standard Okapi BM25 defaults)
const (
	bm25K1 = 1.5  // Term frequency saturation parameter
	bm25B  = 0.75 // Length normalization parameter
)

// BM25Search provides BM25-based document chunk retrieval.
// This is the same ranking algorithm used by Elasticsearch/Lucene.
// Pure math, zero external dependencies or API calls.
type BM25Search struct {
	// Pre-computed index data
	chunks     []*proto.Chunk
	chunkTerms []map[string]int // term frequency per chunk
	idf        map[string]float64
	avgDocLen  float64
	docLens    []int
	indexed    bool
}

// NewBM25Search creates a new BM25 search instance
func NewBM25Search() *BM25Search {
	return &BM25Search{}
}

// Index pre-computes IDF and term frequencies for all chunks.
// Call this once after loading/uploading a document.
func (b *BM25Search) Index(chunks []*proto.Chunk) {
	if len(chunks) == 0 {
		return
	}

	b.chunks = chunks
	b.chunkTerms = make([]map[string]int, len(chunks))
	b.docLens = make([]int, len(chunks))

	// Document frequency: how many chunks contain each term
	df := make(map[string]int)
	totalLen := 0

	for i, chunk := range chunks {
		terms := tokenize(chunk.Text)
		b.docLens[i] = len(terms)
		totalLen += len(terms)

		// Count term frequencies in this chunk
		tf := make(map[string]int)
		seen := make(map[string]bool)
		for _, term := range terms {
			tf[term]++
			if !seen[term] {
				df[term]++
				seen[term] = true
			}
		}
		b.chunkTerms[i] = tf
	}

	b.avgDocLen = float64(totalLen) / float64(len(chunks))

	// Compute IDF for each term: log((N - df + 0.5) / (df + 0.5) + 1)
	n := float64(len(chunks))
	b.idf = make(map[string]float64, len(df))
	for term, docFreq := range df {
		b.idf[term] = math.Log((n-float64(docFreq)+0.5)/(float64(docFreq)+0.5) + 1.0)
	}

	b.indexed = true
}

// Search returns the top-K chunks ranked by BM25 relevance to the query.
func (b *BM25Search) Search(query string, topK int) []*proto.Chunk {
	if !b.indexed || len(b.chunks) == 0 {
		return []*proto.Chunk{}
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return []*proto.Chunk{}
	}

	type scored struct {
		chunk *proto.Chunk
		score float64
	}

	results := make([]scored, 0, len(b.chunks))

	for i, chunk := range b.chunks {
		score := 0.0
		dl := float64(b.docLens[i])

		for _, qt := range queryTerms {
			idf, exists := b.idf[qt]
			if !exists {
				continue
			}

			tf := float64(b.chunkTerms[i][qt])
			if tf == 0 {
				continue
			}

			// BM25 scoring formula
			numerator := tf * (bm25K1 + 1)
			denominator := tf + bm25K1*(1-bm25B+bm25B*dl/b.avgDocLen)
			score += idf * (numerator / denominator)
		}

		if score > 0 {
			results = append(results, scored{chunk: chunk, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Return top K
	out := make([]*proto.Chunk, 0, topK)
	for i, s := range results {
		if i >= topK {
			break
		}
		out = append(out, s.chunk)
	}

	// If no matches, fallback to first few chunks
	if len(out) == 0 && len(b.chunks) > 0 {
		maxReturn := topK
		if len(b.chunks) < maxReturn {
			maxReturn = len(b.chunks)
		}
		return b.chunks[:maxReturn]
	}

	return out
}

// FindRelevantChunks is a compatibility wrapper matching the VectorSearch interface
func (b *BM25Search) FindRelevantChunks(chunks []*proto.Chunk, query string, topK int) []*proto.Chunk {
	// Re-index if chunks changed
	if !b.indexed || len(chunks) != len(b.chunks) {
		b.Index(chunks)
	}
	return b.Search(query, topK)
}

// tokenize splits text into lowercase terms, filtering out very short words
func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	result := make([]string, 0, len(words))
	for _, w := range words {
		// Strip common punctuation from edges
		w = strings.Trim(w, ".,;:!?\"'()[]{}—–-/\\")
		if len(w) > 1 { // Skip single chars
			result = append(result, w)
		}
	}
	return result
}
