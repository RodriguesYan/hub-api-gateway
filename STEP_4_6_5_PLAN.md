# Step 4.6.5: API Gateway - Monolith Integration Testing

## 📋 Overview

Before proceeding to security features (Step 4.7), we need to **verify that the API Gateway can successfully communicate with the existing HubInvestments monolith**. This is a critical integration step to ensure the gateway works in a real-world scenario.

---

## 🎯 Objectives

1. ✅ Create HTTP proxy handler (monolith uses HTTP REST, not gRPC)
2. ✅ Configure gateway to forward requests to monolith
3. ✅ Test all critical endpoints through gateway
4. ✅ Verify authentication flow works end-to-end
5. ✅ Measure performance overhead
6. ✅ Document any issues and solutions

---

## 🏗️ Current Architecture

### **Monolith (HubInvestmentsServer)**
```
Port: localhost:8080 (HTTP REST)
Endpoints:
  - POST   /login                          (public)
  - GET    /getBalance                     (protected)
  - GET    /getPortfolioSummary           (protected)
  - GET    /getMarketData                 (protected)
  - GET    /getWatchlist                  (protected)
  - POST   /orders                        (protected)
  - GET    /orders/{id}                   (protected)
  - GET    /orders/{id}/status            (protected)
  - PUT    /orders/{id}/cancel            (protected)
  - GET    /orders/history                (protected)
  - POST   /admin/market-data/cache/invalidate (protected)
  - POST   /admin/market-data/cache/warm       (protected)
  - WS     /ws/quotes                     (websocket)
```

### **API Gateway**
```
Port: localhost:9090 (to avoid conflict with monolith)
Function: Reverse proxy that forwards HTTP → HTTP
```

---

## 🔧 Implementation Tasks

### **Task 1: Create HTTP Proxy Handler**

The current proxy handler only supports gRPC. We need to add HTTP support:

```go
// internal/proxy/http_proxy_handler.go

type HTTPProxyHandler struct {
    registry *ServiceRegistry
    metrics  *metrics.Metrics
    client   *http.Client
}

func (h *HTTPProxyHandler) ProxyHTTPRequest(w http.ResponseWriter, r *http.Request, route *router.Route) {
    // 1. Get target service address
    // 2. Build target URL
    // 3. Copy request headers (including Authorization)
    // 4. Forward request to monolith
    // 5. Copy response back to client
    // 6. Record metrics
}
```

### **Task 2: Update Service Registry**

Add HTTP service configuration:

```yaml
# config.example.yaml
services:
  user-service:
    address: "localhost:50051"
    protocol: "grpc"
    timeout: 10s
  
  hub-monolith:
    address: "http://localhost:8080"
    protocol: "http"
    timeout: 30s
```

### **Task 3: Update Routes Configuration**

Map gateway routes to monolith endpoints:

```yaml
# config/routes.yaml
routes:
  # Login (direct to monolith)
  - name: "login"
    path: "/api/v1/auth/login"
    method: POST
    service: hub-monolith
    monolith_path: "/login"
    protocol: http
    auth_required: false
  
  # Portfolio Summary
  - name: "portfolio-summary"
    path: "/api/v1/portfolio/summary"
    method: GET
    service: hub-monolith
    monolith_path: "/getPortfolioSummary"
    protocol: http
    auth_required: true
  
  # Balance
  - name: "get-balance"
    path: "/api/v1/balance"
    method: GET
    service: hub-monolith
    monolith_path: "/getBalance"
    protocol: http
    auth_required: true
  
  # Orders
  - name: "submit-order"
    path: "/api/v1/orders"
    method: POST
    service: hub-monolith
    monolith_path: "/orders"
    protocol: http
    auth_required: true
  
  - name: "get-order-history"
    path: "/api/v1/orders/history"
    method: GET
    service: hub-monolith
    monolith_path: "/orders/history"
    protocol: http
    auth_required: true
  
  # Market Data
  - name: "get-market-data"
    path: "/api/v1/market-data"
    method: GET
    service: hub-monolith
    monolith_path: "/getMarketData"
    protocol: http
    auth_required: true
```

### **Task 4: Update Main Router**

