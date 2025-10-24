# Hub API Gateway - Docker Deployment Guide

Complete guide for containerizing and deploying the Hub API Gateway using Docker and Docker Compose.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Building the Docker Image](#building-the-docker-image)
4. [Running with Docker](#running-with-docker)
5. [Running with Docker Compose](#running-with-docker-compose)
6. [Configuration](#configuration)
7. [Health Checks](#health-checks)
8. [Monitoring](#monitoring)
9. [Troubleshooting](#troubleshooting)
10. [Production Deployment](#production-deployment)

---

## Prerequisites

### Required Software

- **Docker**: 20.10+ ([Install Docker](https://docs.docker.com/get-docker/))
- **Docker Compose**: 2.0+ ([Install Docker Compose](https://docs.docker.com/compose/install/))
- **Make**: For using Makefile commands (optional)

### Verify Installation

```bash
docker --version
# Docker version 24.0.0 or higher

docker-compose --version
# Docker Compose version v2.0.0 or higher

make --version
# GNU Make 4.3 or higher (optional)
```

---

## Quick Start

### 1. Clone and Navigate

```bash
cd hub-api-gateway
```

### 2. Create Environment File

```bash
# Copy example environment file
cp env.example .env

# Edit with your values
nano .env
```

**Important**: Set the `JWT_SECRET` to match your user service and monolith:

```bash
JWT_SECRET=HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^
```

### 3. Start Services

```bash
# Using Make
make docker-compose-up

# Or using Docker Compose directly
docker-compose up -d
```

### 4. Verify

```bash
# Check health
curl http://localhost:8080/health

# Expected response:
# {
#   "status": "healthy",
#   "version": "1.0.0",
#   "timestamp": "2024-01-15T10:35:00Z"
# }
```

---

## Building the Docker Image

### Basic Build

```bash
# Using Make
make docker-build

# Or using Docker directly
docker build -t hub-api-gateway:latest .
```

### Build with Version Tags

```bash
# Build with specific version
VERSION=1.0.0 make docker-build

# Build with build arguments
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t hub-api-gateway:1.0.0 \
  -t hub-api-gateway:latest \
  .
```

### Build Without Cache

```bash
# Force rebuild (useful after dependency changes)
make docker-build-no-cache

# Or
docker build --no-cache -t hub-api-gateway:latest .
```

### Verify Build

```bash
# Check image size
make docker-size

# Or
docker images hub-api-gateway:latest

# Expected output:
# REPOSITORY          TAG       SIZE
# hub-api-gateway     latest    ~15-20MB
```

### Inspect Image

```bash
# View image details
docker inspect hub-api-gateway:latest

# View image labels
docker inspect hub-api-gateway:latest | jq '.[0].Config.Labels'
```

---

## Running with Docker

### Run Container

```bash
# Using Make
make docker-run

# Or using Docker directly
docker run -d \
  --name hub-api-gateway \
  -p 8080:8080 \
  -e JWT_SECRET="your-secret-here" \
  -e REDIS_HOST=redis \
  -e USER_SERVICE_ADDRESS=user-service:50051 \
  hub-api-gateway:latest
```

### Run with Environment File

```bash
docker run -d \
  --name hub-api-gateway \
  -p 8080:8080 \
  --env-file .env \
  hub-api-gateway:latest
```

### Run with Volume Mounts

```bash
docker run -d \
  --name hub-api-gateway \
  -p 8080:8080 \
  -e JWT_SECRET="your-secret-here" \
  -v $(pwd)/config:/app/config:ro \
  -v gateway-logs:/app/logs \
  hub-api-gateway:latest
```

### View Logs

```bash
# Using Make
make docker-logs

# Or using Docker directly
docker logs -f hub-api-gateway

# View last 100 lines
docker logs --tail 100 hub-api-gateway
```

### Access Container Shell

```bash
# Using Make
make docker-shell

# Or using Docker directly
docker exec -it hub-api-gateway /bin/sh
```

### Stop Container

```bash
# Using Make
make docker-stop

# Or using Docker directly
docker stop hub-api-gateway
docker rm hub-api-gateway
```

---

## Running with Docker Compose

### Start All Services

```bash
# Using Make (recommended)
make docker-compose-up

# Or using Docker Compose directly
docker-compose up -d
```

This starts:
- **hub-api-gateway**: API Gateway service (port 8080)
- **hub-redis**: Redis cache (port 6379)

### View Service Status

```bash
# Using Make
make docker-compose-ps

# Or using Docker Compose
docker-compose ps

# Expected output:
# NAME              STATUS    PORTS
# hub-api-gateway   Up        0.0.0.0:8080->8080/tcp
# hub-redis         Up        0.0.0.0:6379->6379/tcp
```

### View Logs

```bash
# All services
make docker-compose-logs

# Specific service
docker-compose logs -f gateway

# Last 50 lines
docker-compose logs --tail=50 gateway
```

### Restart Services

```bash
# Restart all services
make docker-compose-restart

# Restart specific service
docker-compose restart gateway
```

### Stop Services

```bash
# Stop all services
make docker-compose-down

# Stop and remove volumes
docker-compose down -v
```

### Rebuild Services

```bash
# Rebuild and start
make docker-compose-build
docker-compose up -d

# Or in one command
docker-compose up -d --build
```

---

## Configuration

### Environment Variables

The gateway supports configuration via environment variables:

#### Server Configuration

```bash
HTTP_PORT=8080                    # HTTP server port
ENVIRONMENT=development           # Environment (development/staging/production)
SERVER_TIMEOUT=30s                # Request timeout
SHUTDOWN_TIMEOUT=10s              # Graceful shutdown timeout
```

#### Redis Configuration

```bash
REDIS_HOST=localhost              # Redis host
REDIS_PORT=6379                   # Redis port
REDIS_PASSWORD=                   # Redis password (optional)
REDIS_DB=0                        # Redis database number
```

#### Service Addresses

```bash
USER_SERVICE_ADDRESS=localhost:50051              # User service gRPC
MONOLITH_SERVICE_ADDRESS=localhost:50060          # Monolith gRPC
```

#### Authentication

```bash
JWT_SECRET=your-secret-here       # JWT signing secret (REQUIRED)
AUTH_CACHE_ENABLED=true           # Enable token caching
AUTH_CACHE_TTL=5m                 # Cache TTL
```

#### Logging

```bash
LOG_LEVEL=info                    # Log level (debug/info/warn/error)
LOG_FORMAT=json                   # Log format (json/text)
```

#### Rate Limiting

```bash
RATE_LIMIT_ENABLED=true           # Enable rate limiting
RATE_LIMIT_REQUESTS=100           # Requests per window
RATE_LIMIT_WINDOW=1m              # Time window
```

#### Circuit Breaker

```bash
CIRCUIT_BREAKER_ENABLED=true      # Enable circuit breaker
CIRCUIT_BREAKER_THRESHOLD=5       # Failure threshold
CIRCUIT_BREAKER_TIMEOUT=30s       # Timeout before retry
```

### Configuration Files

Mount configuration files as volumes:

```yaml
volumes:
  - ./config:/app/config:ro       # Read-only config mount
```

---

## Health Checks

### Container Health Check

Docker automatically monitors container health:

```bash
# Check health status
docker inspect --format='{{.State.Health.Status}}' hub-api-gateway

# View health check logs
docker inspect --format='{{json .State.Health}}' hub-api-gateway | jq
```

### Manual Health Check

```bash
# Gateway health
curl http://localhost:8080/health

# Expected response:
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:35:00Z"
}

# Redis health
docker exec hub-redis redis-cli ping
# Expected: PONG
```

### Health Check Endpoints

| Endpoint | Description |
|----------|-------------|
| `/health` | Gateway health status |
| `/metrics` | Prometheus metrics |
| `/metrics/json` | JSON metrics |
| `/metrics/summary` | Human-readable metrics |

---

## Monitoring

### View Metrics

```bash
# Prometheus format
curl http://localhost:8080/metrics

# JSON format
curl http://localhost:8080/metrics/json | jq

# Human-readable summary
curl http://localhost:8080/metrics/summary
```

### Key Metrics

```bash
# Request count
gateway_requests_total

# Request latency
gateway_request_duration_seconds

# Cache hit rate
gateway_auth_cache_hits_total / gateway_auth_cache_total

# Error rate
gateway_errors_total / gateway_requests_total

# Circuit breaker status
gateway_circuit_breaker_state
```

### Docker Stats

```bash
# Real-time resource usage
docker stats hub-api-gateway

# Expected:
# CONTAINER         CPU %   MEM USAGE / LIMIT   MEM %   NET I/O
# hub-api-gateway   0.5%    50MB / 512MB        10%     1MB / 2MB
```

### Logs

```bash
# Structured JSON logs
docker logs hub-api-gateway | jq

# Filter by level
docker logs hub-api-gateway | jq 'select(.level=="error")'

# Filter by path
docker logs hub-api-gateway | jq 'select(.path=="/api/v1/orders")'
```

---

## Troubleshooting

### Container Won't Start

**Problem**: Container exits immediately

```bash
# Check logs
docker logs hub-api-gateway

# Common issues:
# 1. Missing JWT_SECRET
docker run -e JWT_SECRET="your-secret" ...

# 2. Port already in use
lsof -i :8080
# Kill process or use different port

# 3. Configuration error
docker run -it hub-api-gateway /bin/sh
# Debug inside container
```

### Cannot Connect to Redis

**Problem**: Token caching fails

```bash
# Check Redis container
docker ps | grep redis

# Test Redis connection
docker exec hub-redis redis-cli ping

# Check network
docker network inspect hub-network

# Gateway should see Redis as "redis" hostname
docker exec hub-api-gateway ping redis
```

### Cannot Connect to Services

**Problem**: gRPC connection failures

```bash
# Check service addresses
docker exec hub-api-gateway env | grep SERVICE_ADDRESS

# Test connectivity
docker exec hub-api-gateway nc -zv user-service 50051

# Verify network
docker network inspect hub-network | jq '.[0].Containers'
```

### High Memory Usage

**Problem**: Container using too much memory

```bash
# Check current usage
docker stats hub-api-gateway --no-stream

# Set memory limit
docker run -m 512m hub-api-gateway

# Or in docker-compose.yml:
deploy:
  resources:
    limits:
      memory: 512M
```

### Authentication Failures

**Problem**: 401 Unauthorized errors

```bash
# Verify JWT_SECRET matches across services
docker exec hub-api-gateway env | grep JWT_SECRET
docker exec hub-user-service env | grep JWT_SECRET

# Test token validation
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}'

# Use token
TOKEN="eyJhbGci..."
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/orders
```

### Build Failures

**Problem**: Docker build fails

```bash
# Clear build cache
docker builder prune -af

# Build with no cache
make docker-build-no-cache

# Check Go module issues
docker run --rm -v $(pwd):/app -w /app golang:1.25.1-alpine go mod verify
```

---

## Production Deployment

### Production Dockerfile

For production, consider these optimizations:

```dockerfile
# Already implemented in our Dockerfile:
# ✅ Multi-stage build (reduces image size)
# ✅ Non-root user (security)
# ✅ Health checks (orchestration)
# ✅ Minimal base image (Alpine)
# ✅ Build arguments (versioning)
```

### Production docker-compose.yml

```yaml
version: '3.8'

services:
  gateway:
    image: hub-api-gateway:1.0.0
    restart: always
    
    # Resource limits
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
      
      # Replicas for high availability
      replicas: 3
    
    # Environment
    environment:
      ENVIRONMENT: production
      LOG_LEVEL: warn
      RATE_LIMIT_ENABLED: "true"
    
    # Secrets (use Docker secrets in production)
    secrets:
      - jwt_secret
    
    # Health check
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3

secrets:
  jwt_secret:
    external: true
```

### Security Best Practices

1. **Use Secrets Management**
   ```bash
   # Docker Swarm secrets
   echo "your-secret" | docker secret create jwt_secret -
   
   # Kubernetes secrets
   kubectl create secret generic jwt-secret --from-literal=JWT_SECRET=your-secret
   ```

2. **Enable TLS**
   ```bash
   # Use reverse proxy (Nginx/Traefik) for TLS termination
   # Or implement TLS in gateway
   ```

3. **Scan for Vulnerabilities**
   ```bash
   # Scan image
   docker scan hub-api-gateway:latest
   
   # Or use Trivy
   trivy image hub-api-gateway:latest
   ```

4. **Use Read-Only Filesystem**
   ```yaml
   services:
     gateway:
       read_only: true
       tmpfs:
         - /tmp
         - /app/logs
   ```

### Monitoring in Production

1. **Prometheus + Grafana**
   ```yaml
   services:
     prometheus:
       image: prom/prometheus
       volumes:
         - ./prometheus.yml:/etc/prometheus/prometheus.yml
     
     grafana:
       image: grafana/grafana
       ports:
         - "3000:3000"
   ```

2. **Log Aggregation**
   ```yaml
   services:
     gateway:
       logging:
         driver: "json-file"
         options:
           max-size: "10m"
           max-file: "3"
   ```

3. **Distributed Tracing**
   ```bash
   # Add Jaeger/Zipkin integration
   # Configure OpenTelemetry
   ```

### Deployment Checklist

- [ ] Set `ENVIRONMENT=production`
- [ ] Configure proper `JWT_SECRET` (use secrets)
- [ ] Enable rate limiting
- [ ] Set resource limits
- [ ] Configure health checks
- [ ] Set up monitoring (Prometheus)
- [ ] Configure log aggregation
- [ ] Enable TLS/HTTPS
- [ ] Scan for vulnerabilities
- [ ] Test failover scenarios
- [ ] Document rollback procedure
- [ ] Set up alerts

---

## Summary

### Common Commands

```bash
# Build
make docker-build

# Run standalone
make docker-run

# Run with compose
make docker-compose-up

# View logs
make docker-compose-logs

# Stop
make docker-compose-down

# Clean up
make docker-clean
```

### Key Files

- `Dockerfile` - Multi-stage build configuration
- `docker-compose.yml` - Service orchestration
- `.dockerignore` - Build context optimization
- `env.example` - Environment variable template
- `Makefile` - Build and deployment commands

### Next Steps

1. ✅ Containerize hub-api-gateway (COMPLETE)
2. 🔄 Containerize hub-user-service (Next)
3. 🔄 Containerize HubInvestmentsServer monolith
4. 🔄 Set up Kubernetes deployment
5. 🔄 Configure CI/CD pipeline

---

**Status**: ✅ Phase 10 - Step 5.1 Complete (Containerization)  
**Service**: hub-api-gateway  
**Image Size**: ~15-20MB (optimized)  
**Security**: Non-root user, minimal base image  
**Production Ready**: ✅ Yes

For questions or issues, refer to the main [README.md](README.md) or contact the platform team.

