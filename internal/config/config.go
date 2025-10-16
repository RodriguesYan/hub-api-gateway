package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds all gateway configuration
type Config struct {
	Server    ServerConfig
	Redis     RedisConfig
	Services  map[string]ServiceConfig
	Auth      AuthConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
	Logging   LoggingConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            string
	Timeout         time.Duration
	ShutdownTimeout time.Duration
	MaxBodySize     int64
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host          string
	Port          string
	Password      string
	DB            int
	TokenCacheTTL time.Duration
}

// ServiceConfig holds microservice configuration
type ServiceConfig struct {
	Address    string
	Timeout    time.Duration
	MaxRetries int
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret    string
	CacheEnabled bool
	CacheTTL     time.Duration
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled      bool
	PerUserLimit int
	PerUserBurst int
	PerIPLimit   int
	PerIPBurst   int
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string
	Format     string
	MaskTokens bool
}

var globalConfig *Config

// Load loads configuration from environment variables
func Load() (*Config, error) {
	log.Println("Loading configuration from environment variables...")

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("HTTP_PORT", "8080"),
			Timeout:         getDurationEnv("SERVER_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getDurationEnv("SHUTDOWN_TIMEOUT", 30*time.Second),
			MaxBodySize:     getInt64Env("MAX_BODY_SIZE", 10485760), // 10MB
		},
		Redis: RedisConfig{
			Host:          getEnv("REDIS_HOST", "localhost"),
			Port:          getEnv("REDIS_PORT", "6379"),
			Password:      getEnv("REDIS_PASSWORD", ""),
			DB:            getIntEnv("REDIS_DB", 0),
			TokenCacheTTL: getDurationEnv("TOKEN_CACHE_TTL", 5*time.Minute),
		},
		Services: map[string]ServiceConfig{
			"user-service": {
				Address:    getEnv("USER_SERVICE_ADDRESS", "localhost:50051"),
				Timeout:    getDurationEnv("USER_SERVICE_TIMEOUT", 5*time.Second),
				MaxRetries: getIntEnv("USER_SERVICE_MAX_RETRIES", 3),
			},
			"order-service": {
				Address:    getEnv("ORDER_SERVICE_ADDRESS", "localhost:50052"),
				Timeout:    getDurationEnv("ORDER_SERVICE_TIMEOUT", 10*time.Second),
				MaxRetries: getIntEnv("ORDER_SERVICE_MAX_RETRIES", 3),
			},
			"position-service": {
				Address:    getEnv("POSITION_SERVICE_ADDRESS", "localhost:50053"),
				Timeout:    getDurationEnv("POSITION_SERVICE_TIMEOUT", 5*time.Second),
				MaxRetries: getIntEnv("POSITION_SERVICE_MAX_RETRIES", 3),
			},
			"market-data-service": {
				Address:    getEnv("MARKET_DATA_SERVICE_ADDRESS", "localhost:50054"),
				Timeout:    getDurationEnv("MARKET_DATA_SERVICE_TIMEOUT", 3*time.Second),
				MaxRetries: getIntEnv("MARKET_DATA_SERVICE_MAX_RETRIES", 3),
			},
		},
		Auth: AuthConfig{
			JWTSecret:    getEnv("JWT_SECRET", ""),
			CacheEnabled: getBoolEnv("AUTH_CACHE_ENABLED", true),
			CacheTTL:     getDurationEnv("AUTH_CACHE_TTL", 5*time.Minute),
		},
		CORS: CORSConfig{
			Enabled:          getBoolEnv("CORS_ENABLED", true),
			AllowedOrigins:   []string{getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
			AllowCredentials: getBoolEnv("CORS_ALLOW_CREDENTIALS", true),
		},
		RateLimit: RateLimitConfig{
			Enabled:      getBoolEnv("RATE_LIMIT_ENABLED", true),
			PerUserLimit: getIntEnv("RATE_LIMIT_PER_USER", 100),
			PerUserBurst: getIntEnv("RATE_LIMIT_PER_USER_BURST", 10),
			PerIPLimit:   getIntEnv("RATE_LIMIT_PER_IP", 20),
			PerIPBurst:   getIntEnv("RATE_LIMIT_PER_IP_BURST", 5),
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			MaskTokens: getBoolEnv("LOG_MASK_TOKENS", true),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	globalConfig = cfg
	cfg.LogConfiguration()

	return cfg, nil
}

// Get returns the global configuration
func Get() *Config {
	if globalConfig == nil {
		log.Fatal("Configuration not loaded. Call Load() first.")
	}
	return globalConfig
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}

	if len(c.Auth.JWTSecret) < 32 {
		log.Println("⚠️  WARNING: JWT secret is shorter than 32 characters. Use a stronger secret in production!")
	}

	if c.Server.Port == "" {
		return fmt.Errorf("HTTP_PORT is required")
	}

	if c.Redis.Host == "" {
		return fmt.Errorf("REDIS_HOST is required")
	}

	userServiceAddr := c.Services["user-service"].Address
	if userServiceAddr == "" {
		return fmt.Errorf("USER_SERVICE_ADDRESS is required")
	}

	return nil
}

// LogConfiguration logs the loaded configuration (with sensitive data masked)
func (c *Config) LogConfiguration() {
	log.Println("✅ Configuration loaded successfully:")
	log.Printf("   Server: localhost:%s (timeout: %v)", c.Server.Port, c.Server.Timeout)
	log.Printf("   Redis: %s:%s (cache TTL: %v)", c.Redis.Host, c.Redis.Port, c.Redis.TokenCacheTTL)
	log.Printf("   JWT Secret: %s (length: %d bytes)", maskSecret(c.Auth.JWTSecret), len(c.Auth.JWTSecret))
	log.Printf("   User Service: %s", c.Services["user-service"].Address)
	log.Printf("   CORS: enabled=%v, origins=%v", c.CORS.Enabled, c.CORS.AllowedOrigins)
	log.Printf("   Rate Limit: enabled=%v, per_user=%d/min, per_ip=%d/min",
		c.RateLimit.Enabled, c.RateLimit.PerUserLimit, c.RateLimit.PerIPLimit)
	log.Printf("   Logging: level=%s, format=%s", c.Logging.Level, c.Logging.Format)
}

// GetRedisAddress returns the full Redis address
func (c *Config) GetRedisAddress() string {
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}
