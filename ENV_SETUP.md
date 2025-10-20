# Environment Setup for hub-api-gateway

## ✅ Fixed: No More Manual Exports!

The hub-api-gateway now automatically loads configuration from a `.env` file, just like HubInvestmentsServer and hub-user-service.

## Quick Setup

### 1. Create .env file (one-time setup)

```bash
cd hub-api-gateway
./create-env.sh
```

### 2. Run the gateway

```bash
# Option 1: Using make
make run

# Option 2: Direct go run
go run cmd/server/main.go
```

**That's it!** No need to export JWT_SECRET anymore! 🎉

## What Changed?

### Before (Required Manual Export)
```bash
# You had to do this every time:
export JWT_SECRET="HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^"
go run cmd/server/main.go
```

### After (Automatic)
```bash
# Now it just works:
go run cmd/server/main.go
```

## How It Works

The gateway now uses `godotenv` (same as HubInvestmentsServer and hub-user-service) to automatically load environment variables from the `.env` file.

```go
// internal/config/config.go
func Load() (*Config, error) {
    // Automatically loads .env file
    err := godotenv.Load(".env")
    if err != nil {
        log.Printf("⚠️  Could not load .env file: %v", err)
        log.Println("Using environment variables or default values...")
    } else {
        log.Println("✅ Loaded configuration from .env file")
    }
    // ... rest of config loading
}
```

## Configuration Files

### .env (Your Active Configuration)
```bash
# Hub API Gateway Environment Variables
JWT_SECRET=HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^
CONFIG_PATH=config/config.yaml
```

**Note:** `.env` is in `.gitignore` and should never be committed.

### create-env.sh (Helper Script)
A convenience script to quickly create the `.env` file with the correct JWT_SECRET.

## Configuration Priority

The gateway loads configuration in this order:

1. **`.env` file** (if exists) ← **NEW!**
2. **Environment variables** (override .env)
3. **Default values** (fallback)

This means you can still override values with environment variables if needed:

```bash
# Override just one value
JWT_SECRET="different-secret" go run cmd/server/main.go

# Or export for the session
export JWT_SECRET="different-secret"
go run cmd/server/main.go
```

## Consistency Across Services

All three services now work the same way:

| Service | Config File | Auto-loads? |
|---------|-------------|-------------|
| HubInvestmentsServer | `config.env` | ✅ YES |
| hub-user-service | `config.env` | ✅ YES |
| hub-api-gateway | `.env` | ✅ YES |

## Environment Variables

The gateway reads these environment variables from `.env`:

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | JWT signing secret (MUST match other services) | *required* |
| `HTTP_PORT` | HTTP server port | `8080` |
| `REDIS_HOST` | Redis server host | `localhost` |
| `REDIS_PORT` | Redis server port | `6379` |
| `USER_SERVICE_ADDRESS` | User service gRPC address | `localhost:50051` |
| `HUB_MONOLITH_ADDRESS` | Monolith gRPC address | `localhost:50060` |

See `config/config.yaml` for more configuration options.

## Troubleshooting

### "JWT_SECRET environment variable is required"

**Solution:** Create the `.env` file:
```bash
./create-env.sh
```

### ".env file not found"

**Solution:** You're in the wrong directory. Make sure you're in `hub-api-gateway/`:
```bash
cd hub-api-gateway
./create-env.sh
```

### "Could not load .env file"

This is just a warning. The gateway will still work if you have environment variables set. But it's better to create the `.env` file for convenience.

## Migration Guide

If you have existing scripts or documentation that export JWT_SECRET, you can simplify them:

### Old Way
```bash
#!/bin/bash
export JWT_SECRET="HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^"
export REDIS_HOST="localhost"
export REDIS_PORT="6379"
cd hub-api-gateway
go run cmd/server/main.go
```

### New Way
```bash
#!/bin/bash
cd hub-api-gateway
# Just run it - .env is loaded automatically!
go run cmd/server/main.go
```

## Best Practices

1. **Never commit `.env`** - It's already in `.gitignore`
2. **Use strong secrets in production** - The example secret is for development only
3. **Keep secrets in sync** - JWT_SECRET must match across all services
4. **Use different secrets per environment** - dev, staging, production should have different secrets

## Related Files

- `.env` - Your active configuration (created by `create-env.sh`)
- `create-env.sh` - Helper script to create `.env`
- `internal/config/config.go` - Configuration loading logic
- `config/config.yaml` - Additional YAML configuration
- `Makefile` - Build and run commands

## Summary

✅ **No more manual exports!**  
✅ **Consistent with other services**  
✅ **Simpler development workflow**  
✅ **One-time setup with `create-env.sh`**  

Just create the `.env` file once and you're good to go! 🚀

