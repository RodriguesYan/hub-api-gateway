# Proto Contract Management Strategy

## 📋 Overview

This document explains how proto contracts are managed across the Hub Investments microservices architecture.

---

## 🏗️ Current Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Proto Contract Sources                    │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  hub-user-service/proto/                                     │
│  ├── auth_service.proto         (Authentication contracts)   │
│  └── common.proto                (Shared types)              │
│                                                               │
│  HubInvestmentsServer/shared/grpc/proto/                     │
│  ├── monolith_services.proto     (All monolith services)    │
│  ├── balance_service.proto       (Balance service)          │
│  ├── market_data_service.proto   (Market data service)      │
│  ├── order_service.proto         (Order service)            │
│  └── position_service.proto      (Position service)         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            ↓ (manual copy)
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway Contracts                     │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  hub-api-gateway/internal/proto/                             │
│  ├── auth_service.proto          (copied from user-service) │
│  ├── common.proto                 (copied from user-service) │
│  ├── monolith_services.proto      (copied from monolith)    │
│  ├── balance_service.proto        (copied from monolith)    │
│  ├── market_data_service.proto    (copied from monolith)    │
│  ├── order_service.proto          (copied from monolith)    │
│  └── position_service.proto       (copied from monolith)    │
│                                                               │
│  Generated files:                                            │
│  ├── *.pb.go                      (protobuf messages)        │
│  └── *_grpc.pb.go                 (gRPC service stubs)       │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## ⚠️ Current Approach: Manual Duplication

### ✅ Advantages
- **Simple**: No additional infrastructure needed
- **Fast to implement**: Copy-paste and regenerate
- **Independent builds**: Each service has its own contracts
- **No external dependencies**: Services don't depend on shared module

### ❌ Disadvantages
- **No single source of truth**: Multiple copies of same contracts
- **Version drift risk**: Services can get out of sync
- **Manual sync required**: Must remember to copy updated files
- **Breaking changes**: Hard to detect when contracts change
- **Maintenance burden**: More files to manage

---

## 🔄 Sync Process (Current)

### When to Sync
- When proto files are updated in User Service
- When proto files are updated in Monolith
- Before deploying API Gateway
- When adding new RPC methods

### How to Sync

#### Option 1: Using Sync Script (Recommended)
```bash
cd hub-api-gateway
./scripts/sync_proto_files.sh
```

This script will:
1. ✅ Copy all proto files from User Service
2. ✅ Copy all proto files from Monolith
3. ✅ Generate Go code automatically
4. ✅ Show summary of changes

#### Option 2: Manual Sync
```bash
# Copy from User Service
cp hub-user-service/proto/*.proto hub-api-gateway/internal/proto/

# Copy from Monolith
cp HubInvestmentsServer/shared/grpc/proto/*.proto hub-api-gateway/internal/proto/

# Generate Go code
cd hub-api-gateway/internal/proto
protoc --go_out=. --go-grpc_out=. *.proto
```

### After Syncing
1. ✅ Review changes: `git diff internal/proto/`
2. ✅ Test API Gateway: `go build ./...`
3. ✅ Run integration tests: `go test ./...`
4. ✅ Commit changes: `git add internal/proto/ && git commit -m "sync: Update proto contracts"`

---

## 🎯 Recommended Future Approach: Shared Proto Repository

### When to Migrate
- ✅ When you have 3+ microservices
- ✅ When contract changes become frequent
- ✅ When you need strict versioning
- ✅ When you want automated contract validation

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│         hub-proto-contracts (Shared Repository)             │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  auth/                                                        │
│  ├── auth_service.proto                                      │
│  └── common.proto                                            │
│                                                               │
│  monolith/                                                    │
│  ├── balance_service.proto                                   │
│  ├── market_data_service.proto                              │
│  ├── order_service.proto                                     │
│  └── position_service.proto                                  │
│                                                               │
│  common/                                                      │
│  └── types.proto                                             │
│                                                               │
│  go.mod  (Go module for importing)                           │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            ↓ (import as dependency)
┌─────────────────────────────────────────────────────────────┐
│                      All Services                            │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  go.mod:                                                     │
│  require github.com/RodriguesYan/hub-proto-contracts v1.2.3 │
│                                                               │
│  import (                                                     │
│      pb "github.com/RodriguesYan/hub-proto-contracts/auth"  │
│  )                                                            │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Steps

#### Step 1: Create Shared Repository
```bash
# Create new repository
mkdir hub-proto-contracts
cd hub-proto-contracts

# Initialize Go module
go mod init github.com/RodriguesYan/hub-proto-contracts

# Create directory structure
mkdir -p auth monolith common
```

