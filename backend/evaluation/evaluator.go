package evaluation

import (
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
	"time"
)

// Evaluator runs the evaluation pipeline across multiple retrieval methods.
// Used in CI to gate deployments on retrieval quality thresholds.
type Evaluator struct {
	hybridSearch *services.HybridSearch
	bm25Search   *services.BM25Search
	thresholds   *EvalThresholds
}

// PipelineResult holds evaluation results for one pipeline
type PipelineResult struct {
	Pipeline     string            `json:"pipeline"`
	Metrics      *RetrievalMetrics `json:"metrics"`
	AvgLatencyMs int64             `json:"avg_latency_ms"`
	PassesGate   bool              `json:"passes_gate"`
	FailedChecks []string          `json:"failed_checks,omitempty"`
	QueryResults []QueryResult     `json:"query_results"`
}

// QueryResult holds per-query evaluation results
type QueryResult struct {
	QueryID   string            `json:"query_id"`
	Question  string            `json:"question"`
	Category  string            `json:"category"`
	Retrieved []string          `json:"retrieved_ids"`
	Metrics   *RetrievalMetrics `json:"metrics"`
	LatencyMs int64             `json:"latency_ms"`
}

// EvalReport is the complete evaluation report
type EvalReport struct {
	Timestamp    string                     `json:"timestamp"`
	DocumentName string                     `json:"document_name"`
	TotalQueries int                        `json:"total_queries"`
	Pipelines    map[string]*PipelineResult `json:"pipelines"`
	Winner       string                     `json:"winner"`
	Summary      string                     `json:"summary"`
}

// NewEvaluator creates a new evaluator
func NewEvaluator(
	hybridSearch *services.HybridSearch,
	bm25Search *services.BM25Search,
	thresholds *EvalThresholds,
) *Evaluator {
	if thresholds == nil {
		thresholds = DefaultThresholds()
	}
	return &Evaluator{
		hybridSearch: hybridSearch,
		bm25Search:   bm25Search,
		thresholds:   thresholds,
	}
}

// RunEvaluation executes the evaluation pipeline against golden QA pairs.
// Tests all retrieval methods and produces a gated report.
func (e *Evaluator) RunEvaluation(
	goldenSet *GoldenSet,
	allChunks []*proto.Chunk,
) *EvalReport {
	report := &EvalReport{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		DocumentName: goldenSet.DocumentName,
		TotalQueries: len(goldenSet.QAPairs),
		Pipelines:    make(map[string]*PipelineResult),
	}

	// Build page → chunk ID mapping for evaluation
	pageChunkMap := buildPageChunkMap(allChunks)

	// Evaluate each pipeline
	report.Pipelines["bm25_only"] = e.evaluateBM25Only(goldenSet, allChunks, pageChunkMap)
	report.Pipelines["hybrid_rrf"] = e.evaluateHybrid(goldenSet, allChunks, pageChunkMap)

	// Determine winner
	report.Winner = e.determineWinner(report.Pipelines)
	report.Summary = e.buildSummary(report)

	return report
}

