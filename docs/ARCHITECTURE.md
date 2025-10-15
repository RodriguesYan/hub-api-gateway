# Hub API Gateway - Architecture Document

## Overview

The **Hub API Gateway** serves as the single entry point for all client requests in the Hub Investments microservices architecture. It handles authentication, request routing, and cross-cutting concerns, allowing microservices to focus on business logic.

---

## Design Decision: Go-Based Custom Gateway

### Why Go-Based Custom Gateway?

After evaluating three options:

1. **Go-based custom gateway** ✅ **SELECTED**
2. Kong Gateway (mature, plugin ecosystem)
3. Traefik (cloud-native, Kubernetes-ready)

**Decision: Go-Based Custom Gateway**

### Rationale

| Criteria | Go Custom | Kong | Traefik | Winner |
|----------|-----------|------|---------|--------|
| **Control** | Full control over logic | Plugin-based | Config-based | Go ✅ |
| **Performance** | Native, lightweight | Lua + Nginx | Go-based | Go ✅ |
| **Learning Curve** | Go expertise available | New tech stack | New tech stack | Go ✅ |
| **Complexity** | Simple, no extra deps | Heavy, many features | Medium | Go ✅ |
| **Debugging** | Easy (same language) | Complex (multi-layer) | Medium | Go ✅ |
| **Deployment** | Single binary | Multi-component | Single binary | Tie |
| **Cost** | Free, minimal resources | Free but resource-heavy | Free | Go ✅ |
| **Extensibility** | Code-based | Plugin ecosystem | Limited | Kong |
| **Time to Market** | Fast (2-3 weeks) | Medium (3-4 weeks) | Medium (3-4 weeks) | Go ✅ |

### Benefits of Go Custom Gateway

✅ **Full Control**: Complete flexibility over authentication, routing, and business logic  
✅ **Performance**: Native Go performance, no overhead from external layers  
✅ **Team Expertise**: Leverage existing Go knowledge across the team  
✅ **Simplicity**: Single codebase, easy to understand and maintain  
✅ **Debugging**: Same language as microservices, unified debugging  
✅ **Lightweight**: Minimal dependencies, fast startup, small footprint  
✅ **Integration**: Native gRPC support, easy integration with services  
✅ **Evolution**: Easy to add features incrementally as needed  

### Trade-offs Accepted

❌ **No Plugin Ecosystem**: Must implement all features ourselves (acceptable for MVP)  
❌ **Manual Implementation**: No out-of-box features like Kong (but keeps it simple)  
❌ **Limited GUI**: No admin interface (can add later if needed)  

### Conclusion

For Phase 10.1 (User Service Migration), a **Go-based custom gateway** is the optimal choice. It provides the right balance of simplicity, performance, and control while minimizing complexity and learning curve.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │
│  │ Web Browser  │  │ Mobile App   │  │ External API │             │
│  └──────────────┘  └──────────────┘  └──────────────┘             │
└─────────────────────────────────────────────────────────────────────┘
                                ↓ HTTPS/REST
┌─────────────────────────────────────────────────────────────────────┐
│                      HUB API GATEWAY                                │
│                     (hub-api-gateway)                               │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                  MIDDLEWARE CHAIN                          │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │   │
│  │  │ Logging  │→ │   CORS   │→ │   Auth   │→ │  Router  │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │              AUTHENTICATION HANDLER                        │   │
│  │  • Login: Forward to User Service                          │   │
│  │  • Token Validation: Cache + gRPC to User Service          │   │
│  │  • User Context Injection                                  │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                  REQUEST ROUTER                            │   │
│  │  • Route Matching (path-based)                             │   │
│  │  • Service Discovery                                       │   │
│  │  • Load Balancing (future)                                 │   │
│  │  • Circuit Breaker Pattern                                 │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                    gRPC PROXY                              │   │
│  │  • HTTP → gRPC Translation                                 │   │
│  │  • Connection Pooling                                      │   │
│  │  • Error Handling & Retry                                  │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                  REDIS CACHE                               │   │
│  │  • Token Validation Cache (5min TTL)                       │   │
│  │  • Rate Limiting State                                     │   │
│  └────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                ↓ gRPC (Internal Network)
┌─────────────────────────────────────────────────────────────────────┐
│                      MICROSERVICES LAYER                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │
│  │ User Service │  │Order Service │  │Market Data   │             │
│  │  (port 50051)│  │ (port 50052) │  │ (port 50054) │             │
│  └──────────────┘  └──────────────┘  └──────────────┘             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │
│  │Position Svc  │  │Watchlist Svc │  │  Balance Svc │             │
│  │ (port 50053) │  │ (port 50055) │  │ (port 50056) │             │
│  └──────────────┘  └──────────────┘  └──────────────┘             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Authentication Flow

