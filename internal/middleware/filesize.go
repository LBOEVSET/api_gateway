package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
)

// FileSizeLimit enforces maximum total request body size for multipart uploads.
//
// Important: we ONLY wrap the body with MaxBytesReader — we do NOT call
// ParseMultipartForm, because that consumes the body stream entirely.
// If the body were parsed here the downstream service (NestJS/multer) would
// receive an empty stream and abort. Individual file-type and per-file size
// validation is handled by the downstream service.
func FileSizeLimit(cfg *config.Config) gin.HandlerFunc {
	maxBytes := cfg.MaxFileSizeBytes * int64(cfg.MaxFileCount)
	return func(c *gin.Context) {
		ct := c.ContentType()
		if !strings.HasPrefix(ct, "multipart/form-data") {
			c.Next()
			return
		}

		// Reject via Content-Length header before reading anything
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"message": fmt.Sprintf(
					"Request too large. Maximum total size is %d MB.",
					maxBytes/(1024*1024),
				),
				"statusCode": 413,
			})
			return
		}

		// Wrap body so the downstream read is capped — body stays unread here
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		c.Next()
	}
}
