package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ReverseProxy creates a Gin handler that proxies requests to the given target URL.
func ReverseProxy(targetURL string) gin.HandlerFunc {
	return ReverseProxyWithStripPrefix(targetURL, "")
}

// ReverseProxyWithStripPrefix creates a Gin handler that proxies requests to the given target URL,
// stripping the specified prefix from the request path before forwarding.
// e.g. strip "/api/hospital/v1" so "/api/hospital/v1/auth/login" → "/auth/login" on the upstream.
func ReverseProxyWithStripPrefix(targetURL, stripPrefix string) gin.HandlerFunc {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic(fmt.Sprintf("invalid proxy target URL %q: %v", targetURL, err))
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Strip gateway prefix so the upstream sees only its own path segment.
			if stripPrefix != "" && strings.HasPrefix(req.URL.Path, stripPrefix) {
				req.URL.Path = req.URL.Path[len(stripPrefix):]
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
				if req.URL.RawPath != "" {
					req.URL.RawPath = req.URL.RawPath[len(stripPrefix):]
				}
			}

			// Remove hop-by-hop headers that should not be forwarded
			req.Header.Del("Te")
			req.Header.Del("Trailers")
		},
		Transport: &http.Transport{
			// MaxIdleConnsPerHost controls keep-alive connection reuse per upstream.
			// Default is 2 which causes queuing under any meaningful load.
			// This gateway proxies to 2 upstreams so 100 per host is appropriate.
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			// Prevent hanging indefinitely if the upstream is slow to respond.
			ResponseHeaderTimeout: 30 * time.Second,
			// Keep-alive is enabled by default; make it explicit.
			DisableKeepAlives: false,
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
