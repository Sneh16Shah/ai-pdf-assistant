package usecases

import (
	"fmt"
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
)

// SummaryUseCase handles summary generation business logic
type SummaryUseCase struct {
	sessionRepo *repositories.SessionRepository
	aiService   services.AIService
}

// NewSummaryUseCase creates a new summary use case
func NewSummaryUseCase(
	sessionRepo *repositories.SessionRepository,
	aiService services.AIService,
) *SummaryUseCase {
	return &SummaryUseCase{
		sessionRepo: sessionRepo,
		aiService:   aiService,
	}
}

// GenerateSummary generates a summary for a document
func (uc *SummaryUseCase) GenerateSummary(req *proto.SummaryRequest) (*proto.SummaryResponse, error) {
	// Get session to access document
	session, err := uc.sessionRepo.Get(req.SessionId)
	if err != nil {
		return &proto.SummaryResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: fmt.Sprintf("Session not found: %v", err),
			},
		}, nil
	}

	// Get document text
	doc := session.Document
	if doc == nil {
		return &proto.SummaryResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "DOCUMENT_NOT_FOUND",
				Message: "Document not found in session",
			},
		}, nil
	}

	// Generate summary using AI service
	summary, takeaways, topics, err := uc.aiService.GenerateSummary(doc.Text)
	if err != nil {
		return &proto.SummaryResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "AI_SERVICE_ERROR",
				Message: fmt.Sprintf("Failed to generate summary: %v", err),
			},
		}, nil
	}

	return &proto.SummaryResponse{
		Status:        proto.Status_STATUS_SUCCESS,
		Summary:       summary,
		KeyTakeaways:  takeaways,
		MainTopics:    topics,
	}, nil
}

