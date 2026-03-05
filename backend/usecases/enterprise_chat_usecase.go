package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"log"
	"strings"
	"sync"
)

// EnterpriseChatUseCase handles chat with the full enterprise RAG pipeline.
// Provides grounded answers with citations and comprehensive statistics
// for comparison against the standard and smart pipelines.
type EnterpriseChatUseCase struct {
	sessionRepo     *repositories.SessionRepository
	aiService       services.AIService
	contextBuilder  *EnterpriseContextBuilder
	citationService *services.CitationService
	visionService   *services.VisionService
	imageGenService *services.ImageGenService

	// Session-level stats
	sessionStats map[string]*EnterpriseCumulativeStats
	statsMu      sync.RWMutex
}

// EnterpriseCumulativeStats tracks cumulative statistics across a session
type EnterpriseCumulativeStats struct {
	TotalQueries        int     `json:"total_queries"`
	TotalRawTokens      int     `json:"total_raw_tokens"`
	TotalFinalTokens    int     `json:"total_final_tokens"`
	TotalSavedTokens    int     `json:"total_saved_tokens"`
	AvgSavingsPercent   float64 `json:"avg_savings_percent"`
	AvgGroundingScore   float64 `json:"avg_grounding_score"`
	TotalCitations      int     `json:"total_citations"`
	TotalValidCitations int     `json:"total_valid_citations"`
	AvgRetrievedChunks  float64 `json:"avg_retrieved_chunks"`
	AvgRerankedChunks   float64 `json:"avg_reranked_chunks"`
}

// EnterpriseQueryStats holds per-query statistics for the comparison dashboard
type EnterpriseQueryStats struct {
	TokenStats    *EnterpriseTokenStats   `json:"token_stats"`
	HybridStats   *services.HybridStats   `json:"hybrid_stats"`
	CitationStats *services.CitationStats `json:"citation_stats"`
	Pipeline      string                  `json:"pipeline"`
}

// NewEnterpriseChatUseCase creates a new enterprise chat use case
func NewEnterpriseChatUseCase(
	sessionRepo *repositories.SessionRepository,
	aiService services.AIService,
	contextBuilder *EnterpriseContextBuilder,
	citationService *services.CitationService,
	visionService *services.VisionService,
	imageGenService *services.ImageGenService,
) *EnterpriseChatUseCase {
	return &EnterpriseChatUseCase{
		sessionRepo:     sessionRepo,
		aiService:       aiService,
		contextBuilder:  contextBuilder,
		citationService: citationService,
		visionService:   visionService,
		imageGenService: imageGenService,
		sessionStats:    make(map[string]*EnterpriseCumulativeStats),
	}
}

