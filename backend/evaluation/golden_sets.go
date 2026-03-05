package evaluation

// GoldenQAPair represents a question-answer pair with expected retrieval targets.
// Used to evaluate retrieval quality across pipelines.
type GoldenQAPair struct {
	// ID is a unique identifier for this test case
	ID string `json:"id"`
	// Question is the user query to test
	Question string `json:"question"`
	// ExpectedPages lists pages that SHOULD be retrieved
	ExpectedPages []int32 `json:"expected_pages"`
	// RelevantChunkIDs lists chunk IDs that are relevant (if known)
	RelevantChunkIDs []string `json:"relevant_chunk_ids,omitempty"`
	// RelevanceScores maps chunk IDs to relevance scores (0-1) for NDCG
	RelevanceScores map[string]float64 `json:"relevance_scores,omitempty"`
	// ExpectedAnswer is a reference answer for answer quality evaluation
	ExpectedAnswer string `json:"expected_answer,omitempty"`
	// Category groups related test cases (e.g., "factual", "multi-hop", "negation")
	Category string `json:"category"`
	// Difficulty rates how hard this question is (1=easy, 5=hard)
	Difficulty int `json:"difficulty"`
}

// GoldenSet is a collection of golden QA pairs for a document
type GoldenSet struct {
	DocumentName string         `json:"document_name"`
	Description  string         `json:"description"`
	QAPairs      []GoldenQAPair `json:"qa_pairs"`
}

// EvalThresholds defines minimum acceptable metric values for CI gating
type EvalThresholds struct {
	MinRecallAt5 float64 `json:"min_recall_at_5"`
	MinMRR       float64 `json:"min_mrr"`
	MinNDCGAt10  float64 `json:"min_ndcg_at_10"`
	MinGrounding float64 `json:"min_grounding_score"`
	MaxLatencyMs int64   `json:"max_latency_ms"`
}

// DefaultThresholds returns sensible defaults for CI gating
func DefaultThresholds() *EvalThresholds {
	return &EvalThresholds{
		MinRecallAt5: 0.60,  // At least 60% of relevant chunks in top 5
		MinMRR:       0.50,  // First relevant result in top 2 on average
		MinNDCGAt10:  0.50,  // Reasonable ranking quality
		MinGrounding: 0.80,  // 80%+ of citations must be valid
		MaxLatencyMs: 10000, // 10 seconds max per query
	}
}

// NewSampleGoldenSet creates a sample golden set for testing.
// In production, these would be human-curated and stored as JSON files.
func NewSampleGoldenSet() *GoldenSet {
	return &GoldenSet{
		DocumentName: "sample_document",
		Description:  "Sample golden QA pairs for demonstrating the evaluation pipeline",
		QAPairs: []GoldenQAPair{
			{
				ID:            "factual_001",
				Question:      "What is the main topic of this document?",
				ExpectedPages: []int32{1, 2},
				Category:      "factual",
				Difficulty:    1,
			},
			{
				ID:            "specific_001",
				Question:      "What specific methodology is described?",
				ExpectedPages: []int32{3, 4, 5},
				Category:      "specific",
				Difficulty:    2,
			},
			{
				ID:            "multi_hop_001",
				Question:      "How do the conclusions relate to the methodology?",
				ExpectedPages: []int32{3, 4, 5, 8, 9},
				Category:      "multi_hop",
				Difficulty:    4,
			},
			{
				ID:            "negation_001",
				Question:      "What limitations or exclusions are mentioned?",
				ExpectedPages: []int32{6, 7},
				Category:      "negation",
				Difficulty:    3,
			},
			{
				ID:            "numerical_001",
				Question:      "What are the key statistics or numbers mentioned?",
				ExpectedPages: []int32{4, 5, 8},
				Category:      "numerical",
				Difficulty:    2,
			},
		},
	}
}