func (e *Evaluator) evaluateBM25Only(
	goldenSet *GoldenSet,
	allChunks []*proto.Chunk,
	pageChunkMap map[int32][]string,
) *PipelineResult {
	result := &PipelineResult{
		Pipeline: "bm25_only",
	}

	var totalRecall5, totalRecall10, totalMRR, totalNDCG, totalPrec5 float64
	var totalLatency int64

	for _, qa := range goldenSet.QAPairs {
		start := time.Now()

		// Run BM25 retrieval
		bm25Results := e.hybridSearch.SearchBM25Only(allChunks, qa.Question, 10)
		latency := time.Since(start).Milliseconds()
		totalLatency += latency

		// Extract retrieved chunk IDs
		retrievedIDs := make([]string, len(bm25Results))
		for i, r := range bm25Results {
			retrievedIDs[i] = r.Chunk.Id
		}

		// Build relevant chunk IDs from expected pages
		relevantIDs := buildRelevantIDs(qa, pageChunkMap)
		relevanceScores := buildRelevanceScores(qa, pageChunkMap)

		// Compute metrics
		metrics := ComputeAll(retrievedIDs, relevantIDs, relevanceScores)

		totalRecall5 += metrics.RecallAt5
		totalRecall10 += metrics.RecallAt10
		totalMRR += metrics.MRR
		totalNDCG += metrics.NDCGAt10
		totalPrec5 += metrics.PrecisionAt5

		result.QueryResults = append(result.QueryResults, QueryResult{
			QueryID:   qa.ID,
			Question:  qa.Question,
			Category:  qa.Category,
			Retrieved: retrievedIDs,
			Metrics:   metrics,
			LatencyMs: latency,
		})
	}

	n := float64(len(goldenSet.QAPairs))
	if n == 0 {
		n = 1
	}

	result.Metrics = &RetrievalMetrics{
		RecallAt5:    totalRecall5 / n,
		RecallAt10:   totalRecall10 / n,
		MRR:          totalMRR / n,
		NDCGAt10:     totalNDCG / n,
		PrecisionAt5: totalPrec5 / n,
		F1At5:        F1Score(totalPrec5/n, totalRecall5/n),
	}
	result.AvgLatencyMs = totalLatency / int64(len(goldenSet.QAPairs))

	// Check CI gates
	result.PassesGate, result.FailedChecks = e.checkGates(result.Metrics, result.AvgLatencyMs)

	return result
}

func (e *Evaluator) evaluateHybrid(
	goldenSet *GoldenSet,
	allChunks []*proto.Chunk,
	pageChunkMap map[int32][]string,
) *PipelineResult {
	result := &PipelineResult{
		Pipeline: "hybrid_rrf",
	}

	var totalRecall5, totalRecall10, totalMRR, totalNDCG, totalPrec5 float64
	var totalLatency int64

	for _, qa := range goldenSet.QAPairs {
		start := time.Now()

		// Run hybrid retrieval
		hybridResults, _ := e.hybridSearch.Search(allChunks, qa.Question, 10)
		latency := time.Since(start).Milliseconds()
		totalLatency += latency

		retrievedIDs := make([]string, len(hybridResults))
		for i, r := range hybridResults {
			retrievedIDs[i] = r.Chunk.Id
		}

		relevantIDs := buildRelevantIDs(qa, pageChunkMap)
		relevanceScores := buildRelevanceScores(qa, pageChunkMap)

		metrics := ComputeAll(retrievedIDs, relevantIDs, relevanceScores)

		totalRecall5 += metrics.RecallAt5
		totalRecall10 += metrics.RecallAt10
		totalMRR += metrics.MRR
		totalNDCG += metrics.NDCGAt10
		totalPrec5 += metrics.PrecisionAt5

		result.QueryResults = append(result.QueryResults, QueryResult{
			QueryID:   qa.ID,
			Question:  qa.Question,
			Category:  qa.Category,
			Retrieved: retrievedIDs,
			Metrics:   metrics,
			LatencyMs: latency,
		})
	}

	n := float64(len(goldenSet.QAPairs))
	if n == 0 {
		n = 1
	}

	result.Metrics = &RetrievalMetrics{
		RecallAt5:    totalRecall5 / n,
		RecallAt10:   totalRecall10 / n,
		MRR:          totalMRR / n,
		NDCGAt10:     totalNDCG / n,
		PrecisionAt5: totalPrec5 / n,
		F1At5:        F1Score(totalPrec5/n, totalRecall5/n),
	}
	result.AvgLatencyMs = totalLatency / int64(len(goldenSet.QAPairs))

	result.PassesGate, result.FailedChecks = e.checkGates(result.Metrics, result.AvgLatencyMs)

	return result
}

