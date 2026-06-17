package router

import (
	"net/http"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/consoleshop/api-gateway/internal/middleware"
	"github.com/consoleshop/api-gateway/internal/proxy"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// route registers both the bare path and the wildcard sub-path for a given
// prefix, so requests like POST /api/v1/statistics (no trailing segment) and
// GET /api/v1/statistics/aggregate both match correctly.
// In Gin, /*path requires at least one character after the slash, so a bare
// path like /v1/statistics would not match /v1/statistics/*path.
func route(g *gin.RouterGroup, prefix string, handler gin.HandlerFunc) {
	g.Any(prefix, handler)
	g.Any(prefix+"/*path", handler)
}

// New builds and returns the Gin engine with all routes and middleware wired up.
func New(cfg *config.Config) *gin.Engine {
	r := gin.New()

	// ─── Global middleware ────────────────────────────────────────────────────
	r.Use(middleware.RequestLogger())
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "Cookie"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
	}))

	r.Use(middleware.RateLimit(cfg))
	r.Use(middleware.ContentTypeValidator())
	r.Use(middleware.FileSizeLimit(cfg))
	r.Use(middleware.Auth(cfg))

	// ─── Health check (gateway itself) ───────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api-gateway"})
	})

	// ─── Proxied routes ───────────────────────────────────────────────────────
	backendProxy := proxy.ReverseProxy(cfg.BackendURL)
	paymentProxy := proxy.ReverseProxy(cfg.PaymentGatewayURL)
	// Strip "/api/hospital/v1" so NestJS sees "/auth", "/bookings", etc.
	hospitalProxy := proxy.ReverseProxyWithStripPrefix(cfg.HospitalBackendURL, "/api/hospital/v1")

	api := r.Group("/api")
	{
		route(api, "/v1/auth", backendProxy)
		route(api, "/v1/products", backendProxy)
		route(api, "/v1/cart", backendProxy)
		route(api, "/v1/orders", backendProxy)
		route(api, "/v1/profile", backendProxy)
		route(api, "/v1/articles", backendProxy)
		route(api, "/v1/events", backendProxy)
		route(api, "/v1/merchandise", backendProxy)
		route(api, "/v1/dashboard", backendProxy)
		route(api, "/v1/statistics", backendProxy)
		route(api, "/v1/support-tickets", backendProxy)
		route(api, "/v1/chat", backendProxy)
		route(api, "/v1/health-check", backendProxy)
		route(api, "/v1/subscription", backendProxy)
		route(api, "/v1/concerts", backendProxy)
		route(api, "/v1/venues", backendProxy)

		// ── Payments → Payment Gateway ──────────────────────────────────────
		route(api, "/v1/payments", paymentProxy)

		// ── Hospital routes → Hospital Backend ─────────────────────────────
		// All /api/hospital/v1/* paths are forwarded to the hospital-backend service.
		// The /hospital prefix is stripped by the reverse proxy so the backend
		// receives the request at its own /auth, /bookings, /doctors, etc. paths.
		route(api, "/hospital/v1/auth", hospitalProxy)
		route(api, "/hospital/v1/profile", hospitalProxy)
		route(api, "/hospital/v1/bookings", hospitalProxy)
		route(api, "/hospital/v1/packages", hospitalProxy)
		route(api, "/hospital/v1/doctor", hospitalProxy)  // singular — doctor self-service (/doctor/me, /doctor/me/schedule)
		route(api, "/hospital/v1/doctors", hospitalProxy)
		route(api, "/hospital/v1/services", hospitalProxy)
		route(api, "/hospital/v1/articles", hospitalProxy)
		route(api, "/hospital/v1/awards", hospitalProxy)
		route(api, "/hospital/v1/hospital-info", hospitalProxy)
		route(api, "/hospital/v1/campaigns", hospitalProxy)
		route(api, "/hospital/v1/rewards", hospitalProxy)
		route(api, "/hospital/v1/doctor-schedule", hospitalProxy)
		route(api, "/hospital/v1/admin", hospitalProxy)
		route(api, "/hospital/v1/health", hospitalProxy)
	}

	return r
}
