# API Gateway Routing Guide

## Overview

The API Gateway uses a flexible, configuration-driven routing system to forward HTTP requests to appropriate microservices. This document explains how routing works, how to configure routes, and how to add new services.

---

## Architecture

```
Client HTTP Request
         ↓
┌────────────────────────────────────────────┐
│          API Gateway                        │
│                                             │
│  1. Parse incoming request                  │
│     (path, method, headers, body)          │
│          ↓                                  │
│  2. Find matching route                     │
│     (check routes.yaml config)             │
│          ↓                                  │
│  3. Check authentication                    │
│     (if route.auth_required = true)        │
│          ↓                                  │
│  4. Extract path variables                  │
│     (e.g., /orders/{id} -> id=123)         │
│          ↓                                  │
│  5. Forward to target service               │
│     (via gRPC)                              │
│          ↓                                  │
│  6. Transform response                      │
│     (gRPC → HTTP JSON)                     │
│          ↓                                  │
└────────────────────────────────────────────┘
         ↓
    HTTP Response to Client
```

---

## Route Configuration

Routes are defined in `config/routes.yaml`. Each route specifies how an HTTP request should be handled.

### Basic Route Structure

```yaml
routes:
  - name: "submit-order"              # Unique route name
    path: "/api/v1/orders"            # URL path pattern
    method: POST                       # HTTP method
    service: order-service             # Target microservice
    grpc_service: "OrderService"       # gRPC service name
    grpc_method: "SubmitOrder"         # gRPC method name
    auth_required: true                # Requires authentication
    description: "Submit a new order"  # Human-readable description
```

### Path Patterns

The gateway supports three types of path patterns:

#### 1. **Exact Match**
```yaml
path: "/api/v1/orders"
```
- Matches: `/api/v1/orders`
- Does NOT match: `/api/v1/orders/123`

#### 2. **Path Variables**
```yaml
path: "/api/v1/orders/{id}"
```
- Matches: `/api/v1/orders/123`, `/api/v1/orders/abc`
- Variables extracted: `{"id": "123"}`
- Multiple variables: `/api/v1/orders/{orderId}/items/{itemId}`

#### 3. **Wildcards**
```yaml
path: "/api/v1/market-data/*"
```
- Matches: `/api/v1/market-data/AAPL`, `/api/v1/market-data/quotes/AAPL`
- Lowest priority (matches after exact and variable paths)

### Route Priority

Routes are matched in order of specificity:

1. **Exact matches** (highest priority)
   - `/api/v1/orders` → exact
   
2. **Path variables**
   - `/api/v1/orders/{id}` → with variables
   
3. **Wildcards** (lowest priority)
   - `/api/v1/orders/*` → catch-all

4. **Longer paths** are more specific
   - `/api/v1/orders/history` > `/api/v1/orders`

---

## Authentication

Routes can require authentication by setting `auth_required: true`:

### Protected Route
```yaml
- name: "get-positions"
  path: "/api/v1/positions"
  method: GET
  service: position-service
  grpc_service: "PositionService"
  grpc_method: "GetPositions"
  auth_required: true  # ← Requires valid JWT token
```

**Request:**
```http
GET /api/v1/positions HTTP/1.1
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Public Route
```yaml
- name: "get-market-data"
  path: "/api/v1/market-data/{symbol}"
  method: GET
  service: market-data-service
  grpc_service: "MarketDataService"
  grpc_method: "GetMarketData"
  auth_required: false  # ← No authentication required
```

---

## Adding a New Route

### Step 1: Add Route Definition

Edit `config/routes.yaml`:

```yaml
routes:
  # ... existing routes ...
  
  - name: "get-user-profile"
    path: "/api/v1/users/{userId}/profile"
    method: GET
    service: user-service
    grpc_service: "UserService"
    grpc_method: "GetProfile"
    auth_required: true
    description: "Get user profile information"
```

### Step 2: Restart Gateway

The gateway loads routes at startup:

```bash
# Stop gateway (Ctrl+C)
# Start gateway
./bin/gateway
```

You should see in logs:
```
✅ Loaded 15 routes from config/routes.yaml
📋 Configured Routes:
=====================================================

