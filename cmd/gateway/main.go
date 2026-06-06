package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/consoleshop/api-gateway/internal/config"
	"github.com/consoleshop/api-gateway/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Switch Gin to release mode. In debug mode Gin prints every registered
	// route and has extra overhead per request. GIN_MODE env var overrides this
	// so local developers can set GIN_MODE=debug to get verbose output.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg := config.Load()

	if cfg.JWTSecret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}

	r := router.New(cfg)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("API Gateway starting",
			"port", cfg.Port,
			"zone", cfg.Zone,
			"backend", cfg.BackendURL,
			"payment", cfg.PaymentGatewayURL,
			"internalSecretLen", len(cfg.InternalSecret),
			"internalSecretPrefix", func() string {
				if len(cfg.InternalSecret) > 8 {
					return cfg.InternalSecret[:8] + "..."
				}
				return cfg.InternalSecret
			}(),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "err", err)
	}

	slog.Info("API Gateway stopped")
}
