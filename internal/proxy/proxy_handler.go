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

	monolithpb "github.com/RodriguesYan/hub-proto-contracts/monolith"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

	// Invoke gRPC method with proper protobuf messages
	grpcService, grpcMethod := route.GetGRPCTarget()
	// Use the full proto package name for the service
	fullMethod := fmt.Sprintf("/hub_investments.%s/%s", grpcService, grpcMethod)

	// Create proper protobuf request and response messages
	request, response, err := h.createProtoMessages(grpcService, grpcMethod, body, pathVars, userContext)
	if err != nil {
		log.Printf("❌ Failed to create proto messages: %v", err)
		h.sendError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	err = conn.Invoke(ctx, fullMethod, request, response)

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

	// Convert proto response to JSON
	h.sendProtoJSON(w, http.StatusOK, response)
}

// createProtoMessages creates the appropriate protobuf request and response messages
func (h *ProxyHandler) createProtoMessages(service, method string, body []byte, pathVars map[string]string, userContext *middleware.UserContext) (proto.Message, proto.Message, error) {
	// Map service.method to proto message types
	switch fmt.Sprintf("%s.%s", service, method) {
	case "BalanceService.GetBalance":
		req := &monolithpb.GetBalanceRequest{}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.GetBalanceResponse{}, nil

	case "PortfolioService.GetPortfolioSummary":
		req := &monolithpb.GetPortfolioSummaryRequest{}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.GetPortfolioSummaryResponse{}, nil

	case "OrderService.SubmitOrder":
		req := &monolithpb.SubmitOrderRequest{}
		if len(body) > 0 {
			if err := protojson.Unmarshal(body, req); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal SubmitOrder request: %w", err)
			}
		}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.SubmitOrderResponse{}, nil

	case "OrderService.GetOrderDetails":
		req := &monolithpb.GetOrderDetailsRequest{}
		if orderId, ok := pathVars["id"]; ok {
			req.OrderId = orderId
		}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.GetOrderDetailsResponse{}, nil

	case "OrderService.GetOrderStatus":
		req := &monolithpb.GetOrderStatusRequest{}
		if orderId, ok := pathVars["id"]; ok {
			req.OrderId = orderId
		}
		return req, &monolithpb.GetOrderStatusResponse{}, nil

	case "OrderService.CancelOrder":
		req := &monolithpb.CancelOrderRequest{}
		if orderId, ok := pathVars["id"]; ok {
			req.OrderId = orderId
		}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.CancelOrderResponse{}, nil

	case "PositionService.GetPositions":
		req := &monolithpb.GetPositionsRequest{}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.GetPositionsResponse{}, nil

	case "PositionService.GetPositionAggregation":
		req := &monolithpb.GetPositionAggregationRequest{}
		if userContext != nil {
			req.UserId = userContext.UserID
		}
		return req, &monolithpb.GetPositionAggregationResponse{}, nil

	case "MarketDataService.GetMarketData":
		req := &monolithpb.GetMarketDataRequest{}
		if symbol, ok := pathVars["symbol"]; ok {
			req.Symbol = symbol
		}
		return req, &monolithpb.GetMarketDataResponse{}, nil

	case "MarketDataService.GetAssetDetails":
		req := &monolithpb.GetAssetDetailsRequest{}
		if symbol, ok := pathVars["symbol"]; ok {
			req.Symbol = symbol
		}
		return req, &monolithpb.GetAssetDetailsResponse{}, nil

	case "MarketDataService.GetBatchMarketData":
		req := &monolithpb.GetBatchMarketDataRequest{}
		if len(body) > 0 {
			if err := protojson.Unmarshal(body, req); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal GetBatchMarketData request: %w", err)
			}
		}
		return req, &monolithpb.GetBatchMarketDataResponse{}, nil

	default:
		return nil, nil, fmt.Errorf("unsupported service method: %s.%s", service, method)
	}
}

// sendProtoJSON sends a protobuf message as JSON
func (h *ProxyHandler) sendProtoJSON(w http.ResponseWriter, statusCode int, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Convert proto message to JSON
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	jsonBytes, err := marshaler.Marshal(msg)
	if err != nil {
		log.Printf("❌ Failed to marshal proto to JSON: %v", err)
		h.sendError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode response")
		return
	}

	// Unwrap api_response wrapper for cleaner API responses
	unwrappedJSON := h.unwrapAPIResponse(jsonBytes)
	w.Write(unwrappedJSON)
}

// unwrapAPIResponse removes the api_response wrapper from the JSON response
func (h *ProxyHandler) unwrapAPIResponse(jsonBytes []byte) []byte {
	var response map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &response); err != nil {
		// If unmarshal fails, return original
		return jsonBytes
	}

	// Check if api_response exists and is successful
	if apiResp, ok := response["api_response"].(map[string]interface{}); ok {
		// Check if the response was successful
		if success, ok := apiResp["success"].(bool); ok && !success {
			// If not successful, keep the api_response for error details
			return jsonBytes
		}

		// Remove api_response from the response
		delete(response, "api_response")

		// If response only has one other field, unwrap it
		if len(response) == 1 {
			for _, value := range response {
				// Return just the inner object
				if innerBytes, err := json.Marshal(value); err == nil {
					return innerBytes
				}
			}
		}
	}

	// Re-marshal without api_response
	if cleanBytes, err := json.Marshal(response); err == nil {
		return cleanBytes
	}

	// If anything fails, return original
	return jsonBytes
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
