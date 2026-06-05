package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs every request with method, path, status, latency and request ID.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			requestID = hex.EncodeToString(b)
		}

		c.Header("X-Request-ID", requestID)
		c.Request.Header.Set("X-Request-ID", requestID)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(
			c.Request.Context(),
			level,
			fmt.Sprintf("%s %s", method, path),
			"status", status,
			"latency", latency.String(),
			"ip", c.ClientIP(),
			"requestId", requestID,
		)
	}
}
