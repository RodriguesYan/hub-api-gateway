package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const version = "1.0.0"

func main() {
	log.Printf("🚀 Hub API Gateway v%s starting...", version)

	// TODO: Step 4.2 - Load configuration
	// TODO: Step 4.2 - Initialize Redis connection
	// TODO: Step 4.2 - Initialize gRPC clients (user service, etc.)
	// TODO: Step 4.3 - Set up middleware chain (auth, cors, logging, rate limit)
	// TODO: Step 4.4 - Set up router and routes
	// TODO: Step 4.5 - Start HTTP server

	log.Println("✅ Gateway initialized successfully")
	log.Println("📡 Listening on http://localhost:8080")
	log.Println("📊 Health check: http://localhost:8080/health")
	log.Println("📈 Metrics: http://localhost:8080/metrics")

	// Temporary: Just keep running until interrupted
	// Will be replaced with actual HTTP server in Step 4.5
	fmt.Println("\n⚠️  This is a placeholder. Implementation coming in Steps 4.2-4.5")
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n👋 Shutting down gracefully...")
	log.Println("✅ Gateway stopped")
}
