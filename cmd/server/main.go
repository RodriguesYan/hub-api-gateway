package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hub-api-gateway/internal/auth"
	"hub-api-gateway/internal/config"
	"hub-api-gateway/internal/middleware"
	"hub-api-gateway/internal/router"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

const version = "1.0.0"

func main() {
	log.Printf("🚀 Hub API Gateway v%s starting...", version)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}

	// Initialize Redis client (optional, for caching)
	var redisClient *redis.Client
	if cfg.Auth.CacheEnabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		// Test Redis connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("⚠️  Warning: Redis connection failed (continuing without cache): %v", err)
			redisClient = nil
		} else {
			log.Println("✅ Connected to Redis for token caching")
		}
	} else {
		log.Println("ℹ️  Redis caching disabled")
	}

	if redisClient != nil {
		defer redisClient.Close()
	}

	// Initialize User Service gRPC client
	userClient, err := auth.NewUserServiceClient(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create User Service client: %v", err)
	}
	defer userClient.Close()

	// Test User Service connectivity
	if err := userClient.Ping(context.Background()); err != nil {
		log.Printf("⚠️  Warning: User Service connectivity check failed: %v", err)
	}

	// Initialize authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(userClient, redisClient, cfg)

	// Load route configuration
	serviceRouter, err := router.NewServiceRouter("config/routes.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load routes: %v", err)
	}

	// List all configured routes
	serviceRouter.ListRoutes()

	// Create HTTP router
	muxRouter := mux.NewRouter()

	// Health check endpoint
	muxRouter.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Metrics endpoint (placeholder)
	muxRouter.HandleFunc("/metrics", metricsHandler).Methods("GET")

	// Authentication endpoints (public)
	loginHandler := auth.NewLoginHandler(userClient)
	muxRouter.HandleFunc("/api/v1/auth/login", loginHandler.Handle).Methods("POST", "OPTIONS")

	// Protected routes (require authentication)
	protectedRouter := muxRouter.PathPrefix("/api/v1").Subrouter()
	protectedRouter.Use(authMiddleware.Middleware)

	// Example protected endpoint
	protectedRouter.HandleFunc("/profile", profileHandler).Methods("GET")
	protectedRouter.HandleFunc("/test", testProtectedHandler).Methods("GET")

	// Create HTTP server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	server := &http.Server{
		Addr:           addr,
		Handler:        muxRouter,
		ReadTimeout:    cfg.Server.Timeout,
		WriteTimeout:   cfg.Server.Timeout,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in a goroutine
	go func() {
		log.Println("✅ Gateway initialized successfully")
		log.Printf("📡 Listening on http://localhost%s", addr)
		log.Printf("📊 Health check: http://localhost%s/health", addr)
		log.Printf("📈 Metrics: http://localhost%s/metrics", addr)
		log.Printf("🔐 Login: http://localhost%s/api/v1/auth/login", addr)
		log.Println("")
		log.Println("Gateway is ready to accept requests! 🎉")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n👋 Shutting down gracefully...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Gateway stopped")
}

// healthCheckHandler handles health check requests
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := `{
		"status": "healthy",
		"version": "` + version + `",
		"timestamp": "` + time.Now().Format(time.RFC3339) + `"
	}`

	w.Write([]byte(response))
}

// metricsHandler handles metrics requests (placeholder)
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	// Placeholder metrics
	metrics := `# HELP gateway_info Gateway information
# TYPE gateway_info gauge
gateway_info{version="` + version + `"} 1

# HELP gateway_requests_total Total number of requests
# TYPE gateway_requests_total counter
gateway_requests_total 0
`

	w.Write([]byte(metrics))
}

// profileHandler handles profile requests (protected endpoint example)
func profileHandler(w http.ResponseWriter, r *http.Request) {
	userContext, ok := middleware.GetUserContext(r.Context())
	if !ok {
		http.Error(w, "User context not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := fmt.Sprintf(`{
		"userId": "%s",
		"email": "%s",
		"message": "This is a protected endpoint"
	}`, userContext.UserID, userContext.Email)

	w.Write([]byte(response))
}

// testProtectedHandler is a simple test endpoint that requires authentication
func testProtectedHandler(w http.ResponseWriter, r *http.Request) {
	userContext, ok := middleware.GetUserContext(r.Context())
	if !ok {
		http.Error(w, "User context not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := fmt.Sprintf(`{
		"status": "success",
		"message": "You are authenticated!",
		"user": {
			"userId": "%s",
			"email": "%s"
		},
		"timestamp": "%s"
	}`, userContext.UserID, userContext.Email, time.Now().Format(time.RFC3339))

	w.Write([]byte(response))
}