// AskQuestion processes a question through the enterprise RAG pipeline.
// Returns AI response with citations and detailed query statistics.
func (uc *EnterpriseChatUseCase) AskQuestion(req *proto.ChatRequest) (*proto.ChatResponse, *EnterpriseQueryStats, error) {
	// Validate session
	session, err := uc.sessionRepo.Get(req.SessionId)
	if err != nil {
		return nil, nil, fmt.Errorf("session not found: %w", err)
	}

	docs := session.Documents
	if len(docs) == 0 {
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "NO_DOCUMENTS",
				Message: "No documents in session",
			},
		}, nil, nil
	}

	// Store user message
	uc.sessionRepo.AddMessage(req.SessionId, &proto.ChatMessage{
		Role:    "user",
		Content: req.Message,
	})

	// Combine all docs
	var allChunks []*proto.Chunk
	var fullTextBuilder strings.Builder
	var outlineBuilder strings.Builder

	for _, doc := range docs {
		allChunks = append(allChunks, doc.Chunks...)
		fullTextBuilder.WriteString(doc.Text)
		fullTextBuilder.WriteString("\n\n")
		if doc.Outline != "" {
			outlineBuilder.WriteString(fmt.Sprintf("=== %s ===\n", doc.Filename))
			outlineBuilder.WriteString(doc.Outline)
			outlineBuilder.WriteString("\n")
		}
	}

	fullText := fullTextBuilder.String()
	outline := outlineBuilder.String()

	// === Run Enterprise Pipeline ===
	contextResult := uc.contextBuilder.BuildContext(
		allChunks, fullText, outline, req.Message, req.PageNumber,
	)

	// Build history strings for AI service
	var historyStrings []string
	if len(session.Messages) > 0 {
		startIdx := 0
		if len(session.Messages) > 6 {
			startIdx = len(session.Messages) - 6
		}
		for _, msg := range session.Messages[startIdx:] {
			content := msg.Content
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			historyStrings = append(historyStrings, fmt.Sprintf("%s: %s", msg.Role, content))
		}
	}

	// Call AI service using the correct interface
	aiResponse, _, err := uc.aiService.AnswerQuestion(contextResult.Context, req.Message, historyStrings)
	if err != nil {
		log.Printf("Enterprise AI error: %v", err)
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "AI_ERROR",
				Message: fmt.Sprintf("AI service error: %v", err),
			},
		}, nil, nil
	}

	// === Post-process: Extract and validate citations ===
	citations, citationStats := uc.citationService.ExtractCitations(aiResponse, contextResult.CitationTags)

	// Format response with readable page references
	formattedResponse := uc.citationService.FormatResponseWithCitations(aiResponse, contextResult.CitationTags)

	// Add citation footnotes
	footnotes := uc.citationService.FormatCitationFootnotes(citations)
	if footnotes != "" {
		formattedResponse += footnotes
	}

	// Build page citations for the UI
	pageCitations := uc.citationService.BuildPageCitations(citations)

	// Store assistant response
	uc.sessionRepo.AddMessage(req.SessionId, &proto.ChatMessage{
		Role:    "assistant",
		Content: formattedResponse,
	})

	// Build query stats
	queryStats := &EnterpriseQueryStats{
		TokenStats:    contextResult.TokenStats,
		HybridStats:   contextResult.HybridStats,
		CitationStats: citationStats,
		Pipeline:      "enterprise",
	}

	// Update session stats
	uc.updateSessionStats(req.SessionId, contextResult.TokenStats, citationStats)

	// Build response with citations
	response := &proto.ChatResponse{
		Status:   proto.Status_STATUS_SUCCESS,
		Response: formattedResponse,
	}

	// Add page citations
	if len(pageCitations) > 0 && !isEnterpriseGeneralQuestion(req.Message) {
		citationList := make([]services.Citation, len(pageCitations))
		for i, page := range pageCitations {
			citationList[i] = services.Citation{
				Page: page,
				Text: fmt.Sprintf("Referenced from page %d", page),
			}
		}
		response.Citations = citationList
	}

	return response, queryStats, nil
}

// GetSessionStats returns cumulative statistics for a session
func (uc *EnterpriseChatUseCase) GetSessionStats(sessionID string) *EnterpriseCumulativeStats {
	uc.statsMu.RLock()
	defer uc.statsMu.RUnlock()

	stats, exists := uc.sessionStats[sessionID]
	if !exists {
		return &EnterpriseCumulativeStats{}
	}
	return stats
}

func (uc *EnterpriseChatUseCase) updateSessionStats(sessionID string, tokenStats *EnterpriseTokenStats, citationStats *services.CitationStats) {
	uc.statsMu.Lock()
	defer uc.statsMu.Unlock()

	stats, exists := uc.sessionStats[sessionID]
	if !exists {
		stats = &EnterpriseCumulativeStats{}
		uc.sessionStats[sessionID] = stats
	}

	stats.TotalQueries++
	stats.TotalRawTokens += tokenStats.RawTokens
	stats.TotalFinalTokens += tokenStats.FinalTokens
	stats.TotalSavedTokens += tokenStats.RawTokens - tokenStats.FinalTokens

	if stats.TotalRawTokens > 0 {
		stats.AvgSavingsPercent = float64(stats.TotalSavedTokens) / float64(stats.TotalRawTokens) * 100
	}

	if citationStats != nil {
		stats.TotalCitations += citationStats.TotalCitations
		stats.TotalValidCitations += citationStats.ValidCitations
		if stats.TotalCitations > 0 {
			stats.AvgGroundingScore = float64(stats.TotalValidCitations) / float64(stats.TotalCitations)
		}
	}

	stats.AvgRetrievedChunks = ((stats.AvgRetrievedChunks * float64(stats.TotalQueries-1)) + float64(tokenStats.ChunksRetrieved)) / float64(stats.TotalQueries)
	stats.AvgRerankedChunks = ((stats.AvgRerankedChunks * float64(stats.TotalQueries-1)) + float64(tokenStats.ChunksAfterRerank)) / float64(stats.TotalQueries)
}

func isEnterpriseGeneralQuestion(msg string) bool {
	msg = strings.ToLower(msg)
	generalPhrases := []string{
		"summarize", "summary", "what is this about",
		"overview", "main topics", "key points",
		"what does this document", "tell me about",
	}
	for _, phrase := range generalPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
