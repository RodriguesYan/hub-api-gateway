# Hub API Gateway

**Single entry point for all Hub Investments microservices**

## Overview

The Hub API Gateway is a Go-based custom gateway that handles authentication, request routing, and cross-cutting concerns for the Hub Investments microservices architecture.

## Features

- ✅ **Authentication Management**: JWT token validation and caching
- ✅ **Request Routing**: Path-based routing to microservices
- ✅ **Performance**: Token validation caching (Redis) for <50ms latency
- ✅ **Security**: Rate limiting, CORS, security headers
- ✅ **Observability**: Structured logging, metrics, health checks
- ✅ **Resilience**: Circuit breaker, connection pooling, retries

## Architecture

```
Client → API Gateway → Microservices
         ├─ Authentication (JWT)
         ├─ Token Caching (Redis)
         ├─ Request Routing
         └─ gRPC Proxy
```

For detailed architecture, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

## Quick Start

### Prerequisites

- Go 1.21+
- Redis 7.0+
- Running microservices (hub-user-service, etc.)

### Installation

```bash
# Clone repository
cd hub-api-gateway

# Install dependencies
go mod download

# Copy configuration
cp config/config.example.yaml config/config.yaml

# Edit configuration
nano config/config.yaml
```

### Configuration

Edit `config/config.yaml`:

```yaml
server:
  port: 8080
  timeout: 30s

redis:
  host: localhost
  port: 6379
  db: 0

services:
  user-service:
    address: localhost:50051
  order-service:
    address: localhost:50052
```

### Run

```bash
# Development
go run cmd/server/main.go

# Production
go build -o gateway cmd/server/main.go
./gateway
```

### Docker

```bash
# Build image
docker build -t hub-api-gateway .

# Run container
docker run -p 8080:8080 \
  -e REDIS_HOST=redis \
  -e USER_SERVICE_ADDRESS=user-service:50051 \
  hub-api-gateway
```

## Usage

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'

# Response:
# {
#   "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
#   "userId": "user123",
#   "expiresIn": 600
# }
```

### Protected Request

```bash
curl http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Response:
# {
#   "orders": [...]
# }
```

### Health Check

```bash
curl http://localhost:8080/health

# Response:
# {
#   "status": "healthy",
#   "services": {
#     "hub-user-service": "healthy",
#     "redis": "healthy"
#   }
# }
```

## API Routes

| Path | Method | Service | Auth Required |
|------|--------|---------|---------------|
| `/api/v1/auth/login` | POST | User Service | No |
| `/api/v1/auth/validate` | POST | User Service | No |
| `/api/v1/orders` | GET/POST | Order Service | Yes |
| `/api/v1/orders/{id}` | GET | Order Service | Yes |
| `/api/v1/positions` | GET | Position Service | Yes |
| `/api/v1/market-data/{symbol}` | GET | Market Data Service | No |

See `config/routes.yaml` for complete route configuration.

## Performance

| Metric | Target | Actual |
|--------|--------|--------|
| Gateway Latency (cache hit) | <50ms | TBD |
| Gateway Latency (cache miss) | <100ms | TBD |
| Throughput | 10,000 req/sec | TBD |
| Concurrent Connections | 10,000+ | TBD |
| Cache Hit Rate | >90% | TBD |

## Development

### Project Structure

```
hub-api-gateway/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── auth/
│   │   ├── login_handler.go    # Login endpoint
│   │   └── user_client.go      # User Service gRPC client
│   ├── middleware/
│   │   ├── auth_middleware.go  # JWT validation
│   │   ├── cors_middleware.go  # CORS handling
│   │   ├── logging_middleware.go
│   │   └── rate_limit_middleware.go
│   ├── router/
│   │   ├── service_router.go   # Route matching
│   │   └── route_config.go     # Route configuration
│   ├── proxy/
│   │   └── grpc_proxy.go       # gRPC proxy
│   └── config/
│       └── config.go            # Configuration loader
├── config/
│   ├── config.yaml              # Main configuration
│   └── routes.yaml              # Route definitions
├── docs/
│   └── ARCHITECTURE.md          # Architecture document
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...
```

### Makefile Commands

```bash
make build      # Build binary
make test       # Run tests
make run        # Run locally
make docker     # Build Docker image
make clean      # Clean build artifacts
```

## Monitoring

### Metrics

Access Prometheus metrics at: `http://localhost:8080/metrics`

Key metrics:
- `gateway_requests_total` - Total requests
- `gateway_request_duration_seconds` - Request latency
- `gateway_auth_cache_hits_total` - Token cache hits
- `gateway_errors_total` - Error count

### Logs

Structured JSON logs:

```json
{
  "timestamp": "2024-01-15T10:35:00Z",
  "level": "info",
  "requestId": "req-123...",
  "method": "GET",
  "path": "/api/v1/orders",
  "userId": "user123",
  "status": 200,
  "duration": 45
}
```

## Troubleshooting

### Gateway won't start

```bash
# Check port availability
lsof -i :8080

# Check Redis connection
redis-cli ping

# Check service connectivity
grpcurl -plaintext localhost:50051 list
```

### Authentication failures

```bash
# Verify JWT secret matches user service
echo $JWT_SECRET

# Check token format
curl -v http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer <token>"

# Check Redis cache
redis-cli GET "token_valid:*"
```

### High latency

```bash
# Check cache hit rate
curl http://localhost:8080/metrics | grep cache_hits

# Check service latency
curl http://localhost:8080/metrics | grep service_duration

# Check connection pool
curl http://localhost:8080/metrics | grep grpc_connections
```

## Security

- JWT tokens expire after 10 minutes
- Token validation cached for 5 minutes
- Rate limiting: 100 req/min (authenticated), 20 req/min (IP)
- HTTPS required in production
- Security headers enabled

## Environment Variables

```bash
HTTP_PORT=8080
REDIS_HOST=localhost
REDIS_PORT=6379
USER_SERVICE_ADDRESS=localhost:50051
JWT_SECRET=<shared-secret>
LOG_LEVEL=info
RATE_LIMIT_ENABLED=true
```

## Contributing

1. Follow Go best practices
2. Add tests for new features
3. Update documentation
4. Run `make test` before committing

## License

Proprietary - Hub Investments Platform

## Support

For issues or questions:
- Check [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Review logs: `docker logs hub-api-gateway`
- Contact platform team

---

**Status**: ✅ Phase 10.1 - Step 4.1 Complete (Architecture & Design)
**Next**: Step 4.2 - Authentication Flow Implementation