func (e *Evaluator) checkGates(metrics *RetrievalMetrics, avgLatencyMs int64) (bool, []string) {
	var failures []string

	if metrics.RecallAt5 < e.thresholds.MinRecallAt5 {
		failures = append(failures, fmt.Sprintf(
			"Recall@5=%.3f < threshold %.3f", metrics.RecallAt5, e.thresholds.MinRecallAt5,
		))
	}
	if metrics.MRR < e.thresholds.MinMRR {
		failures = append(failures, fmt.Sprintf(
			"MRR=%.3f < threshold %.3f", metrics.MRR, e.thresholds.MinMRR,
		))
	}
	if metrics.NDCGAt10 < e.thresholds.MinNDCGAt10 {
		failures = append(failures, fmt.Sprintf(
			"NDCG@10=%.3f < threshold %.3f", metrics.NDCGAt10, e.thresholds.MinNDCGAt10,
		))
	}
	if avgLatencyMs > e.thresholds.MaxLatencyMs {
		failures = append(failures, fmt.Sprintf(
			"AvgLatency=%dms > threshold %dms", avgLatencyMs, e.thresholds.MaxLatencyMs,
		))
	}

	return len(failures) == 0, failures
}

func (e *Evaluator) determineWinner(pipelines map[string]*PipelineResult) string {
	bestPipeline := ""
	bestScore := -1.0

	for name, result := range pipelines {
		if result.Metrics == nil {
			continue
		}
		// Composite score: weighted average of key metrics
		score := result.Metrics.RecallAt10*0.3 +
			result.Metrics.MRR*0.3 +
			result.Metrics.NDCGAt10*0.3 +
			result.Metrics.PrecisionAt5*0.1

		if score > bestScore {
			bestScore = score
			bestPipeline = name
		}
	}

	return bestPipeline
}

func (e *Evaluator) buildSummary(report *EvalReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Evaluation of %d queries on '%s'\n", report.TotalQueries, report.DocumentName))
	sb.WriteString(fmt.Sprintf("Winner: %s\n\n", report.Winner))

	for name, result := range report.Pipelines {
		sb.WriteString(fmt.Sprintf("Pipeline: %s\n", name))
		sb.WriteString(fmt.Sprintf("  %s\n", result.Metrics.FormatMetrics()))
		sb.WriteString(fmt.Sprintf("  Avg Latency: %dms\n", result.AvgLatencyMs))
		sb.WriteString(fmt.Sprintf("  Passes CI Gate: %v\n", result.PassesGate))
		if len(result.FailedChecks) > 0 {
			sb.WriteString(fmt.Sprintf("  Failed: %s\n", strings.Join(result.FailedChecks, "; ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Helper functions

func buildPageChunkMap(chunks []*proto.Chunk) map[int32][]string {
	m := make(map[int32][]string)
	for _, chunk := range chunks {
		m[chunk.PageNumber] = append(m[chunk.PageNumber], chunk.Id)
	}
	return m
}

func buildRelevantIDs(qa GoldenQAPair, pageChunkMap map[int32][]string) []string {
	// If explicit chunk IDs are provided, use those
	if len(qa.RelevantChunkIDs) > 0 {
		return qa.RelevantChunkIDs
	}

	// Otherwise, derive from expected pages
	var ids []string
	for _, page := range qa.ExpectedPages {
		ids = append(ids, pageChunkMap[page]...)
	}
	return ids
}

func buildRelevanceScores(qa GoldenQAPair, pageChunkMap map[int32][]string) map[string]float64 {
	// If explicit scores are provided, use those
	if len(qa.RelevanceScores) > 0 {
		return qa.RelevanceScores
	}

	// Otherwise, assign score 1.0 to all chunks from expected pages
	scores := make(map[string]float64)
	for _, page := range qa.ExpectedPages {
		for _, id := range pageChunkMap[page] {
			scores[id] = 1.0
		}
	}
	return scores
}
