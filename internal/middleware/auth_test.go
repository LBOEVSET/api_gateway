package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-jwt-secret"

func makeToken(claims jwt.MapClaims, secret string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"id":    "user-123",
		"role":  "CUSTOMER",
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
}

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret:      testSecret,
		InternalSecret: "internal-secret",
	}
}

func authRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))
	r.GET("/public/route", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/products", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/products", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/v1/statistics", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/orders", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// ─── isPublic ────────────────────────────────────────────────────────────────

func TestIsPublic_AuthRoutes(t *testing.T) {
	publicPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/auth/send/otp",
		"/api/v1/auth/verify/otp",
		"/api/v1/auth/guest/init",
		"/api/v1/payments/webhook",
		"/api/v1/statistics",
		"/api/v1/health-check",
		"/health",
	}
	for _, p := range publicPaths {
		assert.True(t, isPublic("GET", p), "expected %s to be public", p)
		assert.True(t, isPublic("POST", p), "expected %s to be public", p)
	}
}

func TestIsPublic_ReadOnlyPublicPrefixes(t *testing.T) {
	readOnlyPaths := []string{
		"/api/v1/products",
		"/api/v1/products/123",
		"/api/v1/articles",
		"/api/v1/events",
		"/api/v1/merchandise",
		"/api/v1/subscription/plans",
		"/api/v1/subscription/tiers",
		"/api/v1/profile/file/abc",
	}
	for _, p := range readOnlyPaths {
		assert.True(t, isPublic("GET", p), "GET %s should be public", p)
		assert.False(t, isPublic("POST", p), "POST %s should NOT be public", p)
		assert.False(t, isPublic("DELETE", p), "DELETE %s should NOT be public", p)
	}
}

func TestIsPublic_ProtectedRoutes(t *testing.T) {
	assert.False(t, isPublic("GET", "/api/v1/orders"))
	assert.False(t, isPublic("POST", "/api/v1/cart"))
	assert.False(t, isPublic("GET", "/api/v1/profile"))
	assert.False(t, isPublic("DELETE", "/api/v1/products/123"))
}

// ─── extractToken ─────────────────────────────────────────────────────────────

func TestExtractToken_FromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "cookie-token"})
	c.Request = req

	token, err := extractToken(c)
	require.NoError(t, err)
	assert.Equal(t, "cookie-token", token)
}

func TestExtractToken_FromBearerHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	c.Request = req

	token, err := extractToken(c)
	require.NoError(t, err)
	assert.Equal(t, "header-token", token)
}

func TestExtractToken_CookieTakesPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "cookie-token"})
	req.Header.Set("Authorization", "Bearer header-token")
	c.Request = req

	token, err := extractToken(c)
	require.NoError(t, err)
	assert.Equal(t, "cookie-token", token)
}

func TestExtractToken_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	token, err := extractToken(c)
	require.NoError(t, err)
	assert.Empty(t, token)
}

// ─── parseToken ──────────────────────────────────────────────────────────────

func TestParseToken_Valid(t *testing.T) {
	claims := validClaims()
	tokenStr := makeToken(claims, testSecret)

	parsed, err := parseToken(tokenStr, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-123", parsed["id"])
	assert.Equal(t, "CUSTOMER", parsed["role"])
}

func TestParseToken_WrongSecret(t *testing.T) {
	tokenStr := makeToken(validClaims(), testSecret)
	_, err := parseToken(tokenStr, "wrong-secret")
	assert.Error(t, err)
}

func TestParseToken_Expired(t *testing.T) {
	claims := jwt.MapClaims{
		"id":  "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	tokenStr := makeToken(claims, testSecret)
	_, err := parseToken(tokenStr, testSecret)
	assert.Error(t, err)
}

func TestParseToken_Malformed(t *testing.T) {
	_, err := parseToken("not.a.token", testSecret)
	assert.Error(t, err)
}

// ─── Auth middleware ─────────────────────────────────────────────────────────

func TestAuth_PublicRoute_NoToken(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_PublicRoute_StatisticsNoToken(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("POST", "/api/v1/statistics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_ReadOnlyPublicGET_NoToken(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("GET", "/api/v1/products", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_ReadOnlyPublicPOST_NoToken_Returns401(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("POST", "/api/v1/products", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ProtectedRoute_NoToken_Returns401(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ProtectedRoute_ValidToken(t *testing.T) {
	cfg := testConfig()
	r := authRouter(cfg)
	tokenStr := makeToken(validClaims(), testSecret)

	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: tokenStr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_ProtectedRoute_InvalidToken_Returns401(t *testing.T) {
	r := authRouter(testConfig())
	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: "invalid.token.here"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken_SetsUserHeaders(t *testing.T) {
	cfg := testConfig()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))

	var capturedID, capturedRole, capturedEmail string
	r.GET("/api/v1/orders", func(c *gin.Context) {
		capturedID = c.Request.Header.Get("X-User-ID")
		capturedRole = c.Request.Header.Get("X-User-Role")
		capturedEmail = c.Request.Header.Get("X-User-Email")
		c.Status(http.StatusOK)
	})

	tokenStr := makeToken(validClaims(), testSecret)
	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "user-123", capturedID)
	assert.Equal(t, "CUSTOMER", capturedRole)
	assert.Equal(t, "test@example.com", capturedEmail)
}

func TestAuth_AlwaysSetsInternalSecret(t *testing.T) {
	cfg := testConfig()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))

	var capturedSecret string
	r.GET("/health", func(c *gin.Context) {
		capturedSecret = c.Request.Header.Get("X-Internal-Secret")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "internal-secret", capturedSecret)
}

func TestAuth_TokenWithNoRole_DefaultsToCustomer(t *testing.T) {
	cfg := testConfig()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(cfg))

	var capturedRole string
	r.GET("/api/v1/orders", func(c *gin.Context) {
		capturedRole = c.Request.Header.Get("X-User-Role")
		c.Status(http.StatusOK)
	})

	claims := jwt.MapClaims{
		"id":  "user-456",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tokenStr := makeToken(claims, testSecret)
	req := httptest.NewRequest("GET", "/api/v1/orders", nil)
	req.AddCookie(&http.Cookie{Name: "accessToken", Value: tokenStr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "CUSTOMER", capturedRole)
}
