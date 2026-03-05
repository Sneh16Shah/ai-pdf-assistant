package evaluation

import (
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Metrics implements standard information retrieval evaluation metrics.
// These are the same metrics used in TREC, BEIR, and MTEB benchmarks.

// RecallAtK measures the fraction of relevant documents retrieved in top K.
// Recall@K = |relevant ∩ retrieved_top_k| / |relevant|
func RecallAtK(retrieved []string, relevant []string, k int) float64 {
	if len(relevant) == 0 {
		return 1.0 // No relevant docs means perfect recall by convention
	}

	relevantSet := toSet(relevant)

	limit := k
	if limit > len(retrieved) {
		limit = len(retrieved)
	}

	hits := 0
	for i := 0; i < limit; i++ {
		if relevantSet[retrieved[i]] {
			hits++
		}
	}

	return float64(hits) / float64(len(relevant))
}

// MRR computes Mean Reciprocal Rank — at what position is the first relevant result?
// MRR = 1/rank of first relevant result (0 if none found)
func MRR(retrieved []string, relevant []string) float64 {
	if len(relevant) == 0 {
		return 1.0
	}

	relevantSet := toSet(relevant)
	for i, id := range retrieved {
		if relevantSet[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// NDCG computes Normalized Discounted Cumulative Gain at K.
// Rewards relevant results that appear higher in the ranking.
// NDCG@K = DCG@K / IDCG@K
func NDCG(retrieved []string, relevanceScores map[string]float64, k int) float64 {
	limit := k
	if limit > len(retrieved) {
		limit = len(retrieved)
	}

	// DCG: sum of relevance / log2(rank+1)
	dcg := 0.0
	for i := 0; i < limit; i++ {
		rel, exists := relevanceScores[retrieved[i]]
		if !exists {
			continue
		}
		dcg += (math.Pow(2, rel) - 1) / math.Log2(float64(i+2))
	}

	// IDCG: best possible DCG (all relevant docs at top)
	scores := make([]float64, 0, len(relevanceScores))
	for _, s := range relevanceScores {
		scores = append(scores, s)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(scores)))

	idcg := 0.0
	idealLimit := limit
	if idealLimit > len(scores) {
		idealLimit = len(scores)
	}
	for i := 0; i < idealLimit; i++ {
		idcg += (math.Pow(2, scores[i]) - 1) / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0.0
	}
	return dcg / idcg
}

// Precision computes the fraction of retrieved documents that are relevant.
// Precision@K = |relevant ∩ retrieved_top_k| / K
func Precision(retrieved []string, relevant []string, k int) float64 {
	if k == 0 {
		return 0.0
	}

	relevantSet := toSet(relevant)
	limit := k
	if limit > len(retrieved) {
		limit = len(retrieved)
	}

	hits := 0
	for i := 0; i < limit; i++ {
		if relevantSet[retrieved[i]] {
			hits++
		}
	}

	return float64(hits) / float64(k)
}

// F1Score computes the harmonic mean of precision and recall
func F1Score(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0.0
	}
	return 2 * (precision * recall) / (precision + recall)
}

// ChunkOverlap measures how many chunks two retrieval methods share
func ChunkOverlap(retrievedA, retrievedB []string) float64 {
	if len(retrievedA) == 0 || len(retrievedB) == 0 {
		return 0.0
	}

	setA := toSet(retrievedA)
	overlap := 0
	for _, id := range retrievedB {
		if setA[id] {
			overlap++
		}
	}

	union := len(setA)
	for _, id := range retrievedB {
		if !setA[id] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}
	return float64(overlap) / float64(union)
}

// ContextRelevance estimates how relevant the retrieved context is to the query.
// Uses keyword overlap as a lightweight proxy (no API call needed).
func ContextRelevance(query string, chunks []*proto.Chunk) float64 {
	if len(chunks) == 0 {
		return 0.0
	}

	queryWords := strings.Fields(strings.ToLower(query))
	if len(queryWords) == 0 {
		return 0.0
	}

	totalScore := 0.0
	for _, chunk := range chunks {
		chunkLower := strings.ToLower(chunk.Text)
		hits := 0
		for _, word := range queryWords {
			if len(word) > 2 && strings.Contains(chunkLower, word) {
				hits++
			}
		}
		totalScore += float64(hits) / float64(len(queryWords))
	}

	return totalScore / float64(len(chunks))
}

// RetrievalMetrics holds all computed metrics for one retrieval run
type RetrievalMetrics struct {
	RecallAt5    float64 `json:"recall_at_5"`
	RecallAt10   float64 `json:"recall_at_10"`
	MRR          float64 `json:"mrr"`
	NDCGAt10     float64 `json:"ndcg_at_10"`
	PrecisionAt5 float64 `json:"precision_at_5"`
	F1At5        float64 `json:"f1_at_5"`
}

// ComputeAll computes all retrieval metrics for one query
func ComputeAll(retrieved []string, relevant []string, relevanceScores map[string]float64) *RetrievalMetrics {
	recallAt5 := RecallAtK(retrieved, relevant, 5)
	precAt5 := Precision(retrieved, relevant, 5)

	return &RetrievalMetrics{
		RecallAt5:    recallAt5,
		RecallAt10:   RecallAtK(retrieved, relevant, 10),
		MRR:          MRR(retrieved, relevant),
		NDCGAt10:     NDCG(retrieved, relevanceScores, 10),
		PrecisionAt5: precAt5,
		F1At5:        F1Score(precAt5, recallAt5),
	}
}

// FormatMetrics returns a human-readable summary of metrics
func (m *RetrievalMetrics) FormatMetrics() string {
	return fmt.Sprintf(
		"Recall@5=%.3f  Recall@10=%.3f  MRR=%.3f  NDCG@10=%.3f  P@5=%.3f  F1@5=%.3f",
		m.RecallAt5, m.RecallAt10, m.MRR, m.NDCGAt10, m.PrecisionAt5, m.F1At5,
	)
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
