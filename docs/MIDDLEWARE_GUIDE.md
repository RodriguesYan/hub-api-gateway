# API Gateway Middleware Guide

## Overview

The API Gateway uses a middleware-based architecture to handle cross-cutting concerns like authentication, logging, rate limiting, and CORS. This document focuses on the **Authentication Middleware** implementation.

---

## Authentication Middleware

### Purpose

The authentication middleware validates JWT tokens for protected endpoints, ensuring only authenticated users can access secured resources.

### Features

1. **JWT Token Extraction**: Extracts Bearer tokens from `Authorization` headers
2. **Token Validation**: Validates tokens via gRPC call to `hub-user-service`
3. **Redis Caching**: Caches validation results to reduce gRPC calls and improve performance
4. **User Context Injection**: Adds validated user information to request context and headers
5. **Graceful Degradation**: Works without Redis if caching is disabled or unavailable

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Client Request                             │
│                 Authorization: Bearer <token>                    │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                   API Gateway                                    │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │         1. Extract Token from Header                    │   │
│  └───────────────────────────┬─────────────────────────────┘   │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │         2. Check Redis Cache (Optional)                 │   │
│  │                                                           │   │
│  │   Key: "token_valid:<SHA256(token)>"                    │   │
│  │   Value: {"userId": "...", "email": "..."}              │   │
│  │   TTL: 5 minutes                                         │   │
│  └───────────────┬─────────────────────────────────────────┘   │
│                  │                                               │
│          Cache HIT? │                                           │
│                  │                                               │
│         ┌────────┴────────┐                                     │
│         │                 │                                     │
│    Yes  │                 │  No                                 │
│         │                 │                                     │
│         ▼                 ▼                                     │
│  ┌──────────┐   ┌──────────────────────────┐                  │
│  │  Return  │   │  3. Call User Service    │                  │
│  │  Cached  │   │     ValidateToken()      │                  │
│  │   User   │   │     (gRPC)               │                  │
│  └──────────┘   └────────┬─────────────────┘                  │
│                           │                                     │
│                           ▼                                     │
│                  ┌──────────────────────────┐                  │
│                  │  4. Cache Result         │                  │
│                  │     in Redis             │                  │
│                  └────────┬─────────────────┘                  │
│                           │                                     │
│                           ▼                                     │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  5. Add User Context to Request                         │  │
│  │                                                           │  │
│  │   - r.Context(): "user" = UserContext                   │  │
│  │   - Headers: "X-User-ID", "X-User-Email"                │  │
│  └───────────────────────────┬─────────────────────────────┘  │
│                               │                                 │
└───────────────────────────────┼─────────────────────────────────┘
                                │
                                ▼
                    ┌─────────────────────────┐
                    │  Protected Handler       │
                    │  (Next Middleware/Route) │
                    └─────────────────────────┘
```

---

## Implementation Details

### File Structure

```
hub-api-gateway/
├── internal/
│   └── middleware/
│       └── auth_middleware.go  # Authentication middleware implementation
├── cmd/
│   └── server/
│       └── main.go              # Middleware integration
└── docs/
    └── MIDDLEWARE_GUIDE.md      # This file
```

### Core Components

#### 1. **AuthMiddleware Struct**

```go
type AuthMiddleware struct {
    userClient  *auth.UserServiceClient  // gRPC client for user service
    redisClient *redis.Client            // Redis client for caching (optional)
    config      *config.Config           // Application configuration
}
```

#### 2. **UserContext Struct**

```go
type UserContext struct {
    UserID string `json:"userId"`
    Email  string `json:"email"`
}
```

This struct holds validated user information that is injected into the request context.

#### 3. **Middleware Method**

```go
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler
```

This is the main HTTP middleware function that wraps protected routes.

---

## Usage

### Basic Setup

```go
// Initialize Redis client (optional)
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
    DB:   0,
})

// Initialize User Service gRPC client
userClient, err := auth.NewUserServiceClient(cfg)
if err != nil {
    log.Fatal(err)
}

// Create authentication middleware
authMiddleware := middleware.NewAuthMiddleware(userClient, redisClient, cfg)

// Create router
router := mux.NewRouter()

// Public routes (no authentication required)
router.HandleFunc("/api/v1/auth/login", loginHandler).Methods("POST")

// Protected routes (authentication required)
protectedRouter := router.PathPrefix("/api/v1").Subrouter()
protectedRouter.Use(authMiddleware.Middleware)

