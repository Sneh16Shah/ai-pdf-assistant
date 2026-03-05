package repositories

import (
	"ai-pdf-assistant-backend/database"
	"log"
	"sync"
	"time"
)

// GlobalMetrics holds the public usage counters shown on the landing page
type GlobalMetrics struct {
	TotalUsers     int64 `json:"total_users"`
	TotalDocuments int64 `json:"total_documents"`
	TotalResponses int64 `json:"total_responses"` // assistant messages only
	CachedAt       int64 `json:"cached_at"`
}

// MetricsRepository queries global counters and caches them to avoid
// expensive COUNT(*) queries on every landing page visit.
type MetricsRepository struct {
	mu         sync.RWMutex
	cached     *GlobalMetrics
	cachedAt   time.Time
	cacheTTL   time.Duration
}

func NewMetricsRepository() *MetricsRepository {
	return &MetricsRepository{cacheTTL: 5 * time.Minute}
}

// GetGlobalMetrics returns cached metrics or re-queries Postgres if the cache is stale.
func (r *MetricsRepository) GetGlobalMetrics() GlobalMetrics {
	r.mu.RLock()
	if r.cached != nil && time.Since(r.cachedAt) < r.cacheTTL {
		m := *r.cached
		r.mu.RUnlock()
		return m
	}
	r.mu.RUnlock()

	m := r.fetchFromDB()

	r.mu.Lock()
	r.cached = &m
	r.cachedAt = time.Now()
	r.mu.Unlock()

	return m
}

func (r *MetricsRepository) fetchFromDB() GlobalMetrics {
	var m GlobalMetrics
	m.CachedAt = time.Now().Unix()

	if !database.IsConnected() {
		return m
	}

	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&m.TotalUsers); err != nil {
		log.Printf("[Metrics] Failed to count users: %v", err)
	}

	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&m.TotalDocuments); err != nil {
		log.Printf("[Metrics] Failed to count documents: %v", err)
	}

	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE role = 'assistant'`).Scan(&m.TotalResponses); err != nil {
		log.Printf("[Metrics] Failed to count responses: %v", err)
	}

	return m
}
