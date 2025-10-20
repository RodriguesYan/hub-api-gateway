#!/bin/bash

# Script to create .env file for hub-api-gateway

cat > .env << 'EOF'
# Hub API Gateway Environment Variables
JWT_SECRET=HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^
CONFIG_PATH=config/config.yaml
EOF

echo "✅ Created .env file with JWT_SECRET"
echo ""
echo "You can now run the gateway with:"
echo "  make run"
echo ""

