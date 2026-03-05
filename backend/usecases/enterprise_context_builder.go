package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
)

// EnterpriseContextBuilder implements the full enterprise RAG pipeline:
//
//  1. Hybrid retrieval (BM25 + Vector via RRF)
//  2. Cross-encoder reranking
//  3. Citation-enforced context assembly
//
// This is the industry-standard pattern used by Google Vertex AI Search,
// Azure AI Search, Cohere RAG, and most enterprise AI deployments.
type EnterpriseContextBuilder struct {
	hybridSearch    *services.HybridSearch
	reranker        *services.RerankerService
	citationService *services.CitationService
	preprocessor    *services.Preprocessor
}

// EnterpriseContextResult holds the built context plus metadata
type EnterpriseContextResult struct {
	Context        string
	CitationTags   []services.CitationTag
	HybridStats    *services.HybridStats
	TokenStats     *EnterpriseTokenStats
	RetrievedCount int
	RerankedCount  int
}

// EnterpriseTokenStats tracks detailed token usage
type EnterpriseTokenStats struct {
	RawTokens          int     `json:"raw_tokens"`
	AfterPreprocessing int     `json:"after_preprocessing"`
	RetrievedTokens    int     `json:"retrieved_tokens"`
	AfterReranking     int     `json:"after_reranking"`
	FinalTokens        int     `json:"final_tokens"`
	SavingsPercent     float64 `json:"savings_percent"`
	ChunksOriginal     int     `json:"chunks_original"`
	ChunksRetrieved    int     `json:"chunks_retrieved"`
	ChunksAfterRerank  int     `json:"chunks_after_rerank"`
}

// NewEnterpriseContextBuilder creates a new enterprise context builder
func NewEnterpriseContextBuilder(
	hybridSearch *services.HybridSearch,
	reranker *services.RerankerService,
	citationService *services.CitationService,
	preprocessor *services.Preprocessor,
) *EnterpriseContextBuilder {
	return &EnterpriseContextBuilder{
		hybridSearch:    hybridSearch,
		reranker:        reranker,
		citationService: citationService,
		preprocessor:    preprocessor,
	}
}

// BuildContext runs the full enterprise retrieval pipeline.
// Returns context with citation tags for grounded responses.
func (ecb *EnterpriseContextBuilder) BuildContext(
	allChunks []*proto.Chunk,
	fullText string,
	outline string,
	query string,
	pageNumber int32,
) *EnterpriseContextResult {
	stats := &EnterpriseTokenStats{
		RawTokens:      EstimateTokens(fullText),
		ChunksOriginal: len(allChunks),
	}

	result := &EnterpriseContextResult{
		TokenStats: stats,
	}

	// Small document shortcut — send everything but still with citations
	if len(fullText) < SmallDocThreshold {
		stats.FinalTokens = stats.RawTokens
		stats.SavingsPercent = 0
		stats.ChunksRetrieved = len(allChunks)
		stats.ChunksAfterRerank = len(allChunks)

		// Even for small docs, tag chunks for citations
		dummyResults := make([]*services.RerankResult, len(allChunks))
		for i, chunk := range allChunks {
			dummyResults[i] = &services.RerankResult{
				Chunk:          chunk,
				RelevanceScore: 1.0,
				OriginalRank:   i + 1,
				NewRank:        i + 1,
			}
		}
		tags := ecb.citationService.TagChunks(dummyResults)
		result.CitationTags = tags
		result.Context = ecb.citationService.BuildCitationPrompt(tags)
		result.RetrievedCount = len(allChunks)
		result.RerankedCount = len(allChunks)
		return result
	}

	// === STEP 1: Preprocess — strip boilerplate ===
	pageTexts := buildPageTextMap(allChunks)
	processedChunks, _ := ecb.preprocessor.ProcessChunks(allChunks, pageTexts)
	preprocessedText := chunksToText(processedChunks)
	stats.AfterPreprocessing = EstimateTokens(preprocessedText)

	// === STEP 2: Hybrid Retrieval (BM25 + Vector + RRF) ===
	retrievalTopK := 20 // Retrieve more for reranking
	hybridResults, hybridStats := ecb.hybridSearch.Search(processedChunks, query, retrievalTopK)
	result.HybridStats = hybridStats
	stats.ChunksRetrieved = len(hybridResults)

	// Count retrieved tokens
	for _, hr := range hybridResults {
		stats.RetrievedTokens += EstimateTokens(hr.Chunk.Text)
	}
	result.RetrievedCount = len(hybridResults)

	// === STEP 3: Cross-Encoder Reranking ===
	rerankedTopK := 10 // Keep top 10 after reranking
	reranked, err := ecb.reranker.Rerank(query, hybridResults, rerankedTopK)
	if err != nil {
		// Fallback: use hybrid results as-is
		reranked = make([]*services.RerankResult, 0, len(hybridResults))
		for i, hr := range hybridResults {
			if i >= rerankedTopK {
				break
			}
			reranked = append(reranked, &services.RerankResult{
				Chunk:          hr.Chunk,
				RelevanceScore: hr.Score,
				OriginalRank:   i + 1,
				NewRank:        i + 1,
			})
		}
	}
	stats.ChunksAfterRerank = len(reranked)
	result.RerankedCount = len(reranked)

	// === STEP 4: Build Citation-Enforced Context ===
	// Include current page chunks at the top if specified
	var contextBuilder strings.Builder
	maxChars := EstimateCharsFromTokens(MaxContextTokens)

	// Add outline (always)
	if outline != "" {
		contextBuilder.WriteString("=== DOCUMENT OUTLINE ===\n")
		contextBuilder.WriteString(outline)
		contextBuilder.WriteString("\n=== END OUTLINE ===\n\n")
	}

	// Include current page chunks that might not be in retrieval results
	addedIDs := make(map[string]bool)
	if pageNumber > 0 {
		contextBuilder.WriteString(fmt.Sprintf("=== CURRENT PAGE %d ===\n", pageNumber))
		for _, chunk := range processedChunks {
			if chunk.PageNumber == pageNumber {
				contextBuilder.WriteString(chunk.Text)
				contextBuilder.WriteString("\n\n")
				addedIDs[chunk.Id] = true
			}
		}
	}

	// Add citation-tagged reranked chunks
	tags := ecb.citationService.TagChunks(reranked)
	result.CitationTags = tags

	citationPrompt := ecb.citationService.BuildCitationPrompt(tags)
	contextBuilder.WriteString(citationPrompt)

	context := contextBuilder.String()

	// Enforce token budget
	if len(context) > maxChars {
		context = context[:maxChars]
	}

	result.Context = context
	stats.AfterReranking = 0
	for _, rr := range reranked {
		stats.AfterReranking += EstimateTokens(rr.Chunk.Text)
	}
	stats.FinalTokens = EstimateTokens(context)

	if stats.RawTokens > 0 {
		stats.SavingsPercent = float64(stats.RawTokens-stats.FinalTokens) / float64(stats.RawTokens) * 100
	}

	return result
}
