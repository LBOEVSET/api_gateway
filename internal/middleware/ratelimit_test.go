package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func ratelimitRouter(rateLimit int, rateWindow time.Duration, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimit:     rateLimit,
		RateWindow:    rateWindow,
		RateBurstSize: burst,
	}
	r := gin.New()
	r.Use(RateLimit(cfg))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRateLimit_AllowsInitialRequests(t *testing.T) {
	// 100 req/min with burst of 5 — first 5 should pass immediately
	r := ratelimitRouter(100, time.Minute, 5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i+1)
	}
}

func TestRateLimit_BlocksAfterBurstExceeded(t *testing.T) {
	// 1 req/min with burst of 1 — second request should be blocked
	r := ratelimitRouter(1, time.Minute, 1)

	// First request — consumes the single burst token
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.2:1234"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request — no tokens left
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestRateLimit_TooManyRequestsResponse_HasRetryAfter(t *testing.T) {
	r := ratelimitRouter(1, time.Minute, 1)

	// Exhaust burst
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.3:1234"
	httptest.NewRecorder()
	r.ServeHTTP(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.3:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.Contains(t, w2.Body.String(), "retryAfter")
}

func TestRateLimit_DifferentIPsHaveSeparateLimits(t *testing.T) {
	// 1 req/min burst 1 — different IPs should each get their own bucket
	r := ratelimitRouter(1, time.Minute, 1)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0." + string(rune('4'+i)) + ":1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "first request from IP %d should pass", i)
	}
}

func TestRateLimiterStore_GetCreatesNewLimiter(t *testing.T) {
	store := newRateLimiterStore(10, 5)
	l1 := store.get("192.168.1.1")
	l2 := store.get("192.168.1.2")
	assert.NotNil(t, l1)
	assert.NotNil(t, l2)
	assert.NotEqual(t, l1, l2)
}

func TestRateLimiterStore_GetReturnsSameLimiter(t *testing.T) {
	store := newRateLimiterStore(10, 5)
	l1 := store.get("192.168.1.10")
	l2 := store.get("192.168.1.10")
	assert.Equal(t, l1, l2)
}
