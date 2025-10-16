# Testing Hub API Gateway

## Quick Test

### Prerequisites

1. **User Service must be running** (`hub-user-service`)
   ```bash
   cd hub-user-service
   export MY_JWT_SECRET="your-secret-key"
   export DATABASE_URL="your-database-url"
   go run cmd/server/main.go
   ```

2. **Set environment variables** for Gateway
   ```bash
   export JWT_SECRET="your-secret-key"  # MUST match user service
   export USER_SERVICE_ADDRESS="localhost:50051"
   export HTTP_PORT="8080"
   ```

### Start the Gateway

```bash
cd hub-api-gateway
./bin/gateway
```

Expected output:
```
🚀 Hub API Gateway v1.0.0 starting...
Loading configuration from environment variables...
✅ Configuration loaded successfully:
   Server: localhost:8080 (timeout: 30s)
   Redis: localhost:6379 (cache TTL: 5m0s)
   JWT Secret: your... (length: 20 bytes)
   User Service: localhost:50051
Connecting to User Service at localhost:50051...
✅ Connected to User Service at localhost:50051
✅ Gateway initialized successfully
📡 Listening on http://localhost:8080
📊 Health check: http://localhost:8080/health
🔐 Login: http://localhost:8080/api/v1/auth/login

Gateway is ready to accept requests! 🎉
```

### Test Login Endpoint

#### Test 1: Successful Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

**Expected Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 600,
  "userId": "user123",
  "email": "test@example.com"
}
```

#### Test 2: Invalid Credentials

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "wrongpassword"
  }'
```

**Expected Response (401 Unauthorized):**
```json
{
  "error": {
    "code": "AUTH_FAILED",
    "message": "Invalid credentials",
    "timestamp": "2024-10-15T21:15:00Z"
  }
}
```

#### Test 3: Missing Email

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "password": "password123"
  }'
```

**Expected Response (400 Bad Request):**
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Email is required",
    "timestamp": "2024-10-15T21:15:00Z"
  }
}
```

### Test Health Endpoint

```bash
curl http://localhost:8080/health
```

**Expected Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-10-15T21:15:00Z"
}
```

### Automated Test Script

Run the automated test suite:

```bash
./test_login.sh
```

This will run all test scenarios and provide a summary.

## Manual Testing with User Service

### Step 1: Start User Service

```bash
cd ../hub-user-service
export MY_JWT_SECRET="test-jwt-secret-key-for-integration"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_NAME="hub_user_service"
export DB_USER="your_db_user"
export DB_PASSWORD="your_db_password"
go run cmd/server/main.go
```

### Step 2: Start Gateway

```bash
cd ../hub-api-gateway
export JWT_SECRET="test-jwt-secret-key-for-integration"
export USER_SERVICE_ADDRESS="localhost:50051"
./bin/gateway
```

### Step 3: Test Login Flow

```bash
# Login (assuming you have a user in the database)
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "your_email@example.com",
    "password": "your_password"
  }'
```

## Troubleshooting

### Gateway won't start

**Error:** `Failed to connect to user service`

**Solution:** Ensure user service is running:
```bash
grpcurl -plaintext localhost:50051 list
```

### JWT Secret Mismatch

**Error:** `Token validation failed`

**Solution:** Ensure both services use the SAME JWT_SECRET:
```bash
# Check gateway
echo $JWT_SECRET

# Check user service
echo $MY_JWT_SECRET
```

They must be identical!

### Connection Refused

**Error:** `connection refused`

**Solution:** Check service addresses:
```bash
# User service should be on port 50051
netstat -an | grep 50051

# Gateway should be on port 8080
netstat -an | grep 8080
```

## What's Working (Step 4.2)

✅ **Configuration Loader** - Loads from environment variables  
✅ **User Service Client** - gRPC client with connection management  
✅ **Login Handler** - HTTP endpoint that forwards to User Service  
✅ **HTTP Server** - Gorilla Mux router with endpoints  
✅ **Health Check** - `/health` endpoint  
✅ **Error Handling** - Proper HTTP status codes and error responses  

## What's Next (Step 4.3)

- [ ] Token validation middleware
- [ ] Redis caching for token validation
- [ ] Protected endpoint routing
- [ ] Request routing to multiple services

---

**Status**: ✅ Step 4.2 Complete - Authentication Flow Working!

