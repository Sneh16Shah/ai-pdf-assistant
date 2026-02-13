package usecases

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
	"fmt"
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

	// Collect chunks from ALL documents in the session
	var allChunks []*proto.Chunk
	if len(session.Documents) > 0 {
		for _, doc := range session.Documents {
			allChunks = append(allChunks, doc.Chunks...)
		}
	} else if session.Document != nil {
		allChunks = session.Document.Chunks
	}

	// Find the most relevant chunks for context (topK=20 for good coverage)
	relevantChunks := uc.vectorSearch.FindRelevantChunks(allChunks, req.Message, 20)

	// Build context from relevant chunks instead of all chunks to stay within token limits
	context := uc.vectorSearch.BuildContext(relevantChunks)
	if context == "" {
		// Fallback: if no relevant chunks matched, use first ~15000 chars of document text
		var fullText string
		if len(session.Documents) > 0 {
			for _, doc := range session.Documents {
				fullText += "--- " + doc.Filename + " ---\n" + doc.Text + "\n\n"
			}
		} else if session.Document != nil {
			fullText = session.Document.Text
		}
		if len(fullText) > 15000 {
			fullText = fullText[:15000] + "\n... [truncated]"
		}
		context = "Document Context:\n\n" + fullText
	}

	// Use the same relevantChunks for citations

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

	// Get citations for the relevant chunks
	citations := uc.vectorSearch.GetCitations(relevantChunks)

	return &proto.ChatResponse{
		Status:         proto.Status_STATUS_SUCCESS,
		Response:       answer,
		SessionId:      req.SessionId,
		RelevantChunks: relevantChunkTexts,
		AnswerFound:    answerFound,
		Citations:      citations,
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
