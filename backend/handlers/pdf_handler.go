package handlers

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-gonic/gin"
)

// PDFHandler handles PDF-related HTTP requests
type PDFHandler struct {
	pdfUseCase      *usecases.PDFUseCase
	persistenceRepo *repositories.PersistenceRepository
}

// NewPDFHandler creates a new PDF handler
func NewPDFHandler(pdfUseCase *usecases.PDFUseCase, persistenceRepo *repositories.PersistenceRepository) *PDFHandler {
	return &PDFHandler{
		pdfUseCase:      pdfUseCase,
		persistenceRepo: persistenceRepo,
	}
}

// Upload handles PDF upload requests
func (h *PDFHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Create upload directory if it doesn't exist
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	os.MkdirAll(uploadDir, 0755)

	// Save uploaded file
	filename := header.Filename
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file: " + err.Error(),
		})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to copy file: " + err.Error(),
		})
		return
	}

	// Process PDF
	resp, err := h.pdfUseCase.UploadPDF(filePath, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process PDF: " + err.Error(),
		})
		return
	}

	// Convert Protobuf response to JSON
	if resp.Status != proto.Status_STATUS_SUCCESS {
		statusCode := http.StatusInternalServerError
		if resp.Status == proto.Status_STATUS_NOT_FOUND {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{
			"error": resp.Error.Message,
			"code":  resp.Error.Code,
		})
		return
	}

	// Persist to database if user is authenticated
	if userID, exists := c.Get("userID"); exists {
		now := time.Now()
		if err := h.persistenceRepo.SaveSession(&repositories.DBSession{
			ID:           resp.SessionId,
			UserID:       userID.(string),
			Title:        filename,
			CreatedAt:    now,
			LastActivity: now,
		}); err != nil {
			log.Printf("Failed to persist session: %v", err)
		}
		if err := h.persistenceRepo.SaveDocument(&repositories.DBDocument{
			ID:          resp.Document.Id,
			SessionID:   resp.SessionId,
			Filename:    filename,
			FilePath:    filePath,
			Pages:       int(resp.Document.Pages),
			ChunksCount: len(resp.Document.Chunks),
			UploadedAt:  now,
		}); err != nil {
			log.Printf("Failed to persist document: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id": resp.Document.Id,
		"session_id":  resp.SessionId,
		"filename":    resp.Document.Filename,
		"pages":       resp.Document.Pages,
		"chunks":      len(resp.Document.Chunks),
		"message":     "PDF uploaded and processed successfully",
	})
}

// Status handles document status requests
func (h *PDFHandler) Status(c *gin.Context) {
	documentID := c.Param("id")

	req := &proto.StatusRequest{
		DocumentId: documentID,
	}

	resp, err := h.pdfUseCase.GetDocumentStatus(req.DocumentId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get document status: " + err.Error(),
		})
		return
	}

	if resp.Status != proto.Status_STATUS_SUCCESS {
		statusCode := http.StatusNotFound
		if resp.Status == proto.Status_STATUS_ERROR {
			statusCode = http.StatusInternalServerError
		}
		c.JSON(statusCode, gin.H{
			"error": resp.Error.Message,
			"code":  resp.Error.Code,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       resp.Document.Id,
		"filename": resp.Document.Filename,
		"pages":    resp.Document.Pages,
		"chunks":   len(resp.Document.Chunks),
		"status":   "processed",
	})
}

// ListSessionDocuments returns all documents in a session
func (h *PDFHandler) ListSessionDocuments(c *gin.Context) {
	sessionID := c.Param("sessionId")

	docs, err := h.pdfUseCase.GetSessionDocuments(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Convert documents to a simpler format
	documents := make([]gin.H, len(docs))
	for i, doc := range docs {
		documents[i] = gin.H{
			"id":       doc.Id,
			"filename": doc.Filename,
			"pages":    doc.Pages,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"count":     len(documents),
	})
}

// AddToSession handles adding a PDF to an existing session
func (h *PDFHandler) AddToSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Create upload directory if it doesn't exist
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	os.MkdirAll(uploadDir, 0755)

	// Save uploaded file
	filename := header.Filename
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file: " + err.Error(),
		})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to copy file: " + err.Error(),
		})
		return
	}

	// Add PDF to existing session
	resp, err := h.pdfUseCase.AddDocumentToSession(sessionID, filePath, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process PDF: " + err.Error(),
		})
		return
	}

	if resp.Status != proto.Status_STATUS_SUCCESS {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": resp.Error.Message,
			"code":  resp.Error.Code,
		})
		return
	}

	// Persist to database if user is authenticated
	if userID, exists := c.Get("userID"); exists {
		_ = userID // session already saved, just save the document
		if err := h.persistenceRepo.SaveDocument(&repositories.DBDocument{
			ID:          resp.Document.Id,
			SessionID:   sessionID,
			Filename:    filename,
			FilePath:    filePath,
			Pages:       int(resp.Document.Pages),
			ChunksCount: len(resp.Document.Chunks),
			UploadedAt:  time.Now(),
		}); err != nil {
			log.Printf("Failed to persist document: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id": resp.Document.Id,
		"session_id":  sessionID,
		"filename":    resp.Document.Filename,
		"pages":       resp.Document.Pages,
		"chunks":      len(resp.Document.Chunks),
		"message":     "PDF added to session successfully",
	})
}

// DeleteDocument removes a document from a session
func (h *PDFHandler) DeleteDocument(c *gin.Context) {
	sessionID := c.Query("session_id")
	documentID := c.Param("documentId")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id query parameter is required",
		})
		return
	}

	err := h.pdfUseCase.RemoveDocumentFromSession(sessionID, documentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Document removed successfully",
	})
}
