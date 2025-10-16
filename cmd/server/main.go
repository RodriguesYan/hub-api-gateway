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

	"github.com/gorilla/mux"
)

const version = "1.0.0"

func main() {
	log.Printf("🚀 Hub API Gateway v%s starting...", version)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
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

	// Create router
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Metrics endpoint (placeholder)
	router.HandleFunc("/metrics", metricsHandler).Methods("GET")

	// Authentication endpoints
	loginHandler := auth.NewLoginHandler(userClient)
	router.HandleFunc("/api/v1/auth/login", loginHandler.Handle).Methods("POST", "OPTIONS")

	// Create HTTP server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	server := &http.Server{
		Addr:           addr,
		Handler:        router,
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
