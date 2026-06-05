package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
)

// FileSizeLimit enforces maximum file size and file count for multipart uploads.
func FileSizeLimit(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.ContentType()
		if !strings.HasPrefix(ct, "multipart/form-data") {
			c.Next()
			return
		}

		// Limit total request size first
		c.Request.Body = http.MaxBytesReader(
			c.Writer,
			c.Request.Body,
			cfg.MaxFileSizeBytes*int64(cfg.MaxFileCount),
		)

		if err := c.Request.ParseMultipartForm(cfg.MaxFileSizeBytes); err != nil {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"message": fmt.Sprintf(
					"Request body too large. Maximum total size is %d MB.",
					(cfg.MaxFileSizeBytes*int64(cfg.MaxFileCount))/(1024*1024),
				),
				"statusCode": 413,
			})
			return
		}

		if c.Request.MultipartForm == nil {
			c.Next()
			return
		}

		// Count and size-check individual files
		totalFiles := 0
		for _, files := range c.Request.MultipartForm.File {
			totalFiles += len(files)
			if totalFiles > cfg.MaxFileCount {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"message":    fmt.Sprintf("Too many files. Maximum is %d.", cfg.MaxFileCount),
					"statusCode": 400,
				})
				return
			}
			for _, fh := range files {
				if fh.Size > cfg.MaxFileSizeBytes {
					c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
						"message": fmt.Sprintf(
							"File '%s' exceeds the %d MB limit.",
							fh.Filename,
							cfg.MaxFileSizeBytes/(1024*1024),
						),
						"statusCode": 413,
					})
					return
				}
			}
		}

		c.Next()
	}
}
