package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
	"strings"
)

// ChatUseCase handles chat-related business logic
type ChatUseCase struct {
	sessionRepo     *repositories.SessionRepository
	aiService       services.AIService
	vectorSearch    *services.VectorSearch
	contextBuilder  *ContextBuilder
	visionService   *services.VisionService
	imageGenService *services.ImageGenService
}

// NewChatUseCase creates a new chat use case
func NewChatUseCase(
	sessionRepo *repositories.SessionRepository,
	aiService services.AIService,
	vectorSearch *services.VectorSearch,
	visionService *services.VisionService,
	imageGenService *services.ImageGenService,
) *ChatUseCase {
	return &ChatUseCase{
		sessionRepo:     sessionRepo,
		aiService:       aiService,
		vectorSearch:    vectorSearch,
		contextBuilder:  NewContextBuilder(vectorSearch),
		visionService:   visionService,
		imageGenService: imageGenService,
	}
}

// AskQuestion processes a chat question and returns an answer
func (uc *ChatUseCase) AskQuestion(req *proto.ChatRequest) (*proto.ChatResponse, error) {
	// Get session
	session, err := uc.sessionRepo.Get(req.SessionId)
	if err != nil {
		return &proto.ChatResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: fmt.Sprintf("Session not found: %v", err),
			},
		}, nil
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
		}, nil
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

	// Build context using tiered strategy (page-aware, with budget)
	context := uc.contextBuilder.BuildContext(allChunks, fullText, outline, req.Message, req.PageNumber)

	// Get relevant chunks for citations
	relevantChunks := uc.vectorSearch.FindRelevantChunks(allChunks, req.Message, 10)

	// Build conversation history (exclude the current user message we just added)
	history := make([]string, 0)
	for i, msg := range session.Messages {
		// Skip the last message (the one we just added)
		if i == len(session.Messages)-1 {
			continue
		}
		if msg.Role == "user" {
			history = append(history, "User: "+msg.Content)
		} else if msg.Role == "assistant" {
			history = append(history, "Assistant: "+msg.Content)
		}
	}

	// Check if this is an image generation request
	var imageBase64, imageMimeType string
	if services.IsImageGenRequest(req.Message) && uc.imageGenService != nil && uc.imageGenService.IsAvailable() {
		fmt.Printf("INFO: Detected image generation request, using Gemini\n")
		result, err := uc.imageGenService.GenerateImageFromContext(req.Message, context)
		if err != nil {
			fmt.Printf("WARNING: Image generation failed: %v, falling back to text\n", err)
		} else if result != nil {
			imageBase64 = result.ImageBase64
			imageMimeType = result.MimeType
			if result.TextReply != "" {
				// If Gemini returned text alongside the image, use it
				context = context + "\n\nDiagram Generation Note: " + result.TextReply
			}
		}
	}

	// Check if this is a diagram question (and we have vision capabilities)
	if services.IsDiagramQuestion(req.Message) && uc.visionService != nil && imageBase64 == "" {
		fmt.Printf("INFO: Detected diagram question, vision model available for page %d\n", req.PageNumber)
		// Note: Vision requires a rendered page image. For now, we enhance the text prompt
		// to indicate the AI should describe any diagrams. Full image rendering will be
		// added when we integrate a PDF-to-image renderer.
		context = context + "\n\nNote: The user is asking about a visual element (diagram/chart/table). " +
			"If the text context describes a diagram or visual structure, explain it in detail. " +
			"Describe the components, relationships, and flow shown in the visual element."
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
		}, nil
	}

	// Add AI response to session
	aiMessage := &proto.ChatMessage{
		Role:    "assistant",
		Content: answer,
	}
	if err := uc.sessionRepo.AddMessage(req.SessionId, aiMessage); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: Failed to store AI message: %v\n", err)
	}

	// Extract relevant chunk texts for transparency
	relevantChunkTexts := make([]string, len(relevantChunks))
	for i, chunk := range relevantChunks {
		// Limit chunk text length for response
		chunkText := chunk.Text
		if len(chunkText) > 200 {
			chunkText = chunkText[:200] + "..."
		}
		relevantChunkTexts[i] = chunkText
	}

	// Get citations for the relevant chunks (skip for general/overview questions)
	var citations []services.Citation
	if isGeneralQuestion(req.Message) {
		citations = []services.Citation{}
	} else {
		citations = uc.vectorSearch.GetCitations(relevantChunks)
	}

	return &proto.ChatResponse{
		Status:         proto.Status_STATUS_SUCCESS,
		Response:       answer,
		SessionId:      req.SessionId,
		RelevantChunks: relevantChunkTexts,
		AnswerFound:    answerFound,
		Citations:      citations,
		ImageBase64:    imageBase64,
		ImageMimeType:  imageMimeType,
	}, nil
}

// GetHistory retrieves chat history for a session
func (uc *ChatUseCase) GetHistory(sessionID string) (*proto.HistoryResponse, error) {
	session, err := uc.sessionRepo.Get(sessionID)
	if err != nil {
		return &proto.HistoryResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: fmt.Sprintf("Session not found: %v", err),
			},
		}, nil
	}

	return &proto.HistoryResponse{
		Status:  proto.Status_STATUS_SUCCESS,
		Session: session,
	}, nil
}

// ClearSession clears all messages in a session
func (uc *ChatUseCase) ClearSession(sessionID string) (*proto.ClearSessionResponse, error) {
	err := uc.sessionRepo.ClearMessages(sessionID)
	if err != nil {
		return &proto.ClearSessionResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: fmt.Sprintf("Session not found: %v", err),
			},
		}, nil
	}

	return &proto.ClearSessionResponse{
		Status:  proto.Status_STATUS_SUCCESS,
		Message: "Session cleared successfully",
	}, nil
}

// isGeneralQuestion checks if a question is a broad overview/summary question
// that shouldn't have specific page citations.
func isGeneralQuestion(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))

	generalPatterns := []string{
		"what is this pdf about",
		"what is this document about",
		"what is this book about",
		"what is this paper about",
		"what is this file about",
		"what is this about",
		"summarize",
		"summary",
		"overview",
		"what does this cover",
		"what does this document cover",
		"what does this pdf cover",
		"what topics",
		"table of contents",
		"main topics",
		"key topics",
		"give me an overview",
		"tell me about this",
		"describe this document",
		"describe this pdf",
		"what can you tell me about this",
	}

	for _, pattern := range generalPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
