package usecases

import (
	"fmt"
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/infrastructure/services"
	"ai-pdf-assistant-backend/proto"
)

// PDFUseCase handles PDF-related business logic
type PDFUseCase struct {
	docRepo    *repositories.DocumentRepository
	sessionRepo *repositories.SessionRepository
	pdfService  *services.PDFService
}

// NewPDFUseCase creates a new PDF use case
func NewPDFUseCase(
	docRepo *repositories.DocumentRepository,
	sessionRepo *repositories.SessionRepository,
	pdfService *services.PDFService,
) *PDFUseCase {
	return &PDFUseCase{
		docRepo:     docRepo,
		sessionRepo: sessionRepo,
		pdfService:  pdfService,
	}
}

// UploadPDF processes and stores a PDF file
func (uc *PDFUseCase) UploadPDF(filePath string, filename string) (*proto.UploadResponse, error) {
	// Process PDF
	doc, err := uc.pdfService.ProcessPDF(filePath, filename)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "PDF_PROCESSING_ERROR",
				Message: fmt.Sprintf("Failed to process PDF: %v", err),
			},
		}, nil
	}

	// Store document
	if err := uc.docRepo.Store(doc); err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "STORAGE_ERROR",
				Message: fmt.Sprintf("Failed to store document: %v", err),
			},
		}, nil
	}

	// Create session
	session, err := uc.sessionRepo.Create(doc.Id, doc)
	if err != nil {
		return &proto.UploadResponse{
			Status: proto.Status_STATUS_ERROR,
			Error: &proto.Error{
				Code:    "SESSION_ERROR",
				Message: fmt.Sprintf("Failed to create session: %v", err),
			},
		}, nil
	}

	return &proto.UploadResponse{
		Status:     proto.Status_STATUS_SUCCESS,
		Document:   doc,
		SessionId:  session.Id,
	}, nil
}

// GetDocumentStatus retrieves document status
func (uc *PDFUseCase) GetDocumentStatus(documentID string) (*proto.StatusResponse, error) {
	doc, err := uc.docRepo.Get(documentID)
	if err != nil {
		return &proto.StatusResponse{
			Status: proto.Status_STATUS_NOT_FOUND,
			Error: &proto.Error{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Document not found: %v", err),
			},
		}, nil
	}

	return &proto.StatusResponse{
		Status:   proto.Status_STATUS_SUCCESS,
		Document: doc,
	}, nil
}

