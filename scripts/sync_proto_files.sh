#!/bin/bash

# Proto File Sync Script
# Copies proto files from source services to API Gateway
# Run this script whenever proto files are updated in source services

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}🔄 Syncing proto files to API Gateway...${NC}"
echo ""

# Project root (assuming script is in hub-api-gateway/scripts/)
GATEWAY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_ROOT="$(cd "$GATEWAY_ROOT/.." && pwd)"

# Source directories
USER_SERVICE_PROTO="$PROJECT_ROOT/hub-user-service/proto"
MONOLITH_PROTO="$PROJECT_ROOT/HubInvestmentsServer/shared/grpc/proto"

# Destination directory
GATEWAY_PROTO="$GATEWAY_ROOT/internal/proto"

# Create destination directory if it doesn't exist
mkdir -p "$GATEWAY_PROTO"

# Function to copy proto files
copy_proto_files() {
    local source=$1
    local dest=$2
    local service_name=$3
    
    if [ ! -d "$source" ]; then
        echo -e "${RED}❌ Source directory not found: $source${NC}"
        return 1
    fi
    
    echo -e "${YELLOW}📂 Copying $service_name proto files...${NC}"
    
    # Copy all .proto files
    find "$source" -name "*.proto" -exec cp {} "$dest/" \;
    
    local count=$(find "$source" -name "*.proto" | wc -l | tr -d ' ')
    echo -e "${GREEN}✅ Copied $count proto file(s) from $service_name${NC}"
    echo ""
}

# Sync from User Service
echo "═══════════════════════════════════════"
echo "  User Service Proto Files"
echo "═══════════════════════════════════════"
copy_proto_files "$USER_SERVICE_PROTO" "$GATEWAY_PROTO" "User Service"

# Sync from Monolith
echo "═══════════════════════════════════════"
echo "  Monolith Proto Files"
echo "═══════════════════════════════════════"
copy_proto_files "$MONOLITH_PROTO" "$GATEWAY_PROTO" "Monolith"

# List all proto files in gateway
echo "═══════════════════════════════════════"
echo "  Proto Files in Gateway"
echo "═══════════════════════════════════════"
ls -lh "$GATEWAY_PROTO"/*.proto 2>/dev/null || echo "No proto files found"
echo ""

# Generate Go code from proto files
echo "═══════════════════════════════════════"
echo "  Generating Go Code"
echo "═══════════════════════════════════════"

cd "$GATEWAY_PROTO"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}❌ protoc not found. Please install Protocol Buffers compiler.${NC}"
    echo "   brew install protobuf  # macOS"
    echo "   apt install protobuf-compiler  # Ubuntu/Debian"
    exit 1
fi

# Check if Go plugins are installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}⚠️  protoc-gen-go not found. Installing...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}⚠️  protoc-gen-go-grpc not found. Installing...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate Go code for all proto files
echo -e "${YELLOW}🔨 Generating Go code from proto files...${NC}"

for proto_file in *.proto; do
    if [ -f "$proto_file" ]; then
        echo "   Generating: $proto_file"
        protoc --go_out=. --go-grpc_out=. "$proto_file"
    fi
done

echo ""
echo -e "${GREEN}✅ Go code generation complete${NC}"
echo ""

# Count generated files
pb_count=$(ls -1 *.pb.go 2>/dev/null | wc -l | tr -d ' ')
grpc_count=$(ls -1 *_grpc.pb.go 2>/dev/null | wc -l | tr -d ' ')

echo "═══════════════════════════════════════"
echo "  Summary"
echo "═══════════════════════════════════════"
echo -e "${GREEN}✅ Proto files synced successfully${NC}"
echo "   Generated $pb_count .pb.go files"
echo "   Generated $grpc_count _grpc.pb.go files"
echo ""
echo -e "${YELLOW}📝 Next steps:${NC}"
echo "   1. Review generated files in: $GATEWAY_PROTO"
echo "   2. Commit changes to git"
echo "   3. Test API Gateway with updated contracts"
echo ""
echo -e "${GREEN}🎉 Done!${NC}"

