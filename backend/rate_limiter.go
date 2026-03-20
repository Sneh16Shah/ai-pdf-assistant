package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipLimiter holds a token-bucket rate limiter and the last time it was seen.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiterStore holds per-IP limiters and protects them with a mutex.
type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	// r = events per second, b = burst capacity.
	// 100 req/min ≈ 1.67 req/s. We use b=20 to allow short bursts.
	r rate.Limit
	b int
}

var globalLimiterStore = &rateLimiterStore{
	limiters: make(map[string]*ipLimiter),
	r:         rate.Every(600 * time.Millisecond), // ~100 req/min
	b:         20,
}

// getLimiter returns the rate limiter for the given IP, creating one if absent.
func (s *rateLimiterStore) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.limiters[ip]; ok {
		l.lastSeen = time.Now()
		return l.limiter
	}

	l := &ipLimiter{
		limiter:  rate.NewLimiter(s.r, s.b),
		lastSeen: time.Now(),
	}
	s.limiters[ip] = l
	return l.limiter
}

// cleanup removes IP entries that have been idle for more than 3 minutes,
// preventing the in-memory map from growing unboundedly under sustained attack.
func (s *rateLimiterStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ip, l := range s.limiters {
		if time.Since(l.lastSeen) > 3*time.Minute {
			delete(s.limiters, ip)
		}
	}
}

// startCleanupLoop periodically removes stale IP entries.
func init() {
	go func() {
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for range t.C {
			globalLimiterStore.cleanup()
		}
	}()
}

// rateLimitMiddleware returns a Gin middleware that enforces per-IP rate limits.
// It returns HTTP 429 Too Many Requests when the limit is exceeded.
func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract the real client IP (works behind reverse proxies too)
		clientIP := c.ClientIP()
		// Normalize: strip IPv6 brackets if present
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}

		limiter := globalLimiterStore.getLimiter(clientIP)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "too_many_requests",
				"message": "Rate limit exceeded. Please slow down and try again in a moment.",
			})
			return
		}
		c.Next()
	}
}
