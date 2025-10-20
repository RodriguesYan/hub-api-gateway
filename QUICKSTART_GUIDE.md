# API Gateway Quick Start Guide

## 🚀 How to Run All Services

### Prerequisites
- Go 1.21+
- PostgreSQL running
- Redis running (optional, for caching)
- Docker (optional, for containerized setup)

---

## Option 1: Run Everything Locally (Development)

### Step 1: Start Database & Redis
```bash
# Start PostgreSQL (if using Docker)
docker run -d \
  --name postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=hub_investments \
  -p 5432:5432 \
  postgres:15

# Start Redis (optional, for caching)
docker run -d \
  --name redis \
  -p 6379:6379 \
  redis:7-alpine
```

### Step 2: Start User Service (Port 50051)
```bash
cd /Users/yanrodrigues/Documents/HubInvestmentsProject/hub-user-service

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=hub_user_service
export DB_USER=postgres
export DB_PASSWORD=postgres
export MY_JWT_SECRET=your-secret-key-here
export GRPC_PORT=localhost:50051

# Run migrations (first time only)
make migrate-up

# Start the service
go run cmd/server/main.go
```

**Expected output:**
```
✅ User Service started on :50051
```

### Step 3: Start Monolith (Port 50060 for gRPC, 8081 for HTTP)
```bash
cd /Users/yanrodrigues/Documents/HubInvestmentsProject/HubInvestmentsServer

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=hub_investments
export DB_USER=postgres
export DB_PASSWORD=postgres
export MY_JWT_SECRET=your-secret-key-here
export GRPC_PORT=localhost:50060
export HTTP_PORT=8081

# Start the monolith
go run main.go
```

**Expected output:**
```
✅ HTTP Server started on :8081
✅ gRPC Server started on :50060
```

### Step 4: Start API Gateway (Port 8080)
```bash
cd /Users/yanrodrigues/Documents/HubInvestmentsProject/hub-api-gateway

# Set environment variables
export SERVER_PORT=8080
export USER_SERVICE_ADDRESS=localhost:50051
export HUB_MONOLITH_ADDRESS=localhost:50060
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=your-secret-key-here

# Start the gateway
go run cmd/server/main.go
```

**Expected output:**
```
✅ API Gateway started on :8080
✅ Connected to User Service (localhost:50051)
✅ Connected to Hub Monolith (localhost:50060)
✅ Redis cache enabled
```

---

## Option 2: Run with Docker Compose (Recommended)

### Create docker-compose.yml (in project root)
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: hub_investments
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  user-service:
    build:
      context: ./hub-user-service
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_NAME: hub_user_service
      DB_USER: postgres
      DB_PASSWORD: postgres
      MY_JWT_SECRET: your-secret-key-here
      GRPC_PORT: 0.0.0.0:50051
    ports:
      - "50051:50051"
    depends_on:
      - postgres

  monolith:
    build:
      context: ./HubInvestmentsServer
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_NAME: hub_investments
      DB_USER: postgres
      DB_PASSWORD: postgres
      MY_JWT_SECRET: your-secret-key-here
      GRPC_PORT: 0.0.0.0:50060
      HTTP_PORT: 8081
    ports:
      - "8081:8081"
      - "50060:50060"
    depends_on:
      - postgres

  api-gateway:
    build:
      context: ./hub-api-gateway
    environment:
      SERVER_PORT: 8080
      USER_SERVICE_ADDRESS: user-service:50051
      HUB_MONOLITH_ADDRESS: monolith:50060
      REDIS_HOST: redis
      REDIS_PORT: 6379
      JWT_SECRET: your-secret-key-here
    ports:
      - "8080:8080"
    depends_on:
      - user-service
      - monolith
      - redis

volumes:
  postgres_data:
```

### Start everything
```bash
docker-compose up -d
```

---

## 📡 How to Make Requests to API Gateway

### Architecture Overview
```
Client (You)
     ↓ HTTP REST
API Gateway (:8080)
     ↓ gRPC
┌────────────────┬────────────────┐
User Service     Monolith
(:50051)         (:50060)
```

**All requests go through API Gateway on port 8080**

---

## 🔐 Authentication Flow

### 1. Login (Get JWT Token)
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 600,
  "userId": "user-uuid-here",
  "email": "test@example.com"
}
```

**Save the token** - you'll need it for all protected endpoints!

---

## 📊 Making Requests to Protected Endpoints

### 2. Get Portfolio Summary
```bash
TOKEN="your-jwt-token-here"

curl -X GET http://localhost:8080/api/v1/portfolio/summary \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "totalPortfolioValue": 125000.50,
  "cashBalance": 25000.00,
  "positions": [
    {
      "symbol": "AAPL",
      "quantity": 100,
      "averagePrice": 150.00,
      "currentPrice": 175.50,
      "marketValue": 17550.00,
      "unrealizedPnL": 2550.00
    }
  ],
  "lastUpdated": "2025-10-20T10:30:00Z"
}
```