### 1. Login Flow (First Request)

```
┌────────┐                 ┌─────────────┐                ┌──────────────┐
│ Client │                 │  API Gateway │                │ User Service │
└────────┘                 └─────────────┘                └──────────────┘
     │                             │                              │
     │  POST /api/v1/auth/login    │                              │
     │  { email, password }        │                              │
     ├────────────────────────────→│                              │
     │                             │                              │
     │                             │  gRPC: Login(email, pwd)     │
     │                             ├─────────────────────────────→│
     │                             │                              │
     │                             │                              │ Validate
     │                             │                              │ credentials
     │                             │                              │
     │                             │  JWT Token + User Info       │
     │                             │←─────────────────────────────┤
     │                             │                              │
     │                             │ [Optional: Cache token]      │
     │                             │                              │
     │  200 OK                     │                              │
     │  { token, userId, expires } │                              │
     │←────────────────────────────┤                              │
     │                             │                              │
```

**Steps:**
1. Client sends credentials to `/api/v1/auth/login`
2. Gateway validates request format
3. Gateway calls `UserService.Login()` via gRPC
4. User Service validates credentials and generates JWT token
5. Gateway optionally caches token metadata in Redis
6. Gateway returns JWT token to client

### 2. Protected Request Flow (Subsequent Requests)

```
┌────────┐                 ┌─────────────┐                ┌──────────────┐
│ Client │                 │  API Gateway │                │ User Service │
└────────┘                 └─────────────┘                └──────────────┘
     │                             │                              │
     │  GET /api/v1/orders         │                              │
     │  Authorization: Bearer XXX  │                              │
     ├────────────────────────────→│                              │
     │                             │                              │
     │                             │ 1. Extract token from header │
     │                             │                              │
     │                             │ 2. Check Redis cache         │
     │                             │    [Cache Hit? Return cached]│
     │                             │                              │
     │                             │ 3. gRPC: ValidateToken(token)│
     │                             ├─────────────────────────────→│
     │                             │                              │
     │                             │  { valid, userId, email }    │
     │                             │←─────────────────────────────┤
     │                             │                              │
     │                             │ 4. Cache validation result   │
     │                             │    (5 min TTL)               │
     │                             │                              │
     │                             │ 5. Add user context to req   │
     │                             │    X-User-Id: user123        │
     │                             │    X-User-Email: user@ex.com │
     │                             │                              │
┌────────────┐                     │                              │
│Order Service│                    │ 6. Route to Order Service    │
└────────────┘                     │                              │
     │←──────────────────────────gRPC────────────────────────────┤
     │                             │                              │
     │  Order data                 │                              │
     ├────────────────────────────→│                              │
     │                             │                              │
     │  200 OK                     │                              │
     │  { orders: [...] }          │                              │
     │←────────────────────────────┤                              │
     │                             │                              │
```

**Steps:**
1. Client sends request with `Authorization: Bearer <token>` header
2. Gateway extracts JWT token
3. Gateway checks Redis cache for validation result
4. If cache miss, Gateway calls `UserService.ValidateToken()` via gRPC
5. User Service validates token and returns user context
6. Gateway caches validation result (5-minute TTL)
7. Gateway adds user context to request headers (`X-User-Id`, `X-User-Email`)
8. Gateway routes request to appropriate microservice
9. Microservice processes request (can trust user context from gateway)
10. Gateway returns response to client

---

## Request Routing Strategy

### Route Configuration

Routes are defined in `config/routes.yaml`:

```yaml
routes:
  # Authentication routes (no auth required)
  - path: /api/v1/auth/login
    service: hub-user-service
    address: localhost:50051
    protocol: grpc
    method: POST
    grpc_method: AuthService.Login
    auth_required: false
  
  - path: /api/v1/auth/validate
    service: hub-user-service
    address: localhost:50051
    protocol: grpc
    method: POST
    grpc_method: AuthService.ValidateToken
    auth_required: false

  # Order routes (auth required)
  - path: /api/v1/orders
    service: hub-order-service
    address: localhost:50052
    protocol: grpc
    method: POST
    grpc_method: OrderService.SubmitOrder
    auth_required: true
  
  - path: /api/v1/orders/{id}
    service: hub-order-service
    address: localhost:50052
    protocol: grpc
    method: GET
    grpc_method: OrderService.GetOrder
    auth_required: true

  # Position routes (auth required)
  - path: /api/v1/positions
    service: hub-position-service
    address: localhost:50053
    protocol: grpc
    method: GET
    grpc_method: PositionService.GetPositions
    auth_required: true

  # Market data routes (public)
  - path: /api/v1/market-data/{symbol}
    service: hub-market-data-service
    address: localhost:50054
    protocol: grpc
    method: GET
    grpc_method: MarketDataService.GetMarketData
    auth_required: false
```

