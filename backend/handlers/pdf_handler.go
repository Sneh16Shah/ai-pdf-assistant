package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-gonic/gin"
)

// PDFHandler handles PDF-related HTTP requests
type PDFHandler struct {
	pdfUseCase *usecases.PDFUseCase
}

// NewPDFHandler creates a new PDF handler
func NewPDFHandler(pdfUseCase *usecases.PDFUseCase) *PDFHandler {
	return &PDFHandler{
		pdfUseCase: pdfUseCase,
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
		"id":      resp.Document.Id,
		"filename": resp.Document.Filename,
		"pages":   resp.Document.Pages,
		"chunks":  len(resp.Document.Chunks),
		"status":  "processed",
	})
}

