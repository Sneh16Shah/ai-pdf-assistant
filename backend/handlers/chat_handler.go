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

// ChatHandler handles chat-related HTTP requests
type ChatHandler struct {
	chatUseCase     *usecases.ChatUseCase
	persistenceRepo *repositories.PersistenceRepository
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatUseCase *usecases.ChatUseCase, persistenceRepo *repositories.PersistenceRepository) *ChatHandler {
	return &ChatHandler{
		chatUseCase:     chatUseCase,
		persistenceRepo: persistenceRepo,
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
		fmt.Printf("ERROR: Chat AskQuestion failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message: " + err.Error(),
		})
		return
	}

	// Convert Protobuf to JSON
	if resp.Status != proto.Status_STATUS_SUCCESS {
		fmt.Printf("ERROR: Chat response status=%v code=%s msg=%s\n", resp.Status, resp.Error.Code, resp.Error.Message)
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
		h.persistenceRepo.SaveMessage(&repositories.DBMessage{
			ID:        uuid.New().String(),
			SessionID: jsonReq.SessionID,
			Role:      "user",
			Content:   jsonReq.Message,
			CreatedAt: time.Now(),
		})

		// Save AI response
		var citationsJSON json.RawMessage
		if resp.Citations != nil {
			citationsJSON, _ = json.Marshal(resp.Citations)
		}
		h.persistenceRepo.SaveMessage(&repositories.DBMessage{
			ID:        uuid.New().String(),
			SessionID: jsonReq.SessionID,
			Role:      "assistant",
			Content:   resp.Response,
			Citations: citationsJSON,
			CreatedAt: time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"response":        resp.Response,
		"session_id":      resp.SessionId,
		"answer_found":    resp.AnswerFound,
		"relevant_chunks": resp.RelevantChunks,
		"citations":       resp.Citations,
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

// Stream handles SSE streaming chat requests
func (h *ChatHandler) Stream(c *gin.Context) {
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

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Convert JSON to Protobuf
	req := &proto.ChatRequest{
		SessionId: jsonReq.SessionID,
		Message:   jsonReq.Message,
	}

	// Get response from use case (non-streaming for now, but we'll stream to client)
	resp, err := h.chatUseCase.AskQuestion(req)
	if err != nil {
		c.SSEvent("error", gin.H{"message": "Failed to process message: " + err.Error()})
		return
	}

	if resp.Status != proto.Status_STATUS_SUCCESS {
		c.SSEvent("error", gin.H{"message": resp.Error.Message, "code": resp.Error.Code})
		return
	}

	// Stream the response word by word
	words := splitIntoChunks(resp.Response)
	for _, word := range words {
		c.SSEvent("token", gin.H{"content": word})
		c.Writer.Flush()
		time.Sleep(20 * time.Millisecond) // Small delay for streaming effect
	}

	// Send completion event with citations
	c.SSEvent("done", gin.H{
		"response":     resp.Response,
		"session_id":   resp.SessionId,
		"answer_found": resp.AnswerFound,
		"citations":    resp.Citations,
	})
	c.Writer.Flush()
}

// splitIntoChunks splits text into chunks for streaming
func splitIntoChunks(text string) []string {
	var chunks []string
	words := make([]string, 0)

	// Split by spaces but keep punctuation attached
	currentWord := ""
	for _, char := range text {
		if char == ' ' || char == '\n' {
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
			if char == '\n' {
				words = append(words, "\n")
			} else {
				words = append(words, " ")
			}
		} else {
			currentWord += string(char)
		}
	}
	if currentWord != "" {
		words = append(words, currentWord)
	}

	// Group into chunks of ~3-5 words for smoother streaming
	chunk := ""
	wordCount := 0
	for _, word := range words {
		chunk += word
		if word != " " && word != "\n" {
			wordCount++
		}
		if wordCount >= 3 {
			chunks = append(chunks, chunk)
			chunk = ""
			wordCount = 0
		}
	}
	if chunk != "" {
		chunks = append(chunks, chunk)
	}

	return chunks
}