### Route Matching Algorithm

1. **Exact Match**: `/api/v1/orders` matches exactly
2. **Path Parameters**: `/api/v1/orders/{id}` matches `/api/v1/orders/123`
3. **Wildcard**: `/api/v1/market-data/*` matches any path under market-data
4. **Longest Match Wins**: Most specific route takes precedence

---

## Components

### 1. Authentication Middleware (`internal/middleware/auth_middleware.go`)

**Responsibilities:**
- Extract JWT token from `Authorization` header
- Validate token format (`Bearer <token>`)
- Check Redis cache for validation result
- Call User Service for token validation (if cache miss)
- Cache validation result (5-minute TTL)
- Inject user context into request headers
- Return 401 Unauthorized for invalid tokens

**Performance:**
- Cache Hit: <5ms
- Cache Miss: <50ms (includes gRPC call)

### 2. Request Router (`internal/router/service_router.go`)

**Responsibilities:**
- Match incoming request to route configuration
- Resolve service address and gRPC method
- Check if authentication is required
- Forward request to appropriate microservice
- Handle routing errors (404, 503)

**Features:**
- Path parameter extraction
- Wildcard matching
- Service health checking
- Circuit breaker pattern

### 3. gRPC Proxy (`internal/proxy/grpc_proxy.go`)

**Responsibilities:**
- Translate HTTP request → gRPC call
- Manage gRPC connection pool
- Handle gRPC errors and retries
- Translate gRPC response → HTTP response
- Add request tracing

**Connection Pooling:**
- Maintain persistent connections to services
- Reuse connections across requests
- Automatic reconnection on failure
- Health checks every 30 seconds

### 4. CORS Middleware (`internal/middleware/cors_middleware.go`)

**Responsibilities:**
- Handle preflight OPTIONS requests
- Add CORS headers to responses
- Support multiple origins (configurable)
- Handle credentials and headers

### 5. Logging Middleware (`internal/middleware/logging_middleware.go`)

**Responsibilities:**
- Log all incoming requests
- Log request/response duration
- Log errors and status codes
- Add request ID for tracing
- Structured JSON logging

### 6. Rate Limiting Middleware (`internal/middleware/rate_limit_middleware.go`)

**Responsibilities:**
- Rate limit per user (authenticated)
- Rate limit per IP (anonymous)
- Configurable limits (e.g., 100 req/min)
- Return 429 Too Many Requests

---

## Technology Stack

### Core Dependencies

```go
// HTTP Server
github.com/gorilla/mux v1.8.1           // HTTP router

// gRPC
google.golang.org/grpc v1.60.0          // gRPC client
google.golang.org/protobuf v1.32.0      // Protocol Buffers

// Caching
github.com/redis/go-redis/v9 v9.4.0     // Redis client

// Configuration
github.com/spf13/viper v1.18.2          // Config management
gopkg.in/yaml.v3 v3.0.1                 // YAML parsing

// Utilities
github.com/google/uuid v1.5.0           // Request ID generation
go.uber.org/zap v1.26.0                 // Structured logging
```

---

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Gateway Latency (cache hit)** | <50ms | p95 |
| **Gateway Latency (cache miss)** | <100ms | p95 |
| **Token Validation (cached)** | <5ms | p95 |
| **Token Validation (gRPC)** | <50ms | p95 |
| **Throughput** | 10,000 req/sec | sustained |
| **Concurrent Connections** | 10,000+ | simultaneous |
| **Memory Usage** | <500MB | steady state |
| **CPU Usage** | <50% | under load |
| **Cache Hit Rate** | >90% | for token validation |
| **Uptime** | 99.9% | monthly |

---

## Security Features

### 1. Token Validation Caching
- **Purpose**: Reduce load on User Service
- **TTL**: 5 minutes (shorter than token expiration)
- **Key**: `token_valid:sha256(token)`
- **Value**: `{userId, email, validUntil}`

