package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	limit    rate.Limit
	burst    int
}

func newRateLimiterStore(rps float64, burst int) *rateLimiterStore {
	store := &rateLimiterStore{
		limiters: make(map[string]*ipLimiter),
		limit:    rate.Limit(rps),
		burst:    burst,
	}
	// Evict stale entries every 5 minutes
	go store.cleanup()
	return store
}

func (s *rateLimiterStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	if il, ok := s.limiters[ip]; ok {
		il.lastSeen = time.Now()
		return il.limiter
	}
	l := rate.NewLimiter(s.limit, s.burst)
	s.limiters[ip] = &ipLimiter{limiter: l, lastSeen: time.Now()}
	return l
}

func (s *rateLimiterStore) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		for ip, il := range s.limiters {
			if time.Since(il.lastSeen) > 10*time.Minute {
				delete(s.limiters, ip)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimit applies a per-IP token bucket rate limiter.
func RateLimit(cfg *config.Config) gin.HandlerFunc {
	// Convert window-based config to requests-per-second
	rps := float64(cfg.RateLimit) / cfg.RateWindow.Seconds()
	store := newRateLimiterStore(rps, cfg.RateBurstSize)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := store.get(ip)

		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message":     "Too many requests. Please slow down.",
				"statusCode":  429,
				"retryAfter":  "60s",
			})
			return
		}
		c.Next()
	}
}
