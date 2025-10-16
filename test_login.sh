#!/bin/bash

# Test script for API Gateway Login Flow

echo "🧪 Testing Hub API Gateway - Login Flow"
echo "========================================="
echo ""

# Configuration
GATEWAY_URL="http://localhost:8080"
USER_SERVICE_URL="localhost:50051"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "📋 Pre-flight checks:"
echo ""

# Check if gateway is running
echo -n "1. Checking if gateway is running... "
if curl -s "${GATEWAY_URL}/health" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Gateway is running${NC}"
else
    echo -e "${RED}❌ Gateway is not running${NC}"
    echo ""
    echo "Please start the gateway first:"
    echo "  cd hub-api-gateway"
    echo "  export JWT_SECRET=my-test-secret-key-for-testing"
    echo "  export USER_SERVICE_ADDRESS=localhost:50051"
    echo "  ./bin/gateway"
    echo ""
    exit 1
fi

# Check if user service is running
echo -n "2. Checking if user service is running... "
if grpcurl -plaintext ${USER_SERVICE_URL} list > /dev/null 2>&1; then
    echo -e "${GREEN}✅ User service is running${NC}"
else
    echo -e "${YELLOW}⚠️  User service may not be running (optional for this test)${NC}"
fi

echo ""
echo "🔐 Test 1: Login with valid credentials"
echo "----------------------------------------"

# Test login
LOGIN_RESPONSE=$(curl -s -X POST ${GATEWAY_URL}/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }')

echo "Request:"
echo '  POST /api/v1/auth/login'
echo '  {"email": "test@example.com", "password": "password123"}'
echo ""
echo "Response:"
echo "$LOGIN_RESPONSE" | jq '.' 2>/dev/null || echo "$LOGIN_RESPONSE"
echo ""

# Check if login was successful
if echo "$LOGIN_RESPONSE" | grep -q "token"; then
    echo -e "${GREEN}✅ Login successful - Token received${NC}"
    
    # Extract token
    TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token' 2>/dev/null)
    
    if [ ! -z "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
        echo ""
        echo "🎫 JWT Token (first 50 chars):"
        echo "   ${TOKEN:0:50}..."
        echo ""
        
        echo -e "${GREEN}✅ Test 1 PASSED${NC}"
    else
        echo -e "${RED}❌ Test 1 FAILED - Token not found in response${NC}"
    fi
else
    echo -e "${RED}❌ Test 1 FAILED - No token in response${NC}"
fi

echo ""
echo "🔐 Test 2: Login with invalid credentials"
echo "------------------------------------------"

INVALID_RESPONSE=$(curl -s -X POST ${GATEWAY_URL}/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "wrongpassword"
  }')

echo "Request:"
echo '  POST /api/v1/auth/login'
echo '  {"email": "test@example.com", "password": "wrongpassword"}'
echo ""
echo "Response:"
echo "$INVALID_RESPONSE" | jq '.' 2>/dev/null || echo "$INVALID_RESPONSE"
echo ""

if echo "$INVALID_RESPONSE" | grep -q "error"; then
    echo -e "${GREEN}✅ Test 2 PASSED - Correctly rejected invalid credentials${NC}"
else
    echo -e "${RED}❌ Test 2 FAILED - Should have returned error${NC}"
fi

echo ""
echo "🔐 Test 3: Login with missing email"
echo "------------------------------------"

MISSING_EMAIL_RESPONSE=$(curl -s -X POST ${GATEWAY_URL}/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "password": "password123"
  }')

echo "Request:"
echo '  POST /api/v1/auth/login'
echo '  {"password": "password123"}'
echo ""
echo "Response:"
echo "$MISSING_EMAIL_RESPONSE" | jq '.' 2>/dev/null || echo "$MISSING_EMAIL_RESPONSE"
echo ""

if echo "$MISSING_EMAIL_RESPONSE" | grep -q "error"; then
    echo -e "${GREEN}✅ Test 3 PASSED - Correctly rejected missing email${NC}"
else
    echo -e "${RED}❌ Test 3 FAILED - Should have returned validation error${NC}"
fi

echo ""
echo "📊 Test Summary"
echo "================"
echo ""
echo "Gateway URL: ${GATEWAY_URL}"
echo "User Service: ${USER_SERVICE_URL}"
echo ""
echo "All basic tests completed! ✨"
echo ""
echo "Next steps:"
echo "  - Implement token validation middleware (Step 4.3)"
echo "  - Implement request routing (Step 4.4)"
echo "  - Add protected endpoints"

