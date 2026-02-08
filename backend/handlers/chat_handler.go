package handlers

import (
	"net/http"
	"ai-pdf-assistant-backend/proto"
	"ai-pdf-assistant-backend/usecases"

	"github.com/gin-gonic/gin"
)

// ChatHandler handles chat-related HTTP requests
type ChatHandler struct {
	chatUseCase *usecases.ChatUseCase
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatUseCase *usecases.ChatUseCase) *ChatHandler {
	return &ChatHandler{
		chatUseCase: chatUseCase,
	}
}

// Message handles chat message requests
func (h *ChatHandler) Message(c *gin.Context) {
	var jsonReq struct {
		SessionID string `json:"session_id" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&jsonReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Convert JSON to Protobuf
	req := &proto.ChatRequest{
		SessionId: jsonReq.SessionID,
		Message:   jsonReq.Message,
	}

	// Call use case
	resp, err := h.chatUseCase.AskQuestion(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message: " + err.Error(),
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
		"response":        resp.Response,
		"session_id":      resp.SessionId,
		"answer_found":    resp.AnswerFound,
		"relevant_chunks": resp.RelevantChunks,
	})
}

// History handles chat history requests
func (h *ChatHandler) History(c *gin.Context) {
	sessionID := c.Param("sessionId")

	req := &proto.HistoryRequest{
		SessionId: sessionID,
	}

	resp, err := h.chatUseCase.GetHistory(req.SessionId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get history: " + err.Error(),
		})
		return
	}

	if resp.Status != proto.Status_STATUS_SUCCESS {
		statusCode := http.StatusNotFound
		c.JSON(statusCode, gin.H{
			"error": resp.Error.Message,
			"code":  resp.Error.Code,
		})
		return
	}

	// Convert Protobuf messages to JSON
	messages := make([]gin.H, len(resp.Session.Messages))
	for i, msg := range resp.Session.Messages {
		messages[i] = gin.H{
			"id":        msg.Id,
			"role":      msg.Role,
			"content":   msg.Content,
			"timestamp": msg.Timestamp,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": resp.Session.Id,
		"messages":   messages,
		"pdf_info": gin.H{
			"filename": resp.Session.Document.Filename,
			"pages":    resp.Session.Document.Pages,
		},
	})
}

// ClearSession handles session clear requests
func (h *ChatHandler) ClearSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	resp, err := h.chatUseCase.ClearSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clear session: " + err.Error(),
		})
		return
	}

	if resp.Status != proto.Status_STATUS_SUCCESS {
		statusCode := http.StatusNotFound
		c.JSON(statusCode, gin.H{
			"error": resp.Error.Message,
			"code":  resp.Error.Code,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"message":    resp.Message,
	})
}

