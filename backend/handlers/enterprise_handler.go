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

// EnterpriseHandler handles enterprise RAG pipeline HTTP requests.
// Endpoints are under /api/v1/enterprise/ and provide:
// - Chat with hybrid retrieval + reranking + citations
// - Session stats with grounding scores
// - Pipeline comparison endpoint
type EnterpriseHandler struct {
	enterpriseUseCase *usecases.EnterpriseChatUseCase
	smartUseCase      *usecases.SmartChatUseCase
	standardUseCase   *usecases.ChatUseCase
	persistenceRepo   *repositories.PersistenceRepository
}

// NewEnterpriseHandler creates a new enterprise chat handler
func NewEnterpriseHandler(
	enterpriseUseCase *usecases.EnterpriseChatUseCase,
	smartUseCase *usecases.SmartChatUseCase,
	standardUseCase *usecases.ChatUseCase,
	persistenceRepo *repositories.PersistenceRepository,
) *EnterpriseHandler {
	return &EnterpriseHandler{
		enterpriseUseCase: enterpriseUseCase,
		smartUseCase:      smartUseCase,
		standardUseCase:   standardUseCase,
		persistenceRepo:   persistenceRepo,
	}
}

// Message handles enterprise chat message requests.
// Returns response with citations, hybrid retrieval stats, and grounding score.
func (h *EnterpriseHandler) Message(c *gin.Context) {
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

	req := &proto.ChatRequest{
		SessionId:  jsonReq.SessionID,
		Message:    jsonReq.Message,
		PageNumber: jsonReq.PageNumber,
	}

	resp, queryStats, err := h.enterpriseUseCase.AskQuestion(req)
	if err != nil {
		fmt.Printf("ERROR: Enterprise chat AskQuestion failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process message: " + err.Error(),
		})
		return
	}

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

	// Persist messages to database if user is authenticated
	if _, exists := c.Get("userID"); exists {
		if err := h.persistenceRepo.SaveMessage(&repositories.DBMessage{
			ID:        uuid.New().String(),
			SessionID: jsonReq.SessionID,
			Role:      "user",
			Content:   jsonReq.Message,
			CreatedAt: time.Now(),
		}); err != nil {
			fmt.Printf("ERROR: Failed to save user message: %v\n", err)
		}

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

	c.JSON(http.StatusOK, gin.H{
		"response":        resp.Response,
		"session_id":      resp.SessionId,
		"answer_found":    resp.AnswerFound,
		"relevant_chunks": resp.RelevantChunks,
		"citations":       resp.Citations,
		"pipeline":        "enterprise",
		"query_stats":     queryStats,
	})
}

// Stats returns cumulative enterprise pipeline stats for a session
func (h *EnterpriseHandler) Stats(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id is required",
		})
		return
	}

	stats := h.enterpriseUseCase.GetSessionStats(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"pipeline":   "enterprise",
		"stats":      stats,
	})
}

// Compare runs the same query through all three pipelines and returns
// a side-by-side comparison of their performance. This is the key
// endpoint for demonstrating enterprise RAG advantages on a resume.
func (h *EnterpriseHandler) Compare(c *gin.Context) {
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

	req := &proto.ChatRequest{
		SessionId:  jsonReq.SessionID,
		Message:    jsonReq.Message,
		PageNumber: jsonReq.PageNumber,
	}

	type pipelineResult struct {
		Pipeline  string      `json:"pipeline"`
		Response  string      `json:"response"`
		Citations interface{} `json:"citations,omitempty"`
		Stats     interface{} `json:"stats,omitempty"`
		Error     string      `json:"error,omitempty"`
		LatencyMs int64       `json:"latency_ms"`
	}

	results := make([]pipelineResult, 0, 3)

	// === 1. Standard Pipeline ===
	startStd := time.Now()
	stdResp, stdErr := h.standardUseCase.AskQuestion(req)
	stdLatency := time.Since(startStd).Milliseconds()
	stdResult := pipelineResult{
		Pipeline:  "standard",
		LatencyMs: stdLatency,
	}
	if stdErr != nil {
		stdResult.Error = stdErr.Error()
	} else if stdResp.Status == proto.Status_STATUS_SUCCESS {
		stdResult.Response = stdResp.Response
		stdResult.Citations = stdResp.Citations
	} else if stdResp.Error != nil {
		stdResult.Error = stdResp.Error.Message
	}
	results = append(results, stdResult)

	// Add slight delay to prevent hitting free tier rate limits (burst requests)
	time.Sleep(2 * time.Second)

	// === 2. Smart Pipeline ===
	startSmart := time.Now()
	smartResp, smartTokenStats, smartErr := h.smartUseCase.AskQuestion(req)
	smartLatency := time.Since(startSmart).Milliseconds()
	smartResult := pipelineResult{
		Pipeline:  "smart",
		LatencyMs: smartLatency,
	}
	if smartErr != nil {
		smartResult.Error = smartErr.Error()
	} else if smartResp.Status == proto.Status_STATUS_SUCCESS {
		smartResult.Response = smartResp.Response
		smartResult.Citations = smartResp.Citations
		smartResult.Stats = smartTokenStats
	} else if smartResp.Error != nil {
		smartResult.Error = smartResp.Error.Message
	}
	results = append(results, smartResult)

	// Add slight delay to prevent hitting free tier rate limits (burst requests)
	time.Sleep(2 * time.Second)

	// === 3. Enterprise Pipeline ===
	startEnt := time.Now()
	entResp, entStats, entErr := h.enterpriseUseCase.AskQuestion(req)
	entLatency := time.Since(startEnt).Milliseconds()
	entResult := pipelineResult{
		Pipeline:  "enterprise",
		LatencyMs: entLatency,
	}
	if entErr != nil {
		entResult.Error = entErr.Error()
	} else if entResp.Status == proto.Status_STATUS_SUCCESS {
		entResult.Response = entResp.Response
		entResult.Citations = entResp.Citations
		entResult.Stats = entStats
	} else if entResp.Error != nil {
		entResult.Error = entResp.Error.Message
	}
	results = append(results, entResult)

	c.JSON(http.StatusOK, gin.H{
		"query":   jsonReq.Message,
		"results": results,
	})
}
