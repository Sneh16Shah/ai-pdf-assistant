package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
)

// TokenStats tracks token usage for comparison with the standard pipeline
type TokenStats struct {
	RawTokens          int     `json:"raw_tokens"`           // Tokens if full text were sent
	AfterPreprocessing int     `json:"after_preprocessing"`  // Tokens after boilerplate/dedup
	AfterRetrieval     int     `json:"after_retrieval"`      // Tokens after BM25 selection
	FinalTokens        int     `json:"final_tokens"`         // Tokens actually sent to LLM
	SavingsPercent     float64 `json:"savings_percent"`      // % saved vs raw
	BoilerplateRemoved int     `json:"boilerplate_removed"`  // Chars of boilerplate stripped
	ChunksOriginal     int     `json:"chunks_original"`      // Original chunk count
	ChunksAfterDedup   int     `json:"chunks_after_dedup"`   // Chunks after deduplication
	ChunksUsed         int     `json:"chunks_used"`          // Chunks actually sent
	TextRankSummaryLen int     `json:"textrank_summary_len"` // Length of TextRank summaries used
}

// SmartChatUseCase handles chat with optimized context building.
// Mirrors ChatUseCase but uses BM25 + TextRank + preprocessing
// and tracks token savings for comparison.
type SmartChatUseCase struct {
	sessionRepo         *repositories.SessionRepository
	aiService           services.AIService
	smartContextBuilder *SmartContextBuilder
	bm25                *services.BM25Search
	vectorSearch        *services.VectorSearch // for GetCitations compatibility
	visionService       *services.VisionService
	imageGenService     *services.ImageGenService
	// Cumulative session stats
	sessionStats map[string]*CumulativeStats
}

// CumulativeStats tracks total token savings across a session
type CumulativeStats struct {
	TotalQueries      int     `json:"total_queries"`
	TotalRawTokens    int     `json:"total_raw_tokens"`
	TotalFinalTokens  int     `json:"total_final_tokens"`
	TotalSavedTokens  int     `json:"total_saved_tokens"`
	AvgSavingsPercent float64 `json:"avg_savings_percent"`
}

// NewSmartChatUseCase creates a new smart chat use case
func NewSmartChatUseCase(
	sessionRepo *repositories.SessionRepository,
	aiService services.AIService,
	smartContextBuilder *SmartContextBuilder,
	bm25 *services.BM25Search,
	vectorSearch *services.VectorSearch,
	visionService *services.VisionService,
	imageGenService *services.ImageGenService,
) *SmartChatUseCase {
	return &SmartChatUseCase{
		sessionRepo:         sessionRepo,
		aiService:           aiService,
		smartContextBuilder: smartContextBuilder,
		bm25:                bm25,
		vectorSearch:        vectorSearch,
		visionService:       visionService,
		imageGenService:     imageGenService,
		sessionStats:        make(map[string]*CumulativeStats),
	}
}

