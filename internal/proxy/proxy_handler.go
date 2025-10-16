package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"hub-api-gateway/internal/metrics"
	"hub-api-gateway/internal/middleware"
	"hub-api-gateway/internal/router"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ProxyHandler handles HTTP requests and proxies them to gRPC services
type ProxyHandler struct {
	registry *ServiceRegistry
	metrics  *metrics.Metrics
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(registry *ServiceRegistry, m *metrics.Metrics) *ProxyHandler {
	return &ProxyHandler{
		registry: registry,
		metrics:  m,
	}
}

// HandleRequest proxies an HTTP request to the appropriate gRPC service
func (h *ProxyHandler) HandleRequest(w http.ResponseWriter, r *http.Request, route *router.Route) {
	startTime := time.Now()

	log.Printf("📨 Proxying request: %s %s -> %s.%s",
		r.Method, r.URL.Path, route.GRPCService, route.GRPCMethod)

	// Extract path variables
	pathVars := route.ExtractPathVariables(r.URL.Path)

	// Get user context from middleware (if authenticated)
	userContext, _ := middleware.GetUserContext(r.Context())

	// Get circuit breaker for the service
	serviceName := route.GetTargetService()
	circuitBreaker := h.registry.GetCircuitBreaker(serviceName)

	// Get gRPC connection with circuit breaker protection
	var conn *grpc.ClientConn
	if err := circuitBreaker.Call(func() error {
		var err error
		conn, err = h.registry.GetConnection(serviceName)
		if err != nil {
			log.Printf("❌ Failed to get connection to %s: %v", serviceName, err)
			return err
		}
		return nil
	}); err != nil {
		if err == ErrCircuitOpen {
			log.Printf("⚠️  Circuit breaker OPEN for %s", serviceName)
			h.metrics.RecordCircuitBreakerTrip()
			h.metrics.RecordRequest(route.Name, serviceName, time.Since(startTime), false)
			h.sendError(w, http.StatusServiceUnavailable, "CIRCUIT_BREAKER_OPEN",
				fmt.Sprintf("Service %s is temporarily unavailable (circuit breaker open)", serviceName))
			return
		}
		h.metrics.RecordRequest(route.Name, serviceName, time.Since(startTime), false)
		h.sendError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
			fmt.Sprintf("Service %s is unavailable", serviceName))
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Failed to read request body: %v", err)
		h.sendError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Create gRPC context with metadata
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add metadata to gRPC context
	md := metadata.New(map[string]string{
		"x-forwarded-method": r.Method,
		"x-forwarded-path":   r.URL.Path,
		"x-original-uri":     r.RequestURI,
	})

	// Add user context if authenticated
	if userContext != nil {
		md.Set("x-user-id", userContext.UserID)
		md.Set("x-user-email", userContext.Email)
	}

	// Add path variables to metadata
	for key, value := range pathVars {
		md.Set(fmt.Sprintf("x-path-%s", key), value)
	}

	ctx = metadata.NewOutgoingContext(ctx, md)

	// Invoke gRPC method
	grpcService, grpcMethod := route.GetGRPCTarget()
	fullMethod := fmt.Sprintf("/%s/%s", grpcService, grpcMethod)

	var response interface{}
	err = conn.Invoke(ctx, fullMethod, h.buildRequest(body, pathVars), &response)

	if err != nil {
		log.Printf("❌ gRPC call failed for %s: %v", fullMethod, err)
		h.handleGRPCError(w, err)
		return
	}

	// Send success response
	elapsed := time.Since(startTime)
	log.Printf("✅ Request completed in %v: %s %s", elapsed, r.Method, r.URL.Path)

	// Record successful request metrics
	h.metrics.RecordRequest(route.Name, serviceName, elapsed, true)

	h.sendJSON(w, http.StatusOK, response)
}

// buildRequest builds a gRPC request from HTTP body and path variables
func (h *ProxyHandler) buildRequest(body []byte, pathVars map[string]string) interface{} {
	// Try to parse as JSON
	var request map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &request); err != nil {
			// If not JSON, treat as raw body
			request = map[string]interface{}{
				"body": string(body),
			}
		}
	} else {
		request = make(map[string]interface{})
	}

	// Add path variables to request
	for key, value := range pathVars {
		request[key] = value
	}

	return request
}

// handleGRPCError converts gRPC errors to HTTP errors
func (h *ProxyHandler) handleGRPCError(w http.ResponseWriter, err error) {
	// Map gRPC errors to HTTP status codes
	statusCode := http.StatusInternalServerError
	errorCode := "INTERNAL_ERROR"
	message := err.Error()

	// Check for specific gRPC error codes
	// This is a simplified version - in production, use grpc/status package
	switch {
	case contains(message, "NotFound"):
		statusCode = http.StatusNotFound
		errorCode = "NOT_FOUND"
	case contains(message, "AlreadyExists"):
		statusCode = http.StatusConflict
		errorCode = "ALREADY_EXISTS"
	case contains(message, "PermissionDenied"):
		statusCode = http.StatusForbidden
		errorCode = "PERMISSION_DENIED"
	case contains(message, "Unauthenticated"):
		statusCode = http.StatusUnauthorized
		errorCode = "UNAUTHENTICATED"
	case contains(message, "InvalidArgument"):
		statusCode = http.StatusBadRequest
		errorCode = "INVALID_ARGUMENT"
	case contains(message, "Unavailable"):
		statusCode = http.StatusServiceUnavailable
		errorCode = "SERVICE_UNAVAILABLE"
	case contains(message, "DeadlineExceeded"):
		statusCode = http.StatusGatewayTimeout
		errorCode = "TIMEOUT"
	}

	h.sendError(w, statusCode, errorCode, message)
}

// sendJSON sends a JSON response
func (h *ProxyHandler) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ Failed to encode JSON response: %v", err)
	}
}

// sendError sends an error response
func (h *ProxyHandler) sendError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	response := map[string]interface{}{
		"error": message,
		"code":  errorCode,
	}

	h.sendJSON(w, statusCode, response)
}

// contains is a simple string contains helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
