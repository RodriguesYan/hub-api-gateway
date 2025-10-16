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
	"hub-api-gateway/internal/metrics"
	"hub-api-gateway/internal/middleware"
	"hub-api-gateway/internal/proxy"
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

	// Initialize metrics collector
	metricsCollector := metrics.NewMetrics()
	log.Println("✅ Metrics collector initialized")

	// Initialize authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(userClient, redisClient, cfg, metricsCollector)

	// Load route configuration
	serviceRouter, err := router.NewServiceRouter("config/routes.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load routes: %v", err)
	}

	// List all configured routes
	serviceRouter.ListRoutes()

	// Initialize service registry for gRPC connections
	serviceRegistry := proxy.NewServiceRegistry(cfg)
	defer serviceRegistry.Close()

	// Initialize proxy handler
	proxyHandler := proxy.NewProxyHandler(serviceRegistry, metricsCollector)

	// Create HTTP router
	muxRouter := mux.NewRouter()

	// Health check endpoint
	muxRouter.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Metrics endpoints
	metricsHandler := metrics.NewHandler(metricsCollector)
	muxRouter.HandleFunc("/metrics", metricsHandler.HandlePrometheus).Methods("GET")
	muxRouter.HandleFunc("/metrics/json", metricsHandler.HandleJSON).Methods("GET")
	muxRouter.HandleFunc("/metrics/summary", metricsHandler.HandleSummary).Methods("GET")

	// Login endpoint (special case - handled directly)
	loginHandler := auth.NewLoginHandler(userClient)
	muxRouter.HandleFunc("/api/v1/auth/login", loginHandler.Handle).Methods("POST", "OPTIONS")

	// Dynamic route handler for all other routes
	muxRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Find matching route
		route, err := serviceRouter.FindRoute(r.URL.Path, r.Method)
		if err != nil {
			log.Printf("⚠️  No route found for %s %s", r.Method, r.URL.Path)
			http.Error(w, `{"error": "Route not found", "code": "ROUTE_NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		// Check authentication requirement
		if route.RequiresAuth() {
			// Apply auth middleware
			authMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Forward to proxy handler
				proxyHandler.HandleRequest(w, r, route)
			})).ServeHTTP(w, r)
		} else {
			// Public route - forward directly
			proxyHandler.HandleRequest(w, r, route)
		}
	})

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