// AskQuestion processes a chat question using the optimized pipeline.
// Returns the AI response plus detailed token statistics.
func (uc *SmartChatUseCase) AskQuestion(req *proto.ChatRequest) (*proto.ChatResponse, *TokenStats, error) {
	// Get session
	session, err := uc.sessionRepo.Get(req.SessionId)
	if err != nil {
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: fmt.Sprintf("Session not found: %v", err),
			},
		}, nil, nil
	}

	// Add user message to session
	userMessage := &proto.ChatMessage{
		Role:    "user",
		Content: req.Message,
	}
	if err := uc.sessionRepo.AddMessage(req.SessionId, userMessage); err != nil {
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "MESSAGE_STORAGE_ERROR",
				Message: fmt.Sprintf("Failed to store message: %v", err),
			},
		}, nil, nil
	}

	// Collect chunks and full text from ALL documents in the session
	var allChunks []*proto.Chunk
	var fullText string
	var outline string
	if len(session.Documents) > 0 {
		for _, doc := range session.Documents {
			allChunks = append(allChunks, doc.Chunks...)
			fullText += "--- " + doc.Filename + " ---\n" + doc.Text + "\n\n"
			if doc.Outline != "" {
				outline += "=== " + doc.Filename + " ===\n" + doc.Outline + "\n"
			}
		}
	} else if session.Document != nil {
		allChunks = session.Document.Chunks
		fullText = session.Document.Text
		outline = session.Document.Outline
	}

	// === SMART PIPELINE: Build context with BM25 + TextRank + preprocessing ===
	context, stats := uc.smartContextBuilder.BuildContext(allChunks, fullText, outline, req.Message, req.PageNumber)

	fmt.Printf("SMART MODE: Raw=%d tokens, Final=%d tokens, Saved=%.1f%%\n",
		stats.RawTokens, stats.FinalTokens, stats.SavingsPercent)

	// Get relevant chunks for citations (using BM25 instead of keyword)
	relevantChunks := uc.bm25.FindRelevantChunks(allChunks, req.Message, 10)

	// Build conversation history (limited to last 5 turns for token efficiency)
	history := make([]string, 0)
	maxHistory := 10 // 5 turns = 10 messages (user + assistant)
	startIdx := 0
	if len(session.Messages)-1 > maxHistory {
		startIdx = len(session.Messages) - 1 - maxHistory
	}
	for i := startIdx; i < len(session.Messages)-1; i++ {
		msg := session.Messages[i]
		if msg.Role == "user" {
			history = append(history, "User: "+msg.Content)
		} else if msg.Role == "assistant" {
			// Truncate long assistant messages in history to save tokens
			content := msg.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			history = append(history, "Assistant: "+content)
		}
	}

	// Check if this is an image generation request
	var imageBase64, imageMimeType string
	if services.IsImageGenRequest(req.Message) && uc.imageGenService != nil && uc.imageGenService.IsAvailable() {
		result, err := uc.imageGenService.GenerateImageFromContext(req.Message, context)
		if err != nil {
			fmt.Printf("WARNING: Image generation failed: %v, falling back to text\n", err)
		} else if result != nil {
			imageBase64 = result.ImageBase64
			imageMimeType = result.MimeType
		}
	}

	// Check if this is a diagram question
	if services.IsDiagramQuestion(req.Message) && uc.visionService != nil && imageBase64 == "" {
		context = context + "\n\nNote: The user is asking about a visual element (diagram/chart/table). " +
			"If the text context describes a diagram or visual structure, explain it in detail."
	}

	// Get AI response
	answer, answerFound, err := uc.aiService.AnswerQuestion(context, req.Message, history)
	if err != nil {
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "AI_SERVICE_ERROR",
				Message: fmt.Sprintf("Failed to get AI response: %v", err),
			},
		}, stats, nil
	}

	// Add AI response to session
	aiMessage := &proto.ChatMessage{
		Role:    "assistant",
		Content: answer,
	}
	if err := uc.sessionRepo.AddMessage(req.SessionId, aiMessage); err != nil {
		fmt.Printf("Warning: Failed to store AI message: %v\n", err)
	}

	// Extract relevant chunk texts for transparency
	relevantChunkTexts := make([]string, len(relevantChunks))
	for i, chunk := range relevantChunks {
		chunkText := chunk.Text
		if len(chunkText) > 200 {
			chunkText = chunkText[:200] + "..."
		}
		relevantChunkTexts[i] = chunkText
	}

	// Get citations — reuse VectorSearch.GetCitations for compatibility
	var citations []services.Citation
	if isGeneralQuestion(req.Message) {
		citations = []services.Citation{}
	} else {
		citations = uc.vectorSearch.GetCitations(relevantChunks)
	}

	// Update cumulative session stats
	uc.updateSessionStats(req.SessionId, stats)

	return &proto.ChatResponse{
		Status:         proto.Status_STATUS_SUCCESS,
		Response:       answer,
		SessionId:      req.SessionId,
		AnswerFound:    answerFound,
		RelevantChunks: relevantChunkTexts,
		Citations:      citations,
		ImageBase64:    imageBase64,
		ImageMimeType:  imageMimeType,
	}, stats, nil
}

// GetSessionStats returns cumulative token savings for a session
func (uc *SmartChatUseCase) GetSessionStats(sessionID string) *CumulativeStats {
	if stats, ok := uc.sessionStats[sessionID]; ok {
		return stats
	}
	return &CumulativeStats{}
}

// updateSessionStats updates cumulative statistics for a session
func (uc *SmartChatUseCase) updateSessionStats(sessionID string, queryStats *TokenStats) {
	if _, ok := uc.sessionStats[sessionID]; !ok {
		uc.sessionStats[sessionID] = &CumulativeStats{}
	}

	s := uc.sessionStats[sessionID]
	s.TotalQueries++
	s.TotalRawTokens += queryStats.RawTokens
	s.TotalFinalTokens += queryStats.FinalTokens
	s.TotalSavedTokens += queryStats.RawTokens - queryStats.FinalTokens

	if s.TotalRawTokens > 0 {
		s.AvgSavingsPercent = float64(s.TotalSavedTokens) / float64(s.TotalRawTokens) * 100
	}
}

// isSmartGeneralQuestion is a simplified check (avoids circular dep with chat_usecase)
func isSmartGeneralQuestion(msg string) bool {
	msg = strings.ToLower(msg)
	generalPhrases := []string{
		"what is this about", "summarize", "summary",
		"overview", "main topics", "what does this cover",
		"tell me about this", "what's in this",
	}
	for _, phrase := range generalPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