protectedRouter.HandleFunc("/profile", profileHandler).Methods("GET")
protectedRouter.HandleFunc("/orders", ordersHandler).Methods("GET", "POST")
```

### Accessing User Context in Handlers

```go
func profileHandler(w http.ResponseWriter, r *http.Request) {
    // Extract user context from request
    userContext, ok := middleware.GetUserContext(r.Context())
    if !ok {
        http.Error(w, "User context not found", http.StatusInternalServerError)
        return
    }

    // Access user information
    userID := userContext.UserID
    email := userContext.Email

    // Your handler logic here...
}
```

### Accessing User Headers in Downstream Services

The middleware automatically adds these headers for downstream services:

- `X-User-ID`: The authenticated user's ID
- `X-User-Email`: The authenticated user's email

```go
// Downstream service can read these headers
userID := r.Header.Get("X-User-ID")
email := r.Header.Get("X-User-Email")
```

---

## Token Validation Flow

### 1. **Token Extraction**

The middleware extracts the JWT token from the `Authorization` header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Validation Rules:**
- Header must be present
- Format must be: `Bearer <token>`
- Case-insensitive for "Bearer"
- Token must not be empty

### 2. **Cache Check (Optional)**

If Redis is available and caching is enabled:

```go
// Generate cache key
tokenHash := SHA256(token)
cacheKey := "token_valid:" + tokenHash

// Check cache
if cachedUser := redis.Get(cacheKey); cachedUser != nil {
    return cachedUser // Cache HIT 🚀
}
```

**Cache Strategy:**
- **Key**: `token_valid:<SHA256(token)>`
- **Value**: JSON-encoded `UserContext` (`{"userId": "...", "email": "..."}`)
- **TTL**: 5 minutes (shorter than typical JWT expiration)

### 3. **gRPC Validation (Cache Miss)**

If cache miss or Redis unavailable:

```go
// Call user service via gRPC
resp, err := userClient.ValidateToken(ctx, token)

// Check response
if !resp.ApiResponse.Success {
    return error
}

// Extract user info
userContext := &UserContext{
    UserID: resp.UserInfo.UserId,
    Email:  resp.UserInfo.Email,
}
```

### 4. **Cache Update**

Store validation result in Redis for future requests:

```go
redis.Set(cacheKey, json.Marshal(userContext), 5*time.Minute)
```

### 5. **Context Injection**

Add user context to request:

```go
// Add to request context
ctx := context.WithValue(r.Context(), "user", userContext)

// Add to headers for downstream services
r.Header.Set("X-User-ID", userContext.UserID)
r.Header.Set("X-User-Email", userContext.Email)

// Pass to next handler
next.ServeHTTP(w, r.WithContext(ctx))
```

---

## Performance Considerations

### Latency Targets

| Scenario | Target Latency | Notes |
|----------|----------------|-------|
| **Cache HIT** | < 50ms | Redis lookup + middleware overhead |
| **Cache MISS** | < 100ms | gRPC call + validation + cache update |
| **No Cache** | < 150ms | gRPC call + validation (no Redis) |

### Cache Benefits

**Without Cache:**
- Every protected request → 1 gRPC call to user service
- 10,000 requests/sec → 10,000 gRPC calls/sec to user service

**With Cache (5-minute TTL):**
- First request → gRPC call (cache MISS)
- Next 299 requests (assuming 1 req/sec) → Redis lookup (cache HIT)
- **Reduction**: 99.7% fewer gRPC calls to user service

### Token Hash Security

Why we hash tokens for cache keys:

1. **Security**: Avoid storing raw tokens in Redis
2. **Consistency**: SHA256 produces fixed-length keys
3. **Performance**: Fast hashing (< 1ms)

```go
func hashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}
```

---

## Configuration

### Environment Variables

```bash
# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Authentication Configuration
AUTH_CACHE_ENABLED=true          # Enable/disable token caching
AUTH_CACHE_TTL=5m                # Cache TTL (e.g., "5m", "300s")
JWT_SECRET=your-secret-key       # Must match user service secret

# User Service Configuration
USER_SERVICE_ADDRESS=localhost:50051
USER_SERVICE_TIMEOUT=10s
```

### Disabling Cache

To run without Redis caching:

```bash
export AUTH_CACHE_ENABLED=false
```

The middleware will gracefully degrade to always call the user service for validation.

---

## Error Handling

### Error Responses

The middleware returns standardized JSON error responses:

#### Missing Token (401 Unauthorized)
```json
{
  "error": "Authorization token is required",
  "code": "AUTH_TOKEN_MISSING"
}
```

#### Invalid/Expired Token (401 Unauthorized)
```json
{
  "error": "Token expired or invalid",
  "code": "AUTH_TOKEN_INVALID"
}
```

### Error Scenarios

| Scenario | Middleware Behavior | Response |
|----------|---------------------|----------|
| No Authorization header | Return 401 | `AUTH_TOKEN_MISSING` |
| Malformed header | Return 401 | `AUTH_TOKEN_MISSING` |
| Invalid token format | Return 401 | `AUTH_TOKEN_INVALID` |
| Expired token | Return 401 | `AUTH_TOKEN_INVALID` |
| User service unavailable | Return 401 | `AUTH_TOKEN_INVALID` |
| Redis unavailable | Continue without cache | Normal flow |

---

## Testing

### Test Script

Run the provided test script:

```bash
cd hub-api-gateway
./test_protected_endpoint.sh
```

This script tests:
1. ✅ Login to get JWT token
2. ✅ Access protected endpoint with valid token
3. ✅ Reject access without token (401)
4. ✅ Reject access with invalid token (401)
5. ✅ Token validation caching (cache HIT/MISS)

### Manual Testing

#### 1. Login to get token
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "userId": "user123",
  "email": "test@example.com"
}
```

