package middleware

import (
	"net/http"
	"strings"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// publicRoutes lists path prefixes that do NOT require authentication.
// The gateway still tries to extract user info on these routes (for optional auth),
// but will not reject unauthenticated requests.
var publicRoutes = []string{
	"/health",               // gateway health check
	"/api/v1/auth/guest/init",
	"/api/v1/auth/register",
	"/api/v1/auth/login",
	"/api/v1/auth/refresh",
	"/api/v1/auth/send/otp",
	"/api/v1/auth/verify/otp",
	"/api/v1/payments/webhook",
	"/api/v1/health-check",
	// Statistics are analytics-only — allow without auth so guest sessions
	// can record events. Batched to minimise request count.
	"/api/v1/statistics",
}

// readOnlyPublicPrefixes are GET-only public prefixes.
var readOnlyPublicPrefixes = []string{
	"/api/v1/products",
	"/api/v1/articles",
	"/api/v1/events",
	"/api/v1/merchandise",
}

func isPublic(method, path string) bool {
	for _, p := range publicRoutes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	if method == http.MethodGet {
		for _, p := range readOnlyPublicPrefixes {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
	}
	return false
}

// Auth is the JWT authentication middleware.
// It reads the accessToken cookie, verifies it, and injects trusted headers
// (X-User-ID, X-User-Role, X-User-Email) for downstream services.
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Always forward the internal secret so downstream services know
		// the request came through the gateway — regardless of auth status.
		c.Request.Header.Set("X-Internal-Secret", cfg.InternalSecret)

		tokenStr, err := extractToken(c)
		if err != nil || tokenStr == "" {
			if isPublic(c.Request.Method, c.Request.URL.Path) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			return
		}

		claims, err := parseToken(tokenStr, cfg.JWTSecret)
		if err != nil {
			if isPublic(c.Request.Method, c.Request.URL.Path) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token"})
			return
		}

		// Forward trusted user context to downstream services
		userID, _ := claims["id"].(string)
		userRole, _ := claims["role"].(string)
		userEmail, _ := claims["email"].(string)
		guestID, _ := claims["guestId"].(string)

		if userRole == "" && userID != "" {
			userRole = "CUSTOMER"
		}

		c.Request.Header.Set("X-User-ID", userID)
		c.Request.Header.Set("X-User-Role", userRole)
		c.Request.Header.Set("X-User-Email", userEmail)
		c.Request.Header.Set("X-Guest-ID", guestID)

		c.Set("userID", userID)
		c.Set("userRole", userRole)
		c.Set("userEmail", userEmail)

		c.Next()
	}
}

// extractToken reads JWT from cookie first, then Authorization Bearer header.
func extractToken(c *gin.Context) (string, error) {
	if cookie, err := c.Cookie("accessToken"); err == nil && cookie != "" {
		return cookie, nil
	}
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), nil
	}
	return "", nil
}

func parseToken(tokenStr, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}
