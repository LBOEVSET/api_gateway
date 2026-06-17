package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func contenttypeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ContentTypeValidator())
	r.Any("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestContentTypeValidator_GETAlwaysPasses(t *testing.T) {
	r := contenttypeRouter()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_DELETEAlwaysPasses(t *testing.T) {
	r := contenttypeRouter()
	req := httptest.NewRequest("DELETE", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_HEADAlwaysPasses(t *testing.T) {
	r := contenttypeRouter()
	req := httptest.NewRequest("HEAD", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_OPTIONSAlwaysPasses(t *testing.T) {
	r := contenttypeRouter()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_EmptyBody_Passes(t *testing.T) {
	r := contenttypeRouter()
	req := httptest.NewRequest("POST", "/test", nil)
	req.ContentLength = 0
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_NoContentType_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/test", body)
	// No Content-Type header set, Content-Length > 0
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_ValidJSON_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_JSONWithCharset_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_MultipartFormData_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader("--boundary\r\n\r\n")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_FormURLEncoded_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader("key=value")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_TextPlain_Passes(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader("hello")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "text/plain")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentTypeValidator_InvalidContentType_Returns415(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader("<xml>bad</xml>")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "application/xml")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestContentTypeValidator_PUTWithInvalidType_Returns415(t *testing.T) {
	r := contenttypeRouter()
	body := strings.NewReader("data")
	req := httptest.NewRequest("PUT", "/test", body)
	req.Header.Set("Content-Type", "text/html")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}