🔹 user-service:
  GET /api/v1/users/{userId}/profile -> UserService.GetProfile (🔒 protected)
  
... other routes ...
```

### Step 3: Test Route

```bash
# Login to get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}' \
  | jq -r '.token')

# Call new route
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/users/123/profile
```

---

## Advanced Features

### Rate Limiting (Optional)

```yaml
- name: "submit-order"
  path: "/api/v1/orders"
  method: POST
  service: order-service
  grpc_service: "OrderService"
  grpc_method: "SubmitOrder"
  auth_required: true
  rate_limit:
    requests: 10    # Maximum 10 requests
    per: minute     # Per minute
```

### Timeout Configuration (Optional)

```yaml
- name: "long-running-query"
  path: "/api/v1/reports/generate"
  method: POST
  service: report-service
  grpc_service: "ReportService"
  grpc_method: "GenerateReport"
  auth_required: true
  timeout: "60s"  # 60 second timeout
```

---

## Route Matching Examples

### Example 1: Exact Match

**Route:**
```yaml
path: "/api/v1/orders"
method: POST
```

**Requests:**
| Request | Matches? |
|---------|----------|
| `POST /api/v1/orders` | ✅ Yes |
| `GET /api/v1/orders` | ❌ No (method mismatch) |
| `POST /api/v1/orders/123` | ❌ No (path mismatch) |

### Example 2: Path Variables

**Route:**
```yaml
path: "/api/v1/orders/{id}"
method: GET
```

**Requests:**
| Request | Matches? | Extracted Variables |
|---------|----------|---------------------|
| `GET /api/v1/orders/123` | ✅ Yes | `{"id": "123"}` |
| `GET /api/v1/orders/abc-def` | ✅ Yes | `{"id": "abc-def"}` |
| `GET /api/v1/orders/` | ❌ No | - |
| `GET /api/v1/orders/123/items` | ❌ No | - |

### Example 3: Multiple Variables

**Route:**
```yaml
path: "/api/v1/orders/{orderId}/items/{itemId}"
method: GET
```

**Requests:**
| Request | Matches? | Extracted Variables |
|---------|----------|---------------------|
| `GET /api/v1/orders/123/items/456` | ✅ Yes | `{"orderId": "123", "itemId": "456"}` |
| `GET /api/v1/orders/abc/items/xyz` | ✅ Yes | `{"orderId": "abc", "itemId": "xyz"}` |
| `GET /api/v1/orders/123` | ❌ No | - |

### Example 4: Wildcard

**Route:**
```yaml
path: "/api/v1/market-data/*"
method: GET
```

**Requests:**
| Request | Matches? |
|---------|----------|
| `GET /api/v1/market-data/AAPL` | ✅ Yes |
| `GET /api/v1/market-data/quotes/AAPL` | ✅ Yes |
| `GET /api/v1/market-data/` | ✅ Yes |
| `GET /api/v1/market-data` | ✅ Yes |
| `GET /api/v1/orders/123` | ❌ No |

---

## Service Discovery

The gateway uses static configuration for service addresses. Services are defined in `config/config.example.yaml`:

```yaml
services:
  user-service:
    address: "localhost:50051"
    timeout: 10s
    max_retries: 3
  
  order-service:
    address: "localhost:50052"
    timeout: 10s
    max_retries: 3
```

**Future Enhancement**: Dynamic service discovery with Consul/etcd.

---

## Error Handling

### Route Not Found

**Request:**
```http
GET /api/v1/unknown-endpoint HTTP/1.1
```

**Response:**
```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": "No route found for GET /api/v1/unknown-endpoint",
  "code": "ROUTE_NOT_FOUND"
}
```

### Authentication Required

**Request (missing token):**
```http
GET /api/v1/orders HTTP/1.1
```

**Response:**
```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "error": "Authorization token is required",
  "code": "AUTH_TOKEN_MISSING"
}
```

### Method Not Allowed

**Request:**
```http
DELETE /api/v1/orders HTTP/1.1
Authorization: Bearer <token>
```

**Response:**
```http
HTTP/1.1 405 Method Not Allowed
Content-Type: application/json

