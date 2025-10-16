package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"hub-api-gateway/internal/auth"
	"hub-api-gateway/internal/config"

	"github.com/redis/go-redis/v9"
)

// UserContext contains validated user information
type UserContext struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
}

// AuthMiddleware handles JWT token validation
type AuthMiddleware struct {
	userClient  *auth.UserServiceClient
	redisClient *redis.Client
	config      *config.Config
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(userClient *auth.UserServiceClient, redisClient *redis.Client, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		userClient:  userClient,
		redisClient: redisClient,
		config:      cfg,
	}
}

// Middleware returns an HTTP middleware function for token validation
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := m.extractToken(r)
		if err != nil {
			log.Printf("❌ Token extraction failed: %v", err)
			m.sendErrorResponse(w, http.StatusUnauthorized, "AUTH_TOKEN_MISSING", "Authorization token is required")
			return
		}

		userContext, err := m.ValidateToken(r.Context(), token)
		if err != nil {
			log.Printf("❌ Token validation failed: %v", err)
			m.sendErrorResponse(w, http.StatusUnauthorized, "AUTH_TOKEN_INVALID", "Token expired or invalid")
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", userContext)

		// Add user context to request headers for downstream services
		r.Header.Set("X-User-ID", userContext.UserID)
		r.Header.Set("X-User-Email", userContext.Email)

		log.Printf("✅ Token validated for user: %s (%s)", userContext.Email, userContext.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken extracts JWT token from Authorization header
func (m *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header not found")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid authorization header format")
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("authorization scheme must be Bearer")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", fmt.Errorf("token is empty")
	}

	return token, nil
}

// ValidateToken validates a JWT token using cache-first strategy
func (m *AuthMiddleware) ValidateToken(ctx context.Context, token string) (*UserContext, error) {
	tokenHash := hashToken(token)
	cacheKey := fmt.Sprintf("token_valid:%s", tokenHash)

	if m.redisClient != nil {
		cachedUser, err := m.getFromCache(ctx, cacheKey)
		if err == nil && cachedUser != nil {
			log.Printf("🚀 Token validation cache HIT for user: %s", cachedUser.Email)
			return cachedUser, nil
		}
		if err != nil && err != redis.Nil {
			log.Printf("⚠️  Redis error (continuing without cache): %v", err)
		}
	}

	log.Printf("📞 Token validation cache MISS, calling User Service...")

	userContext, err := m.validateTokenWithUserService(ctx, token)
	if err != nil {
		return nil, err
	}

	if m.redisClient != nil {
		if err := m.saveToCache(ctx, cacheKey, userContext, 5*time.Minute); err != nil {
			log.Printf("⚠️  Failed to cache token validation: %v", err)
		} else {
			log.Printf("💾 Cached token validation for user: %s", userContext.Email)
		}
	}

	return userContext, nil
}

// validateTokenWithUserService calls user service gRPC to validate token
func (m *AuthMiddleware) validateTokenWithUserService(ctx context.Context, token string) (*UserContext, error) {
	resp, err := m.userClient.ValidateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token with user service: %w", err)
	}

	if !resp.ApiResponse.Success {
		return nil, fmt.Errorf("token validation failed: %s", resp.ApiResponse.Message)
	}

	if resp.UserInfo == nil {
		return nil, fmt.Errorf("user info not found in response")
	}

	if resp.UserInfo.UserId == "" || resp.UserInfo.Email == "" {
		return nil, fmt.Errorf("invalid user context from service")
	}

	return &UserContext{
		UserID: resp.UserInfo.UserId,
		Email:  resp.UserInfo.Email,
	}, nil
}

// getFromCache retrieves cached user context
func (m *AuthMiddleware) getFromCache(ctx context.Context, key string) (*UserContext, error) {
	val, err := m.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var userContext UserContext
	if err := json.Unmarshal([]byte(val), &userContext); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user context: %w", err)
	}

	return &userContext, nil
}

// saveToCache stores user context in cache
func (m *AuthMiddleware) saveToCache(ctx context.Context, key string, userContext *UserContext, ttl time.Duration) error {
	data, err := json.Marshal(userContext)
	if err != nil {
		return fmt.Errorf("failed to marshal user context: %w", err)
	}

	return m.redisClient.Set(ctx, key, data, ttl).Err()
}

// hashToken creates a SHA256 hash of the token for cache key
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// sendErrorResponse sends a JSON error response
func (m *AuthMiddleware) sendErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error": message,
		"code":  errorCode,
	}

	json.NewEncoder(w).Encode(response)
}

// GetUserContext extracts user context from request context
func GetUserContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value("user").(*UserContext)
	return user, ok
}
