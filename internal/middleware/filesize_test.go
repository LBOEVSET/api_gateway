package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func filesizeRouter(maxFileSizeMB int64, maxFileCount int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		MaxFileSizeBytes: maxFileSizeMB * 1024 * 1024,
		MaxFileCount:     maxFileCount,
	}
	r := gin.New()
	r.Use(FileSizeLimit(cfg))
	r.Any("/upload", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestFileSizeLimit_NonMultipart_Passes(t *testing.T) {
	r := filesizeRouter(10, 5)
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileSizeLimit_MultipartWithinLimit_Passes(t *testing.T) {
	r := filesizeRouter(10, 5)
	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\nsmall content\r\n--boundary--")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileSizeLimit_ContentLengthExceedsLimit_Returns413(t *testing.T) {
	r := filesizeRouter(1, 1) // 1MB * 1 file = 1MB limit
	// Claim 2MB via Content-Length header
	body := strings.NewReader("x")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.ContentLength = 2 * 1024 * 1024 // 2MB > 1MB limit
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestFileSizeLimit_MultipleFiles_TotalLimitApplied(t *testing.T) {
	// 1MB per file, 2 files = 2MB total limit
	r := filesizeRouter(1, 2)

	// Claim 3MB via Content-Length
	body := bytes.NewReader(make([]byte, 1))
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.ContentLength = 3 * 1024 * 1024 // 3MB > 2MB total limit
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestFileSizeLimit_GETRequest_Passes(t *testing.T) {
	r := filesizeRouter(10, 5)
	req := httptest.NewRequest("GET", "/upload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFileSizeLimit_ErrorMessage_ContainsMBLimit(t *testing.T) {
	r := filesizeRouter(5, 2) // 5MB * 2 = 10MB total
	body := strings.NewReader("x")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	req.ContentLength = 11 * 1024 * 1024 // 11MB > 10MB
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), fmt.Sprintf("%d MB", 5*2))
}
