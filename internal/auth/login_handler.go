package auth

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the successful login response
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expiresIn"` // seconds
	UserID    string `json:"userId"`
	Email     string `json:"email"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
	Timestamp string `json:"timestamp"`
}

// LoginHandler handles the login endpoint
type LoginHandler struct {
	userClient *UserServiceClient
}

// NewLoginHandler creates a new login handler
func NewLoginHandler(userClient *UserServiceClient) *LoginHandler {
	return &LoginHandler{
		userClient: userClient,
	}
}

// Handle processes the login request
func (h *LoginHandler) Handle(w http.ResponseWriter, r *http.Request) {
	log.Printf("📥 Received login request from %s", r.RemoteAddr)

	// Only accept POST
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed. Use POST.")
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

	// Parse request
	var loginReq LoginRequest
	if err := json.Unmarshal(body, &loginReq); err != nil {
		log.Printf("❌ Failed to parse request body: %v", err)
		h.sendError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format")
		return
	}

	// Validate request
	if err := h.validateLoginRequest(&loginReq); err != nil {
		log.Printf("❌ Request validation failed: %v", err)
		h.sendError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Call User Service
	log.Printf("🔄 Forwarding login request to User Service for email: %s", loginReq.Email)

	ctx := r.Context()
	resp, err := h.userClient.Login(ctx, loginReq.Email, loginReq.Password)
	if err != nil {
		log.Printf("❌ User Service returned error: %v", err)
		// Determine appropriate error code based on error
		if resp != nil && resp.ApiResponse != nil {
			statusCode := int(resp.ApiResponse.Code)
			if statusCode == 0 {
				statusCode = http.StatusUnauthorized
			}
			h.sendError(w, statusCode, "AUTH_FAILED", resp.ApiResponse.Message)
		} else {
			h.sendError(w, http.StatusUnauthorized, "AUTH_FAILED", "Invalid credentials")
		}
		return
	}

	// Extract user info
	var userID, email string
	if resp.UserInfo != nil {
		userID = resp.UserInfo.UserId
		email = resp.UserInfo.Email
	}

	// Build response
	loginResp := LoginResponse{
		Token:     resp.Token,
		ExpiresIn: 600, // 10 minutes (from user service)
		UserID:    userID,
		Email:     email,
	}

	log.Printf("✅ Login successful for email: %s, userId: %s", email, userID)

	// Send response
	h.sendJSON(w, http.StatusOK, loginResp)
}

// validateLoginRequest validates the login request
func (h *LoginHandler) validateLoginRequest(req *LoginRequest) error {
	if req.Email == "" {
		return &ValidationError{Field: "email", Message: "Email is required"}
	}

	if req.Password == "" {
		return &ValidationError{Field: "password", Message: "Password is required"}
	}

	// Basic email format validation
	if len(req.Email) < 3 || !contains(req.Email, "@") {
		return &ValidationError{Field: "email", Message: "Invalid email format"}
	}

	return nil
}

// sendJSON sends a JSON response
func (h *LoginHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ Failed to encode response: %v", err)
	}
}

// sendError sends an error response
func (h *LoginHandler) sendError(w http.ResponseWriter, status int, code, message string) {
	errResp := ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	h.sendJSON(w, status, errResp)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