{
  "error": "Method DELETE not allowed for /api/v1/orders",
  "code": "METHOD_NOT_ALLOWED"
}
```

---

## Testing Routes

### Manual Testing

```bash
# 1. List all routes (check gateway logs on startup)
./bin/gateway

# Look for output:
# 📋 Configured Routes:
# =====================================================
# 🔹 user-service:
#   POST /api/v1/auth/login -> AuthService.Login (🔓 public)
#   ...

# 2. Test specific route
curl -v http://localhost:8080/api/v1/orders

# 3. Test with authentication
TOKEN="<your-jwt-token>"
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/orders
```

### Automated Testing

See `test_routing.sh` (to be created).

---

## Configuration Files

### `/config/routes.yaml`
- Defines all HTTP → gRPC route mappings
- Loaded once at gateway startup
- Changes require gateway restart

### `/config/config.example.yaml`
- Service addresses and timeouts
- Authentication settings
- Redis configuration
- CORS and rate limiting settings

---

## Best Practices

### 1. **Route Naming**
- Use descriptive names: `get-order`, `submit-order`, `cancel-order`
- Follow convention: `<verb>-<resource>`

### 2. **Path Patterns**
- Use RESTful conventions:
  - `GET /resources` - List resources
  - `GET /resources/{id}` - Get single resource
  - `POST /resources` - Create resource
  - `PUT /resources/{id}` - Update resource
  - `DELETE /resources/{id}` - Delete resource

### 3. **Authentication**
- Protect all sensitive endpoints with `auth_required: true`
- Only public data should have `auth_required: false`

### 4. **Method Specificity**
- Always specify the HTTP method
- Don't use wildcards for methods

### 5. **Documentation**
- Add meaningful descriptions to all routes
- Document expected request/response formats

---

## Troubleshooting

### Issue: "No route found"

**Cause**: Path pattern doesn't match request URL

**Solution**:
1. Check exact spelling of path
2. Verify HTTP method matches
3. Check route priority (exact > variables > wildcards)
4. Review gateway logs for route loading

### Issue: Route matches wrong service

**Cause**: Route priority conflict

**Solution**:
1. Make path more specific
2. Check route order (most specific routes should be defined first conceptually, but the gateway handles this automatically)

### Issue: Path variables not extracted

**Cause**: Incorrect path pattern syntax

**Solution**:
- Use `{varName}` syntax (with curly braces)
- Variable names must be alphanumeric
- Example: `/orders/{id}` ✅, `/orders/:id` ❌

---

## Monitoring

### Route Metrics (Future)

```
# Requests per route
gateway_route_requests_total{route="submit-order", method="POST"} 1234

# Route latency
gateway_route_duration_seconds{route="submit-order", method="POST"} 0.056

# Route errors
gateway_route_errors_total{route="submit-order", method="POST", error="timeout"} 12
```

### Logging

The gateway logs route matches:
```
📍 Route matched: POST /api/v1/orders -> submit-order
```

---

## Future Enhancements

1. **Dynamic Route Loading**: Reload routes without restart
2. **Route Versioning**: Support `/v1/` and `/v2/` with different backends
3. **A/B Testing**: Route % of traffic to different service versions
4. **Circuit Breaker**: Auto-disable routes for failing services
5. **Request Transformation**: Modify requests before forwarding
6. **Response Caching**: Cache GET requests at gateway level

---

## Summary

The API Gateway routing system provides:

✅ **Flexible Matching**: Exact, variables, and wildcards  
✅ **Authentication**: Per-route auth requirements  
✅ **Configuration-Driven**: No code changes for new routes  
✅ **Path Variables**: Extract and forward to services  
✅ **Priority Handling**: Intelligent route matching  
✅ **Error Handling**: Clear error messages  

**Next Step**: Proceed to Step 4.5 (Core Implementation) to add gRPC proxying.