### 3. Get Balance
```bash
curl -X GET http://localhost:8080/api/v1/balance \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "userId": "user-uuid",
  "availableBalance": 25000.00,
  "totalBalance": 25000.00,
  "currency": "USD",
  "lastUpdated": "2025-10-20T10:30:00Z"
}
```

### 4. Submit Order
```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "AAPL",
    "quantity": 10,
    "side": "BUY",
    "type": "MARKET"
  }'
```

**Response:**
```json
{
  "orderId": "order-uuid",
  "status": "PENDING",
  "message": "Order submitted successfully"
}
```

### 5. Get Order Status
```bash
ORDER_ID="order-uuid-here"

curl -X GET http://localhost:8080/api/v1/orders/$ORDER_ID/status \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "orderId": "order-uuid",
  "status": "EXECUTED",
  "symbol": "AAPL",
  "quantity": 10,
  "executionPrice": 175.50,
  "executedAt": "2025-10-20T10:31:00Z"
}
```

### 6. Get Positions
```bash
curl -X GET http://localhost:8080/api/v1/positions \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "positions": [
    {
      "id": "position-uuid",
      "symbol": "AAPL",
      "quantity": 110,
      "averagePrice": 152.00,
      "currentPrice": 175.50,
      "marketValue": 19305.00,
      "unrealizedPnL": 2585.00,
      "unrealizedPnLPct": 16.99
    }
  ]
}
```

### 7. Get Market Data (Public - No Auth Required)
```bash
curl -X GET http://localhost:8080/api/v1/market-data/AAPL
```

**Response:**
```json
{
  "symbol": "AAPL",
  "price": 175.50,
  "change": 2.50,
  "changePercent": 1.44,
  "volume": 50000000,
  "timestamp": "2025-10-20T10:30:00Z"
}
```

---

## 🧪 Testing Script

Create a file `test_api_gateway.sh`:

```bash
#!/bin/bash

API_GATEWAY="http://localhost:8080"

echo "🔐 Step 1: Login and get token..."
LOGIN_RESPONSE=$(curl -s -X POST $API_GATEWAY/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ Login failed!"
  echo "Response: $LOGIN_RESPONSE"
  exit 1
fi

echo "✅ Login successful! Token: ${TOKEN:0:20}..."

echo ""
echo "📊 Step 2: Get Portfolio Summary..."
curl -s -X GET $API_GATEWAY/api/v1/portfolio/summary \
  -H "Authorization: Bearer $TOKEN" | jq '.'

echo ""
echo "💰 Step 3: Get Balance..."
curl -s -X GET $API_GATEWAY/api/v1/balance \
  -H "Authorization: Bearer $TOKEN" | jq '.'

echo ""
echo "📈 Step 4: Get Positions..."
curl -s -X GET $API_GATEWAY/api/v1/positions \
  -H "Authorization: Bearer $TOKEN" | jq '.'

echo ""
echo "📰 Step 5: Get Market Data (Public)..."
curl -s -X GET $API_GATEWAY/api/v1/market-data/AAPL | jq '.'

echo ""
echo "✅ All tests completed!"
```

Make it executable and run:
```bash
chmod +x test_api_gateway.sh
./test_api_gateway.sh
```

---

## 🔍 Troubleshooting

### Gateway won't start
```bash
# Check if ports are available
lsof -i :8080  # API Gateway
lsof -i :50051 # User Service
lsof -i :50060 # Monolith gRPC

# Kill processes if needed
kill -9 <PID>
```

### Can't connect to services
```bash
# Verify services are running
curl http://localhost:8080/health  # Gateway health
grpcurl -plaintext localhost:50051 list  # User service
grpcurl -plaintext localhost:50060 list  # Monolith
```

### Authentication errors (401)
- Check JWT secret matches across all services
- Verify token hasn't expired (10 minutes)
- Check Authorization header format: `Bearer <token>`

### Database connection errors
- Verify PostgreSQL is running: `psql -h localhost -U postgres -d hub_investments`
- Check environment variables are set correctly
- Run migrations: `make migrate-up`

---

## 📚 Additional Resources

- **API Gateway Documentation**: `docs/ARCHITECTURE.md`
- **User Service Documentation**: `../hub-user-service/README.md`
- **Monolith Documentation**: `../HubInvestmentsServer/README.md`
- **Proto Contracts**: `internal/proto/`

---

## 🎯 Quick Reference

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| API Gateway | 8080 | HTTP | Single entry point for all clients |
| User Service | 50051 | gRPC | Authentication & user management |
| Monolith | 50060 | gRPC | Orders, positions, portfolio, market data |
| Monolith | 8081 | HTTP | Legacy HTTP endpoints (optional) |
| PostgreSQL | 5432 | TCP | Database |
| Redis | 6379 | TCP | Caching (optional) |

---

## 🔄 Development Workflow

1. **Start services** (Gateway, User Service, Monolith)
2. **Login** to get JWT token
3. **Make requests** using the token
4. **Check logs** if something fails
5. **Iterate** and develop

**Pro Tip**: Use Postman or Insomnia to save your requests and tokens!

