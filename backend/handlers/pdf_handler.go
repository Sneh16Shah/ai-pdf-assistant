package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-gonic/gin"
)

// sanitizeFilename strips all directory traversal sequences from a filename.
// It returns only the base filename, rejecting names that are still
// suspicious (empty or start with a dot).
func sanitizeFilename(name string) (string, bool) {
	base := filepath.Base(filepath.Clean(name))
	if base == "." || base == "" || strings.HasPrefix(base, "..") {
		return "", false
	}
	return base, true
}

// validatePDF reads the first 512 bytes from the reader to detect the MIME type
// and confirms it is application/pdf. It returns the full reader with the peeked
// bytes prepended so the caller can still read the complete file.
func validatePDF(r io.Reader) (io.Reader, error) {
	header := make([]byte, 512)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	header = header[:n]

	mime := http.DetectContentType(header)
	if mime != "application/pdf" {
		return nil, &pdfValidationError{mime: mime}
	}
	// Re-assemble the full reader: prepend the already-read header bytes.
	return io.MultiReader(bytes.NewReader(header), r), nil
}

type pdfValidationError struct{ mime string }

func (e *pdfValidationError) Error() string {
	return "uploaded file is not a PDF (detected: " + e.mime + ")"
}

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

	// --- FIX CVE-4: Validate file is actually a PDF before touching disk ---
	safeReader, err := validatePDF(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type: " + err.Error(),
		})
		return
	}

	// --- FIX CVE-1: Sanitize filename to prevent path traversal ---
	filename, ok := sanitizeFilename(header.Filename)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file: " + err.Error(),
		})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, safeReader)
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

// Rehydrate restores a session's in-memory state by re-processing a PDF
// without creating a new session in the database. Used when resuming sessions
// after a backend restart.
func (h *PDFHandler) Rehydrate(c *gin.Context) {
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id is required",
		})
		return
	}

	file, header, err := c.Request.FormFile("pdf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Save uploaded file
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	os.MkdirAll(uploadDir, 0755)

	// --- FIX CVE-4: Validate file is actually a PDF before touching disk ---
	safeReader, err := validatePDF(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type: " + err.Error(),
		})
		return
	}

	// --- FIX CVE-1: Sanitize filename to prevent path traversal ---
	filename, ok := sanitizeFilename(header.Filename)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file: " + err.Error(),
		})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, safeReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to copy file: " + err.Error(),
		})
		return
	}

	// Re-hydrate the session (process PDF + load into existing session ID)
	resp, err := h.pdfUseCase.RehydrateSession(sessionID, filePath, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to rehydrate session: " + err.Error(),
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

	c.JSON(http.StatusOK, gin.H{
		"document_id": resp.Document.Id,
		"session_id":  resp.SessionId,
		"filename":    resp.Document.Filename,
		"pages":       resp.Document.Pages,
		"chunks":      len(resp.Document.Chunks),
		"message":     "Session rehydrated successfully",
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

	// Check database first (authoritative source — IDs match DownloadDocument)
	if h.persistenceRepo != nil {
		dbDocs, dbErr := h.persistenceRepo.GetSessionDocuments(sessionID)
		if dbErr == nil && len(dbDocs) > 0 {
			documents := make([]gin.H, len(dbDocs))
			for i, doc := range dbDocs {
				documents[i] = gin.H{
					"id":       doc.ID,
					"filename": doc.Filename,
					"pages":    doc.Pages,
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"documents": documents,
				"count":     len(documents),
			})
			return
		}
	}

	// Fall back to in-memory session (for sessions not yet persisted to DB)
	docs, err := h.pdfUseCase.GetSessionDocuments(sessionID)
	if err == nil && len(docs) > 0 {
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
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error": "Session not found or has no documents",
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

	// --- FIX CVE-4: Validate file is actually a PDF before touching disk ---
	safeReader, err := validatePDF(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type: " + err.Error(),
		})
		return
	}

	// --- FIX CVE-1: Sanitize filename to prevent path traversal ---
	filename, ok := sanitizeFilename(header.Filename)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}
	filePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file: " + err.Error(),
		})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, safeReader)
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

// DownloadDocument serves the stored PDF file for a given document ID
func (h *PDFHandler) DownloadDocument(c *gin.Context) {
	documentID := c.Param("documentId")

	doc, err := h.persistenceRepo.GetDocumentByID(documentID)
	if err != nil || doc == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Document not found",
		})
		return
	}

	if doc.FilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File path not available",
		})
		return
	}

	// Check if file exists on disk
	if _, err := os.Stat(doc.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found on disk",
		})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "inline; filename=\""+doc.Filename+"\"")
	c.File(doc.FilePath)
}