### 2. Rate Limiting
- **Per User**: 100 requests/minute (authenticated)
- **Per IP**: 20 requests/minute (anonymous)
- **Burst**: Allow 10 extra requests temporarily
- **Storage**: Redis (distributed rate limiting)

### 3. Security Headers
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000
Content-Security-Policy: default-src 'self'
```

### 4. Request Size Limits
- **Max Request Body**: 10MB
- **Max Header Size**: 8KB
- **Request Timeout**: 30 seconds

### 5. IP Allowlist/Blocklist (Optional)
- Block known malicious IPs
- Allow specific IPs for admin endpoints

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "AUTH_TOKEN_INVALID",
    "message": "Token has expired",
    "details": "Token expired at 2024-01-15T10:30:00Z",
    "requestId": "req-123e4567-e89b-12d3-a456-426614174000",
    "timestamp": "2024-01-15T10:35:00Z"
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `AUTH_TOKEN_MISSING` | 401 | Authorization header missing |
| `AUTH_TOKEN_INVALID` | 401 | Token is invalid or malformed |
| `AUTH_TOKEN_EXPIRED` | 401 | Token has expired |
| `AUTH_FORBIDDEN` | 403 | User lacks required permissions |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `SERVICE_UNAVAILABLE` | 503 | Downstream service unavailable |
| `ROUTE_NOT_FOUND` | 404 | No route matches request |
| `INTERNAL_ERROR` | 500 | Unexpected error |

---

## Monitoring and Observability

### Metrics (Prometheus)

```
# Request metrics
gateway_requests_total{method, path, status}
gateway_request_duration_seconds{method, path}
gateway_request_size_bytes{method, path}
gateway_response_size_bytes{method, path}

# Authentication metrics
gateway_auth_validations_total{result}
gateway_auth_cache_hits_total
gateway_auth_cache_misses_total

# Service metrics
gateway_service_requests_total{service, status}
gateway_service_request_duration_seconds{service}

# Error metrics
gateway_errors_total{type, code}

# Connection metrics
gateway_grpc_connections_active{service}
gateway_grpc_connection_errors_total{service}
```

### Logging (Structured JSON)

```json
{
  "timestamp": "2024-01-15T10:35:00Z",
  "level": "info",
  "requestId": "req-123e4567-e89b-12d3-a456-426614174000",
  "method": "GET",
  "path": "/api/v1/orders",
  "userId": "user123",
  "status": 200,
  "duration": 45,
  "service": "hub-order-service",
  "cacheHit": true
}
```

### Health Checks

```
GET /health
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600,
  "services": {
    "hub-user-service": "healthy",
    "hub-order-service": "healthy",
    "redis": "healthy"
  }
}
```

---

## Deployment

### Environment Variables

```bash
# Server Configuration
HTTP_PORT=8080
GRPC_TIMEOUT=30s
SHUTDOWN_TIMEOUT=30s

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Service Addresses
USER_SERVICE_ADDRESS=localhost:50051
ORDER_SERVICE_ADDRESS=localhost:50052
POSITION_SERVICE_ADDRESS=localhost:50053
MARKET_DATA_SERVICE_ADDRESS=localhost:50054

# Authentication
JWT_SECRET=<shared-secret-with-user-service>

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_USER=100
RATE_LIMIT_PER_IP=20
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o gateway cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/gateway .
COPY config/ ./config/
EXPOSE 8080
CMD ["./gateway"]
```

---

## Future Enhancements

### Phase 2 (Future)
- [ ] Service mesh integration (Istio)
- [ ] API versioning support
- [ ] GraphQL gateway
- [ ] WebSocket support
- [ ] API documentation (Swagger/OpenAPI)
- [ ] Admin UI for monitoring
- [ ] Advanced rate limiting (token bucket, sliding window)
- [ ] Request/response transformation
- [ ] API key authentication
- [ ] OAuth2 integration

---

## Conclusion

The Hub API Gateway provides a solid, performant foundation for the microservices architecture. By using a Go-based custom implementation, we maintain full control while keeping complexity low and performance high.

**Key Advantages:**
- ✅ Simple architecture, easy to understand
- ✅ High performance with minimal overhead
- ✅ Full control over authentication and routing
- ✅ Easy to debug and maintain
- ✅ Scalable to 10,000+ concurrent connections
- ✅ Production-ready in 2-3 weeks

**Next Steps:**
- Implement authentication flow (Step 4.2)
- Implement token validation middleware (Step 4.3)
- Implement request routing (Step 4.4)
- Deploy and test in development environment

