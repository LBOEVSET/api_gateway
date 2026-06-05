package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// ReverseProxy creates a Gin handler that proxies requests to the given target URL.
// The stripPrefix is removed from the request path before forwarding (empty = no strip).
func ReverseProxy(targetURL string) gin.HandlerFunc {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(fmt.Sprintf("invalid proxy target URL %q: %v", targetURL, err))
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Remove hop-by-hop headers that should not be forwarded
			req.Header.Del("Te")
			req.Header.Del("Trailers")
		},
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"Upstream service unavailable","statusCode":502}`))
		},
		ModifyResponse: func(resp *http.Response) error {
			// Expose request ID in responses
			if rid := resp.Request.Header.Get("X-Request-ID"); rid != "" {
				resp.Header.Set("X-Request-ID", rid)
			}
			return nil
		},
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
