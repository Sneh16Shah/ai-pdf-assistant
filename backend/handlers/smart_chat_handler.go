package handlers

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SmartChatHandler handles smart chat HTTP requests with token optimization.
// All endpoints are under /api/v1/smart/ and mirror the standard chat endpoints
// but include token statistics in the response.
type SmartChatHandler struct {
	smartChatUseCase *usecases.SmartChatUseCase
	persistenceRepo  *repositories.PersistenceRepository
}

// NewSmartChatHandler creates a new smart chat handler
func NewSmartChatHandler(
	smartChatUseCase *usecases.SmartChatUseCase,
	persistenceRepo *repositories.PersistenceRepository,
) *SmartChatHandler {
	return &SmartChatHandler{
		smartChatUseCase: smartChatUseCase,
		persistenceRepo:  persistenceRepo,
	}
}

// Message handles smart chat message requests.
// Same input as /chat/message but returns additional token_stats.
func (h *SmartChatHandler) Message(c *gin.Context) {
	var jsonReq struct {
		SessionID  string `json:"session_id" binding:"required"`
		Message    string `json:"message" binding:"required"`
		PageNumber int32  `json:"page_number"`
	}

	if err := c.ShouldBindJSON(&jsonReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Convert JSON to Protobuf
	req := &proto.ChatRequest{
		SessionId:  jsonReq.SessionID,
		Message:    jsonReq.Message,
		PageNumber: jsonReq.PageNumber,
	}

	// Call smart use case
	resp, tokenStats, err := h.smartChatUseCase.AskQuestion(req)
	if err != nil {
		fmt.Printf("ERROR: Smart chat AskQuestion failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message: " + err.Error(),
		})
		return
	}

	// Handle non-success status
	if resp.Status != proto.Status_STATUS_SUCCESS {
		fmt.Printf("ERROR: Smart chat response status=%v code=%s msg=%s\n", resp.Status, resp.Error.Code, resp.Error.Message)
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

	// Persist messages to database if user is authenticated
	if _, exists := c.Get("userID"); exists {
		// Save user message
		if err := h.persistenceRepo.SaveMessage(&repositories.DBMessage{
			ID:        uuid.New().String(),
			SessionID: jsonReq.SessionID,
			Role:      "user",
			Content:   jsonReq.Message,
			CreatedAt: time.Now(),
		}); err != nil {
			fmt.Printf("ERROR: Failed to save user message: %v\n", err)
		}

		// Save AI response
		var citationsJSON json.RawMessage
		if resp.Citations != nil {
			citationsJSON, _ = json.Marshal(resp.Citations)
		}
		if err := h.persistenceRepo.SaveMessage(&repositories.DBMessage{
			ID:        uuid.New().String(),
			SessionID: jsonReq.SessionID,
			Role:      "assistant",
			Content:   resp.Response,
			Citations: citationsJSON,
			CreatedAt: time.Now(),
		}); err != nil {
			fmt.Printf("ERROR: Failed to save assistant message: %v\n", err)
		}
	}

	// Return response with token stats
	c.JSON(http.StatusOK, gin.H{
		"response":        resp.Response,
		"session_id":      resp.SessionId,
		"answer_found":    resp.AnswerFound,
		"relevant_chunks": resp.RelevantChunks,
		"citations":       resp.Citations,
		"image_base64":    resp.ImageBase64,
		"image_mime_type": resp.ImageMimeType,
		"token_stats":     tokenStats,
	})
}

// Stats returns cumulative token savings for a session
func (h *SmartChatHandler) Stats(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id is required",
		})
		return
	}

	stats := h.smartChatUseCase.GetSessionStats(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"stats":      stats,
	})
}