#### Step 2: Move Proto Files
```bash
# Move auth contracts
cp ../hub-user-service/proto/*.proto auth/

# Move monolith contracts
cp ../HubInvestmentsServer/shared/grpc/proto/*.proto monolith/

# Move common types
cp ../hub-user-service/proto/common.proto common/
```

#### Step 3: Add Generation Script
```bash
# scripts/generate.sh
#!/bin/bash

# Generate Go code for all proto files
for dir in auth monolith common; do
    cd $dir
    protoc --go_out=. --go-grpc_out=. *.proto
    cd ..
done
```

#### Step 4: Publish to GitHub
```bash
git init
git add .
git commit -m "Initial commit: Proto contracts v1.0.0"
git tag v1.0.0
git push origin main --tags
```

#### Step 5: Update Services
```bash
# In each service (hub-api-gateway, hub-user-service, etc.)
go get github.com/RodriguesYan/hub-proto-contracts@v1.0.0

# Update imports
import (
    authpb "github.com/RodriguesYan/hub-proto-contracts/auth"
    monolithpb "github.com/RodriguesYan/hub-proto-contracts/monolith"
)
```

### Benefits
- ✅ **Single source of truth**: One repository for all contracts
- ✅ **Version control**: Semantic versioning (v1.0.0, v1.1.0, v2.0.0)
- ✅ **Automated sync**: `go get` updates contracts automatically
- ✅ **Breaking change detection**: Major version bumps signal incompatibility
- ✅ **CI/CD integration**: Auto-generate and publish on commit
- ✅ **Contract validation**: Automated compatibility checks

---

## 📝 Best Practices

### 1. Version Your Contracts
Add version comments to proto files:
```proto
// Version: 1.2.0
// Last updated: 2025-10-20
// Breaking changes: None since v1.0.0

syntax = "proto3";
package auth;
```

### 2. Document Changes
Maintain a CHANGELOG.md in proto repository:
```markdown
## v1.2.0 (2025-10-20)
- Added: GetUserProfile RPC method
- Changed: Login response now includes email field
- Deprecated: OldLoginMethod (use Login instead)

## v1.1.0 (2025-10-15)
- Added: ValidateToken RPC method
```

### 3. Backward Compatibility Rules
- ✅ **DO**: Add new fields (with default values)
- ✅ **DO**: Add new RPC methods
- ✅ **DO**: Deprecate old fields (don't remove)
- ❌ **DON'T**: Remove fields
- ❌ **DON'T**: Change field types
- ❌ **DON'T**: Rename fields

### 4. Testing Strategy
```bash
# Test contract compatibility
buf breaking --against '.git#branch=main'

# Lint proto files
buf lint

# Generate and test
./scripts/generate.sh
go test ./...
```

### 5. CI/CD Pipeline
```yaml
# .github/workflows/proto-contracts.yml
name: Proto Contracts CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Lint proto files
        run: buf lint
      
      - name: Check breaking changes
        run: buf breaking --against '.git#branch=main'
      
      - name: Generate Go code
        run: ./scripts/generate.sh
      
      - name: Run tests
        run: go test ./...
      
      - name: Publish (on tag)
        if: startsWith(github.ref, 'refs/tags/')
        run: |
          git push origin --tags
```

---

## 🔍 Troubleshooting

### Proto files out of sync
```bash
# Check differences
diff hub-user-service/proto/auth_service.proto \
     hub-api-gateway/internal/proto/auth_service.proto

# Sync using script
./scripts/sync_proto_files.sh
```

### Generated code doesn't match
```bash
# Regenerate all proto files
cd hub-api-gateway/internal/proto
rm *.pb.go *_grpc.pb.go
protoc --go_out=. --go-grpc_out=. *.proto
```

### Import path errors
```bash
# Check go.mod has correct dependencies
go mod tidy

# Verify protoc plugins are installed
which protoc-gen-go
which protoc-gen-go-grpc

# Reinstall if needed
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

---

## 📚 Additional Resources

- **Protocol Buffers Guide**: https://protobuf.dev/
- **gRPC Go Quick Start**: https://grpc.io/docs/languages/go/quickstart/
- **Buf Schema Registry**: https://buf.build/
- **Semantic Versioning**: https://semver.org/

---

## 🎯 Summary

| Approach | When to Use | Pros | Cons |
|----------|-------------|------|------|
| **Manual Duplication** (Current) | 1-2 microservices, rapid prototyping | Simple, fast, no dependencies | Version drift, manual sync, no validation |
| **Shared Proto Repository** (Future) | 3+ microservices, production | Single source of truth, versioning, automation | Additional infrastructure, learning curve |

**Current Recommendation**: Continue with manual duplication + sync script until you have 3+ microservices, then migrate to shared repository.