```go
// cmd/server/main.go

// Dynamic route handler
muxRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    route, err := serviceRouter.FindRoute(r.URL.Path, r.Method)
    if err != nil {
        http.Error(w, `{"error": "Route not found"}`, http.StatusNotFound)
        return
    }

    // Check protocol
    if route.Protocol == "http" {
        // Use HTTP proxy handler
        if route.RequiresAuth() {
            authMiddleware.Middleware(http.HandlerFunc(func(w, r) {
                httpProxyHandler.ProxyHTTPRequest(w, r, route)
            })).ServeHTTP(w, r)
        } else {
            httpProxyHandler.ProxyHTTPRequest(w, r, route)
        }
    } else {
        // Use gRPC proxy handler (existing)
        if route.RequiresAuth() {
            authMiddleware.Middleware(http.HandlerFunc(func(w, r) {
                grpcProxyHandler.HandleRequest(w, r, route)
            })).ServeHTTP(w, r)
        } else {
            grpcProxyHandler.HandleRequest(w, r, route)
        }
    }
})
```

---

## 🧪 Testing Plan

### **Pre-requisites**

1. **Start Monolith**:
   ```bash
   cd /Users/yanrodrigues/Documents/HubInvestmentsProject/HubInvestmentsServer
   ./HubInvestments
   # Should be running on localhost:8080
   ```

2. **Start Gateway** (on different port):
   ```bash
   cd /Users/yanrodrigues/Documents/HubInvestmentsProject/hub-api-gateway
   export HTTP_PORT=9090  # Use port 9090 to avoid conflict
   ./bin/gateway
   # Should be running on localhost:9090
   ```

3. **Verify Database**:
   - PostgreSQL running
   - Redis running
   - Test user exists in database

### **Test Scenarios**

#### **Scenario 1: Login Flow (Public Endpoint)**

```bash
# Test login through gateway
curl -v -X POST http://localhost:9090/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Expected Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "userId": "user123",
  "email": "test@example.com"
}

# Verify:
# ✅ Status 200
# ✅ Token returned
# ✅ Gateway logs show: "Proxying request: POST /api/v1/auth/login -> http://localhost:8080/login"
# ✅ Monolith logs show request received
```

#### **Scenario 2: Protected Endpoint (Portfolio)**

```bash
# Save token from login
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Test portfolio summary
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/portfolio/summary

# Expected Response:
{
  "balance": {
    "available": 10000.00,
    "total": 15000.00
  },
  "positions": [
    {
      "symbol": "AAPL",
      "quantity": 100,
      "averagePrice": 150.00,
      "currentPrice": 155.00,
      "totalValue": 15500.00,
      "unrealizedPnL": 500.00
    }
  ],
  "totalPortfolioValue": 25500.00
}

# Verify:
# ✅ Status 200
# ✅ Portfolio data returned
# ✅ Gateway validated token (cache hit or miss logged)
# ✅ Gateway forwarded Authorization header
# ✅ Monolith authenticated request
```

#### **Scenario 3: Order Submission**

```bash
# Submit order through gateway
curl -v -X POST http://localhost:9090/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "AAPL",
    "quantity": 10,
    "side": "BUY",
    "type": "MARKET"
  }'

# Expected Response:
{
  "orderId": "order-123",
  "status": "PENDING",
  "message": "Order submitted successfully"
}

# Verify:
# ✅ Status 202 (or 200)
# ✅ Order ID returned
# ✅ Order created in database
# ✅ Gateway metrics show request recorded
```

#### **Scenario 4: Error Handling (Invalid Token)**

```bash
# Test with invalid token
curl -v -H "Authorization: Bearer invalid-token" \
  http://localhost:9090/api/v1/portfolio/summary

# Expected Response:
{
  "error": "Invalid or expired token",
  "code": "AUTH_TOKEN_INVALID"
}

# Verify:
# ✅ Status 401
# ✅ Error message clear
# ✅ Gateway blocked request before forwarding to monolith
# ✅ Metrics show failed request
```

#### **Scenario 5: Service Unavailable (Monolith Down)**

```bash
# Stop monolith
# Then try request
curl -v -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/portfolio/summary

# Expected Response:
{
  "error": "Service hub-monolith is unavailable",
  "code": "SERVICE_UNAVAILABLE"
}

# Verify:
# ✅ Status 503
# ✅ Circuit breaker opens after 5 failures
# ✅ Subsequent requests fail fast (circuit breaker OPEN)
# ✅ Metrics show circuit breaker trip
```

