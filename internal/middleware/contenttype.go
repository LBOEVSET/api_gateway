package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// allowedContentTypes lists accepted Content-Type values for request bodies.
var allowedContentTypes = []string{
	"application/json",
	"multipart/form-data",
	"application/x-www-form-urlencoded",
	"text/plain",
	"application/octet-stream",
}

// ContentTypeValidator rejects requests with invalid Content-Type headers.
// Applies only to methods that carry a body.
func ContentTypeValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet ||
			method == http.MethodDelete ||
			method == http.MethodHead ||
			method == http.MethodOptions {
			c.Next()
			return
		}

		// Allow empty body requests (Content-Length: 0)
		if c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		ct := c.ContentType()
		if ct == "" {
			c.Next()
			return
		}

		// Strip parameters (e.g., charset, boundary)
		base := strings.Split(ct, ";")[0]
		base = strings.TrimSpace(strings.ToLower(base))

		for _, allowed := range allowedContentTypes {
			if base == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
			"message":    "Unsupported Content-Type",
			"statusCode": 415,
			"allowed":    allowedContentTypes,
		})
	}
}
