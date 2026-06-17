package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const routerTestSecret = "router-test-secret"

func testRouterConfig() *config.Config {
	return &config.Config{
		JWTSecret:         routerTestSecret,
		JWTRefreshSecret:  "refresh-secret",
		BackendURL:        "http://backend-does-not-exist.local",
		PaymentGatewayURL: "http://payment-does-not-exist.local",
		RateLimit:         1000,
		RateWindow:        time.Minute,
		RateBurstSize:     100,
		MaxFileSizeBytes:  10 * 1024 * 1024,
		MaxFileCount:      5,
		AllowedOrigins:    []string{"http://localhost:3022"},
		InternalSecret:    "internal-test-secret",
		Zone:              "test",
	}
}

func makeRouterToken(userID, role string, secret string) string {
	claims := jwt.MapClaims{
		"id":    userID,
		"role":  role,
		"email": userID + "@test.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

// ─── Health check ─────────────────────────────────────────────────────────────

func TestRouter_HealthCheck(t *testing.T) {
	r := New(testRouterConfig())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "api-gateway", body["service"])
}

// ─── Auth-gated proxy routes return 401 without token ─────────────────────────

func TestRouter_ProtectedRoutes_NoToken_Returns401(t *testing.T) {
	r := New(testRouterConfig())
	protectedRoutes := []struct{ method, path string }{
		{"GET", "/api/v1/orders"},
		{"POST", "/api/v1/cart"},
		{"GET", "/api/v1/profile"},
		{"POST", "/api/v1/dashboard"},
		{"GET", "/api/v1/support-tickets"},
	}

	for _, route := range protectedRoutes {
		req := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"%s %s should return 401", route.method, route.path)
	}
}

// ─── Public GET routes pass without token ─────────────────────────────────────

func TestRouter_PublicGETRoutes_NoToken_ProxyAttempted(t *testing.T) {
	r := New(testRouterConfig())
	// These should NOT return 401 — they'll return 502 (upstream unreachable)
	// which proves the auth gate passed them through.
	publicGets := []string{
		"/api/v1/products",
		"/api/v1/articles",
		"/api/v1/events",
		"/api/v1/merchandise",
	}

	for _, path := range publicGets {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// 502 = proxy attempted (not 401 = auth blocked)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code,
			"GET %s should not be blocked by auth", path)
	}
}

// ─── Auth endpoints are public ────────────────────────────────────────────────

func TestRouter_AuthEndpoints_ArePublic(t *testing.T) {
	r := New(testRouterConfig())
	authRoutes := []struct{ method, path string }{
		{"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/auth/register"},
		{"POST", "/api/v1/auth/refresh"},
	}

	for _, route := range authRoutes {
		req := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// 502 = auth passed, proxy attempted (not 401)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code,
			"%s %s should be public", route.method, route.path)
	}
}

// ─── Statistics is public ─────────────────────────────────────────────────────

func TestRouter_StatisticsPost_IsPublic(t *testing.T) {
	r := New(testRouterConfig())
	req := httptest.NewRequest("POST", "/api/v1/statistics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

// ─── Valid token allows protected routes ──────────────────────────────────────

func TestRouter_ValidToken_AllowsProtectedRoutes(t *testing.T) {
	cfg := testRouterConfig()
	r := New(cfg)

	tokenStr := makeRouterToken("user-1", "CUSTOMER", routerTestSecret)
	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: tokenStr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 502 = auth passed (upstream unreachable in test), not 401
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// ─── CORS headers ─────────────────────────────────────────────────────────────

func TestRouter_CORS_OptionsRequest(t *testing.T) {
	r := New(testRouterConfig())
	req := httptest.NewRequest("OPTIONS", "/api/v1/products", nil)
	req.Header.Set("Origin", "http://localhost:3022")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:3022", w.Header().Get("Access-Control-Allow-Origin"))
}

// ─── X-Request-ID is set ──────────────────────────────────────────────────────

func TestRouter_RequestID_IsSetInResponse(t *testing.T) {
	r := New(testRouterConfig())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRouter_ExistingRequestID_IsPreserved(t *testing.T) {
	r := New(testRouterConfig())
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", "my-trace-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "my-trace-id", w.Header().Get("X-Request-ID"))
}

// ─── Proxy returns 502 for unreachable upstreams ──────────────────────────────

func TestRouter_BackendProxy_Returns502_WhenUnreachable(t *testing.T) {
	cfg := testRouterConfig()
	r := New(cfg)

	tokenStr := makeRouterToken("user-1", "ADMIN", routerTestSecret)
	req := httptest.NewRequest("GET", "/api/v1/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: tokenStr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
