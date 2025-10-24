#!/bin/bash

# Hub API Gateway - Deployment Script
# This script automates the deployment process for the API Gateway
#
# Usage:
#   ./deploy.sh [environment] [action]
#
# Environments: dev, staging, prod
# Actions: build, start, stop, restart, logs, status

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="hub-api-gateway"
VERSION="${VERSION:-latest}"

# Functions
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

check_prerequisites() {
    print_header "Checking Prerequisites"
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    print_success "Docker: $(docker --version)"
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed"
        exit 1
    fi
    print_success "Docker Compose: $(docker-compose --version)"
    
    # Check .env file
    if [ ! -f "$SCRIPT_DIR/.env" ]; then
        print_warning ".env file not found"
        if [ -f "$SCRIPT_DIR/env.example" ]; then
            print_info "Creating .env from env.example..."
            cp "$SCRIPT_DIR/env.example" "$SCRIPT_DIR/.env"
            print_warning "Please edit .env and set JWT_SECRET before continuing"
            exit 1
        else
            print_error "env.example not found"
            exit 1
        fi
    fi
    print_success ".env file exists"
    
    # Check JWT_SECRET
    if ! grep -q "JWT_SECRET=" "$SCRIPT_DIR/.env" || grep -q "JWT_SECRET=$" "$SCRIPT_DIR/.env"; then
        print_error "JWT_SECRET not set in .env file"
        exit 1
    fi
    print_success "JWT_SECRET is configured"
    
    echo ""
}

build_image() {
    print_header "Building Docker Image"
    
    local build_args=(
        "--build-arg VERSION=${VERSION}"
        "--build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
        "--build-arg GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
    )
    
    if [ "$1" == "no-cache" ]; then
        build_args+=("--no-cache")
        print_info "Building without cache..."
    fi
    
    docker build \
        "${build_args[@]}" \
        -t "${PROJECT_NAME}:${VERSION}" \
        -t "${PROJECT_NAME}:latest" \
        "$SCRIPT_DIR"
    
    print_success "Image built: ${PROJECT_NAME}:${VERSION}"
    
    # Show image size
    local size=$(docker images "${PROJECT_NAME}:${VERSION}" --format "{{.Size}}")
    print_info "Image size: $size"
    
    echo ""
}

start_services() {
    print_header "Starting Services"
    
    local compose_file="docker-compose.yml"
    if [ "$ENVIRONMENT" == "prod" ]; then
        compose_file="docker-compose.prod.yml"
    fi
    
    print_info "Using compose file: $compose_file"
    
    docker-compose -f "$SCRIPT_DIR/$compose_file" up -d
    
    print_success "Services started"
    
    # Wait for health checks
    print_info "Waiting for services to be healthy..."
    sleep 5
    
    # Show status
    show_status
    
    echo ""
    print_success "Deployment complete!"
    print_info "Gateway: http://localhost:8080"
    print_info "Health: http://localhost:8080/health"
    print_info "Metrics: http://localhost:8080/metrics"
}

stop_services() {
    print_header "Stopping Services"
    
    local compose_file="docker-compose.yml"
    if [ "$ENVIRONMENT" == "prod" ]; then
        compose_file="docker-compose.prod.yml"
    fi
    
    docker-compose -f "$SCRIPT_DIR/$compose_file" down
    
    print_success "Services stopped"
    echo ""
}

restart_services() {
    print_header "Restarting Services"
    
    stop_services
    sleep 2
    start_services
}

show_logs() {
    print_header "Viewing Logs"
    
    local compose_file="docker-compose.yml"
    if [ "$ENVIRONMENT" == "prod" ]; then
        compose_file="docker-compose.prod.yml"
    fi
    
    docker-compose -f "$SCRIPT_DIR/$compose_file" logs -f
}

show_status() {
    print_header "Service Status"
    
    local compose_file="docker-compose.yml"
    if [ "$ENVIRONMENT" == "prod" ]; then
        compose_file="docker-compose.prod.yml"
    fi
    
    docker-compose -f "$SCRIPT_DIR/$compose_file" ps
    
    echo ""
    print_info "Health Checks:"
    
    # Check gateway health
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        print_success "Gateway: healthy"
    else
        print_error "Gateway: unhealthy"
    fi
    
    # Check Redis health
    if docker exec hub-redis redis-cli ping > /dev/null 2>&1; then
        print_success "Redis: healthy"
    else
        print_error "Redis: unhealthy"
    fi
    
    echo ""
}

clean_resources() {
    print_header "Cleaning Resources"
    
    print_warning "This will remove containers, images, and volumes"
    read -p "Are you sure? (y/N) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        local compose_file="docker-compose.yml"
        if [ "$ENVIRONMENT" == "prod" ]; then
            compose_file="docker-compose.prod.yml"
        fi
        
        docker-compose -f "$SCRIPT_DIR/$compose_file" down -v
        docker rmi "${PROJECT_NAME}:${VERSION}" 2>/dev/null || true
        docker rmi "${PROJECT_NAME}:latest" 2>/dev/null || true
        
        print_success "Resources cleaned"
    else
        print_info "Cancelled"
    fi
    
    echo ""
}

show_help() {
    cat << EOF
Hub API Gateway - Deployment Script

Usage:
  ./deploy.sh [environment] [action]

Environments:
  dev       Development environment (default)
  staging   Staging environment
  prod      Production environment

Actions:
  build         Build Docker image
  start         Start services
  stop          Stop services
  restart       Restart services
  logs          View logs
  status        Show service status
  clean         Clean resources (containers, images, volumes)
  help          Show this help message

Examples:
  ./deploy.sh dev build         # Build image for development
  ./deploy.sh dev start         # Start development services
  ./deploy.sh prod start        # Start production services
  ./deploy.sh dev logs          # View development logs
  ./deploy.sh dev status        # Check service status

Environment Variables:
  VERSION       Docker image version (default: latest)
  JWT_SECRET    JWT signing secret (required in .env)

EOF
}

# Main script
main() {
    # Parse arguments
    ENVIRONMENT="${1:-dev}"
    ACTION="${2:-help}"
    
    # Validate environment
    if [[ ! "$ENVIRONMENT" =~ ^(dev|staging|prod)$ ]]; then
        print_error "Invalid environment: $ENVIRONMENT"
        print_info "Valid environments: dev, staging, prod"
        exit 1
    fi
    
    # Set environment variable
    export ENVIRONMENT
    
    # Execute action
    case "$ACTION" in
        build)
            check_prerequisites
            build_image
            ;;
        build-no-cache)
            check_prerequisites
            build_image "no-cache"
            ;;
        start)
            check_prerequisites
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            check_prerequisites
            restart_services
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        clean)
            clean_resources
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Unknown action: $ACTION"
            show_help
            exit 1
            ;;
    esac
}

# Run main function
main "$@"