#### 2. Access protected endpoint with token
```bash
TOKEN="<your-token-here>"

curl -X GET http://localhost:8080/api/v1/test \
  -H "Authorization: Bearer ${TOKEN}"
```

Response:
```json
{
  "status": "success",
  "message": "You are authenticated!",
  "user": {
    "userId": "user123",
    "email": "test@example.com"
  }
}
```

#### 3. Access without token (should fail)
```bash
curl -X GET http://localhost:8080/api/v1/test
```

Response (401):
```json
{
  "error": "Authorization token is required",
  "code": "AUTH_TOKEN_MISSING"
}
```

---

## Monitoring & Observability

### Log Messages

The middleware logs key events for monitoring:

```
✅ Token validated for user: test@example.com (user123)
🚀 Token validation cache HIT for user: test@example.com
📞 Token validation cache MISS, calling User Service...
💾 Cached token validation for user: test@example.com
❌ Token extraction failed: authorization header not found
❌ Token validation failed: token expired
⚠️  Redis error (continuing without cache): connection refused
```

### Metrics to Monitor

1. **Token Validation Latency**
   - p50, p95, p99 latency
   - Separate cache HIT vs MISS

2. **Cache Hit Rate**
   - Target: > 90% for good performance
   - Low rate → increase cache TTL or investigate

3. **Authentication Errors**
   - 401 error rate
   - Distinguish between missing token vs invalid token

4. **User Service Availability**
   - gRPC call success rate
   - User service response time

5. **Redis Availability**
   - Redis connection errors
   - Cache operation failures

---

## Security Considerations

### Best Practices

1. **JWT Secret**: Ensure gateway and user service use the same secret
2. **HTTPS**: Always use HTTPS in production to protect tokens in transit
3. **Token Hashing**: Tokens are hashed (SHA256) before caching
4. **Short Cache TTL**: 5-minute TTL limits exposure if token is compromised
5. **No Raw Tokens in Logs**: Never log full JWT tokens

### Security Headers

The gateway should add these security headers (future enhancement):

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
```

---

## Troubleshooting

### Common Issues

#### Issue: "Token validation failed: connection refused"

**Cause**: User service is not running or not reachable

**Solution**:
```bash
# Check if user service is running
netstat -an | grep 50051

# Start user service
cd hub-user-service
go run cmd/server/main.go
```

#### Issue: "Redis connection failed (continuing without cache)"

**Cause**: Redis is not running

**Solution**:
```bash
# Start Redis
docker run -d -p 6379:6379 redis:alpine

# Or disable caching
export AUTH_CACHE_ENABLED=false
```

#### Issue: "401 Unauthorized" even with valid token

**Possible Causes**:
1. JWT secret mismatch between gateway and user service
2. Token expired (check expiration time)
3. User service is down or unreachable

**Debug**:
```bash
# Check JWT secret
echo $JWT_SECRET  # Gateway
# Compare with user service JWT_SECRET

# Decode token to check expiration
# https://jwt.io
```

#### Issue: Cache never hits

**Possible Causes**:
1. Cache TTL too short
2. Each request has different token
3. Redis not configured correctly

**Debug**:
```bash
# Check Redis
redis-cli
> KEYS token_valid:*
> TTL token_valid:<hash>
```

---

## Future Enhancements

1. **Token Refresh**: Implement token refresh flow
2. **Revocation List**: Check revoked tokens before validation
3. **Rate Limiting**: Limit authentication attempts per IP
4. **Multi-Level Caching**: Add in-memory cache layer for ultra-low latency
5. **Prometheus Metrics**: Export detailed metrics
6. **Distributed Tracing**: Add OpenTelemetry tracing
7. **Audit Logging**: Log all authentication events for compliance

---

## References

- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [API Gateway Pattern](https://microservices.io/patterns/apigateway.html)
- [Redis Caching Strategies](https://redis.io/docs/manual/patterns/)
- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)

---

## Summary

The authentication middleware provides:

✅ **Secure**: JWT validation via user service  
✅ **Fast**: Redis caching for < 50ms latency  
✅ **Resilient**: Graceful degradation without cache  
✅ **Observable**: Comprehensive logging  
✅ **Easy to Use**: Simple integration with any handler  

**Next Steps**: Proceed to Step 4.4 (Request Routing) to add dynamic routing to microservices.

