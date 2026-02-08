package handlers

import (
	"net/http"
	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-gonic/gin"
)

// SummaryHandler handles summary-related HTTP requests
type SummaryHandler struct {
	summaryUseCase *usecases.SummaryUseCase
}

// NewSummaryHandler creates a new summary handler
func NewSummaryHandler(summaryUseCase *usecases.SummaryUseCase) *SummaryHandler {
	return &SummaryHandler{
		summaryUseCase: summaryUseCase,
	}
}

// Generate handles summary generation requests
func (h *SummaryHandler) Generate(c *gin.Context) {
	var jsonReq struct {
		SessionID  string `json:"session_id" binding:"required"`
		DocumentID string `json:"document_id"`
	}

	if err := c.ShouldBindJSON(&jsonReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Convert JSON to Protobuf
	req := &proto.SummaryRequest{
		SessionId:  jsonReq.SessionID,
		DocumentId: jsonReq.DocumentID,
	}

	// Call use case
	resp, err := h.summaryUseCase.GenerateSummary(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate summary: " + err.Error(),
		})
		return
	}

	// Convert Protobuf to JSON
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
		"summary":       resp.Summary,
		"key_takeaways": resp.KeyTakeaways,
		"main_topics":   resp.MainTopics,
	})
}

