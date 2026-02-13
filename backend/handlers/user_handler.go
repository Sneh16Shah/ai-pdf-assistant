package handlers

import (
	"net/http"

	"ai-pdf-assistant-backend/infrastructure/repositories"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests (sessions, dashboard)
type UserHandler struct {
	persistenceRepo *repositories.PersistenceRepository
}

// NewUserHandler creates a new user handler
func NewUserHandler(persistenceRepo *repositories.PersistenceRepository) *UserHandler {
	return &UserHandler{persistenceRepo: persistenceRepo}
}

// GetSessions returns all sessions for the authenticated user
func (h *UserHandler) GetSessions(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	sessions, err := h.persistenceRepo.GetUserSessions(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sessions"})
		return
	}

	if sessions == nil {
		sessions = []repositories.DBSession{}
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// GetSessionMessages returns all messages for a specific session
func (h *UserHandler) GetSessionMessages(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	sessionID := c.Param("sessionId")

	// Verify session belongs to user
	sessions, err := h.persistenceRepo.GetUserSessions(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify session"})
		return
	}

	found := false
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusForbidden, gin.H{"error": "Session not found or access denied"})
		return
	}

	messages, err := h.persistenceRepo.GetSessionMessages(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	if messages == nil {
		messages = []repositories.DBMessage{}
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// DeleteSession deletes a session for the authenticated user
func (h *UserHandler) DeleteSession(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	sessionID := c.Param("sessionId")

	// Verify session belongs to user
	sessions, err := h.persistenceRepo.GetUserSessions(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify session"})
		return
	}

	found := false
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusForbidden, gin.H{"error": "Session not found or access denied"})
		return
	}

	if err := h.persistenceRepo.DeleteSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}
