package usecases

import (
	"fmt"
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
)

// ChatUseCase handles chat-related business logic
type ChatUseCase struct {
	sessionRepo  *repositories.SessionRepository
	aiService    services.AIService
	vectorSearch *services.VectorSearch
}

// NewChatUseCase creates a new chat use case
func NewChatUseCase(
	sessionRepo *repositories.SessionRepository,
	aiService services.AIService,
	vectorSearch *services.VectorSearch,
) *ChatUseCase {
	return &ChatUseCase{
		sessionRepo:  sessionRepo,
		aiService:    aiService,
		vectorSearch: vectorSearch,
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

	// Find relevant chunks using vector search
	relevantChunks := uc.vectorSearch.FindRelevantChunks(session.Document.Chunks, req.Message, 5)

	// Build context from relevant chunks
	context := uc.vectorSearch.BuildContext(relevantChunks)
	if context == "" {
		// Fallback to full document text if no chunks found
		context = "Document Context:\n\n" + session.Document.Text
	}

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

	return &proto.ChatResponse{
		Status:        proto.Status_STATUS_SUCCESS,
		Response:       answer,
		SessionId:      req.SessionId,
		RelevantChunks: relevantChunkTexts,
		AnswerFound:    answerFound,
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