---

## 📊 Performance Benchmarks

### **Latency Targets**

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Gateway Overhead** | <10ms | Gateway processing time |
| **Total Latency (Login)** | <150ms | Client → Gateway → Monolith → Client |
| **Total Latency (Portfolio)** | <200ms | Including token validation + DB query |
| **Cache Hit Latency** | <50ms | Token validation from Redis |

### **Load Testing**

```bash
# Use Apache Bench to test
ab -n 1000 -c 100 \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/portfolio/summary

# Expected:
# - 0% failed requests
# - Average latency <200ms
# - Gateway overhead <10ms
# - No memory leaks
```

---

## 🔍 Monitoring & Debugging

### **Gateway Logs to Check**

```
✅ Route matched: POST /api/v1/auth/login -> login
✅ Proxying HTTP request to http://localhost:8080/login
✅ Request completed in 45ms: POST /api/v1/auth/login
```

### **Metrics to Verify**

```bash
# Check metrics
curl http://localhost:9090/metrics/summary

# Should show:
# - Total Requests: > 0
# - Successful Requests: matching test count
# - Failed Requests: 0 (or expected failures)
# - Avg Latency: <200ms
# - Cache Hit Rate: >80% (after first requests)
```

---

## 🚨 Common Issues & Solutions

### **Issue 1: Port Conflict**

**Problem**: Gateway won't start - port 8080 already in use

**Solution**: 
```bash
# Use different port for gateway
export HTTP_PORT=9090
./bin/gateway
```

### **Issue 2: Connection Refused**

**Problem**: Gateway can't reach monolith

**Solution**:
```bash
# Verify monolith is running
curl http://localhost:8080/login

# Check gateway configuration
cat config.example.yaml | grep hub-monolith
```

### **Issue 3: Token Not Forwarded**

**Problem**: Monolith returns 401 even with valid token

**Solution**:
- Verify gateway forwards Authorization header
- Check HTTP proxy handler copies all headers
- Verify token format is correct

### **Issue 4: CORS Errors (if testing from browser)**

**Problem**: Browser blocks requests

**Solution**:
- Add CORS headers in gateway (Step 4.7)
- For now, test with curl/Postman

---

## ✅ Success Criteria

### **Must Have**

- [x] Gateway forwards requests to monolith successfully
- [x] Authentication flow works (login → get token → use token)
- [x] Protected endpoints accessible with valid token
- [x] Invalid token correctly rejected
- [x] Error responses formatted correctly
- [x] Gateway overhead <10ms
- [x] No functional regressions

### **Nice to Have**

- [ ] WebSocket support (/ws/quotes)
- [ ] Admin endpoints working
- [ ] Load testing passed
- [ ] Documentation complete

---

## 📝 Deliverables

1. **HTTP Proxy Handler** (`internal/proxy/http_proxy_handler.go`)
2. **Updated Route Model** (support for `protocol` and `monolith_path` fields)
3. **Monolith Service Configuration** (in `config.example.yaml`)
4. **Route Mappings** (updated `config/routes.yaml`)
5. **Integration Test Scripts** (`test/integration/monolith_integration_test.sh`)
6. **Test Results Documentation** (this document + results)
7. **Troubleshooting Guide** (common issues section)

---

## 🎯 Next Steps

After Step 4.6.5 is complete:

1. **Step 4.7**: Add security features (CORS, rate limiting, headers)
2. **Step 4.8**: Comprehensive testing suite
3. **Step 4.9**: Final documentation
4. **Production Deployment**: Deploy gateway to staging environment

---

## 📌 Important Notes

1. **Port Configuration**: Gateway MUST use different port than monolith (9090 recommended)
2. **JWT Secret**: Gateway and monolith MUST use same `MY_JWT_SECRET`
3. **Database**: Both must access same PostgreSQL database
4. **Redis**: Both can share same Redis instance
5. **Testing Order**: Always start monolith first, then gateway
6. **Backwards Compatibility**: Gateway should NOT break existing monolith functionality

---

## 🚀 Ready to Implement!

This step is crucial for validating the gateway works in a real-world scenario. Once complete, we'll have confidence that the gateway can handle production traffic!

