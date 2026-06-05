package config

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Port string

	// JWT
	JWTSecret        string
	JWTRefreshSecret string

	// Upstream services
	BackendURL        string
	PaymentGatewayURL string

	// Rate limiting
	RateLimit     int           // requests per window
	RateWindow    time.Duration // window duration
	RateBurstSize int           // burst allowance

	// File limits
	MaxFileSizeBytes int64
	MaxFileCount     int

	// CORS
	AllowedOrigins []string

	// Trusted internal network (for internal service calls)
	InternalSecret string
}

// Load reads configuration from environment / .env file.
func Load() *Config {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("PORT", "8000")
	viper.SetDefault("BACKEND_URL", "http://localhost:3012")
	viper.SetDefault("PAYMENT_GATEWAY_URL", "http://localhost:8080")
	viper.SetDefault("RATE_LIMIT", 100)
	viper.SetDefault("RATE_WINDOW_SECONDS", 60)
	viper.SetDefault("RATE_BURST_SIZE", 20)
	viper.SetDefault("MAX_FILE_SIZE_MB", 10)
	viper.SetDefault("MAX_FILE_COUNT", 5)
	viper.SetDefault("ALLOWED_ORIGINS", "http://localhost:3022,http://localhost:3030")
	viper.SetDefault("INTERNAL_SECRET", "internal-secret-change-me")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[config] no .env file found, using environment variables only")
	}

	originsRaw := viper.GetString("ALLOWED_ORIGINS")
	origins := strings.Split(originsRaw, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}

	windowSec := viper.GetInt("RATE_WINDOW_SECONDS")

	return &Config{
		Port:              viper.GetString("PORT"),
		JWTSecret:         viper.GetString("JWT_SECRET"),
		JWTRefreshSecret:  viper.GetString("JWT_REFRESH_SECRET"),
		BackendURL:        viper.GetString("BACKEND_URL"),
		PaymentGatewayURL: viper.GetString("PAYMENT_GATEWAY_URL"),
		RateLimit:         viper.GetInt("RATE_LIMIT"),
		RateWindow:        time.Duration(windowSec) * time.Second,
		RateBurstSize:     viper.GetInt("RATE_BURST_SIZE"),
		MaxFileSizeBytes:  int64(viper.GetInt("MAX_FILE_SIZE_MB")) * 1024 * 1024,
		MaxFileCount:      viper.GetInt("MAX_FILE_COUNT"),
		AllowedOrigins:    origins,
		InternalSecret:    viper.GetString("INTERNAL_SECRET"),
	}
}
