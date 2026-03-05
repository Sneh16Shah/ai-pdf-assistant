package handlers

import (
	"ai-pdf-assistant-backend/infrastructure/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MetricsHandler serves the public global metrics endpoint (no auth required).
type MetricsHandler struct {
	repo *repositories.MetricsRepository
}

func NewMetricsHandler(repo *repositories.MetricsRepository) *MetricsHandler {
	return &MetricsHandler{repo: repo}
}

// GetMetrics returns global usage counters for the landing page.
// GET /api/v1/metrics — public, no JWT required.
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	metrics := h.repo.GetGlobalMetrics()
	c.JSON(http.StatusOK, metrics)
}
