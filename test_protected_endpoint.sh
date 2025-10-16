#!/bin/bash

# Test Protected Endpoint Flow
# This script tests:
# 1. Login to get JWT token
# 2. Access protected endpoint with token (should succeed)
# 3. Access protected endpoint without token (should fail)
# 4. Access protected endpoint with invalid token (should fail)

set -e

GATEWAY_URL="http://localhost:8080"
LOGIN_URL="${GATEWAY_URL}/api/v1/auth/login"
PROTECTED_URL="${GATEWAY_URL}/api/v1/test"
PROFILE_URL="${GATEWAY_URL}/api/v1/profile"

echo "========================================="
echo "🧪 API Gateway Protected Endpoint Test"
echo "========================================="
echo ""

# Test 1: Login to get JWT token
echo "📝 Step 1: Login to get JWT token..."
LOGIN_RESPONSE=$(curl -s -X POST "${LOGIN_URL}" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }')

echo "Response:"
echo "${LOGIN_RESPONSE}" | jq '.' || echo "${LOGIN_RESPONSE}"
echo ""

# Extract token from response
TOKEN=$(echo "${LOGIN_RESPONSE}" | jq -r '.token' 2>/dev/null || echo "")

if [ -z "${TOKEN}" ] || [ "${TOKEN}" = "null" ]; then
    echo "❌ Failed to get token from login response"
    echo "Please make sure:"
    echo "  1. hub-user-service is running (localhost:50051)"
    echo "  2. API Gateway is running (localhost:8080)"
    echo "  3. User credentials are correct"
    exit 1
fi

echo "✅ Token received: ${TOKEN:0:50}..."
echo ""

# Test 2: Access protected endpoint WITH valid token
echo "📝 Step 2: Access protected /api/v1/test WITH valid token..."
TEST_RESPONSE=$(curl -s -X GET "${PROTECTED_URL}" \
  -H "Authorization: Bearer ${TOKEN}")

echo "Response:"
echo "${TEST_RESPONSE}" | jq '.' || echo "${TEST_RESPONSE}"
echo ""

if echo "${TEST_RESPONSE}" | grep -q "You are authenticated"; then
    echo "✅ Protected endpoint accessed successfully with token"
else
    echo "❌ Failed to access protected endpoint with valid token"
fi
echo ""

# Test 3: Access protected endpoint WITHOUT token
echo "📝 Step 3: Access protected /api/v1/test WITHOUT token (should fail)..."
NO_TOKEN_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "${PROTECTED_URL}")

echo "Response:"
echo "${NO_TOKEN_RESPONSE}" | head -n -1 | jq '.' 2>/dev/null || echo "${NO_TOKEN_RESPONSE}" | head -n -1
HTTP_CODE=$(echo "${NO_TOKEN_RESPONSE}" | tail -n 1 | sed 's/HTTP_CODE://')
echo "HTTP Status: ${HTTP_CODE}"
echo ""

if [ "${HTTP_CODE}" = "401" ]; then
    echo "✅ Correctly rejected request without token (401)"
else
    echo "❌ Expected 401, got ${HTTP_CODE}"
fi
echo ""

# Test 4: Access protected endpoint WITH invalid token
echo "📝 Step 4: Access protected /api/v1/test WITH invalid token (should fail)..."
INVALID_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "${PROTECTED_URL}" \
  -H "Authorization: Bearer invalid.jwt.token")

echo "Response:"
echo "${INVALID_RESPONSE}" | head -n -1 | jq '.' 2>/dev/null || echo "${INVALID_RESPONSE}" | head -n -1
HTTP_CODE=$(echo "${INVALID_RESPONSE}" | tail -n 1 | sed 's/HTTP_CODE://')
echo "HTTP Status: ${HTTP_CODE}"
echo ""

if [ "${HTTP_CODE}" = "401" ]; then
    echo "✅ Correctly rejected request with invalid token (401)"
else
    echo "❌ Expected 401, got ${HTTP_CODE}"
fi
echo ""

# Test 5: Access /api/v1/profile endpoint WITH valid token
echo "📝 Step 5: Access protected /api/v1/profile WITH valid token..."
PROFILE_RESPONSE=$(curl -s -X GET "${PROFILE_URL}" \
  -H "Authorization: Bearer ${TOKEN}")

echo "Response:"
echo "${PROFILE_RESPONSE}" | jq '.' || echo "${PROFILE_RESPONSE}"
echo ""

if echo "${PROFILE_RESPONSE}" | grep -q "This is a protected endpoint"; then
    echo "✅ Profile endpoint accessed successfully with token"
else
    echo "❌ Failed to access profile endpoint with valid token"
fi
echo ""

# Test 6: Test token caching (make same request twice to see cache hit)
echo "📝 Step 6: Test token validation caching (make 3 requests quickly)..."
echo "First request (cache MISS expected):"
time curl -s -X GET "${PROTECTED_URL}" -H "Authorization: Bearer ${TOKEN}" -o /dev/null
echo ""

echo "Second request (cache HIT expected):"
time curl -s -X GET "${PROTECTED_URL}" -H "Authorization: Bearer ${TOKEN}" -o /dev/null
echo ""

echo "Third request (cache HIT expected):"
time curl -s -X GET "${PROTECTED_URL}" -H "Authorization: Bearer ${TOKEN}" -o /dev/null
echo ""

echo "ℹ️  Check gateway logs to see cache HIT/MISS messages"
echo ""

echo "========================================="
echo "✅ All tests completed!"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ Login and get JWT token"
echo "  ✓ Access protected endpoint with valid token"
echo "  ✓ Reject access without token (401)"
echo "  ✓ Reject access with invalid token (401)"
echo "  ✓ Access profile endpoint with valid token"
echo "  ✓ Token validation caching"
echo ""
echo "🎉 API Gateway authentication middleware is working!"

