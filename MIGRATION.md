# Migration Guide: v0.0.0 → v0.1.0

## Summary of Changes

This release restructures the SDK into a single Go module with a clean, branded API and adds automatic TLS support for production deployments.

## What Changed

### 1. Module Restructuring

**Before:**
```
github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk
```

**After:**
```
github.com/dibbla-agents/sdk-go
```

### 2. Package Name

**Before:**
```go
import sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"

server := sdk.New()
```

**After:**
```go
import "github.com/dibbla-agents/sdk-go"

server := dibbla.New()
```

### 3. TLS Support Added

The SDK now automatically detects and enables TLS for production deployments:

- **Localhost** (`localhost:`, `127.0.0.1:`, `[::1]:`): No TLS
- **Production addresses**: TLS enabled with system certificates

## Migration Steps

### Step 1: Update Dependencies

```bash
go get github.com/dibbla-agents/sdk-go@latest
go mod tidy
```

### Step 2: Update Imports

Replace all occurrences:

```go
// Old
import sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"

// New
import "github.com/dibbla-agents/sdk-go"
```

### Step 3: Update Code

Replace all `sdk.` prefixes with `dibbla.`:

```go
// Old
server, err := sdk.New(
    sdk.WithServerName("my-worker"),
    sdk.WithGrpcServerAddress("localhost:9090"),
)

// New
server, err := dibbla.New(
    dibbla.WithServerName("my-worker"),
    dibbla.WithGrpcServerAddress("localhost:9090"),
)
```

### Step 4: Configure TLS (Optional)

For production deployments with Cloudflare or other TLS terminators:

```bash
# Let SDK auto-detect (recommended)
export GRPC_SERVER_ADDRESS="api.example.com:443"
# TLS will be automatically enabled

# Or explicitly enable
export GRPC_USE_TLS=true
```

For local development, no changes needed:

```bash
# Localhost automatically disables TLS
export GRPC_SERVER_ADDRESS="localhost:9090"
```

### Step 5: Test

```bash
go build ./...
go test ./...
```

## New Features

### Automatic TLS Detection

```go
// Automatically uses TLS for production addresses
server, _ := dibbla.New(
    dibbla.WithGrpcServerAddress("api.example.com:443"),
)

// Automatically disables TLS for localhost
server, _ := dibbla.New(
    dibbla.WithGrpcServerAddress("localhost:9090"),
)
```

### Explicit TLS Control

```go
// Override auto-detection if needed
server, _ := dibbla.New(
    dibbla.WithGrpcServerAddress("localhost:9090"),
    dibbla.WithGrpcTLS(true), // Force TLS on
)
```

### Environment Variable Configuration

```bash
# Force TLS on
export GRPC_USE_TLS=true

# Force TLS off  
export GRPC_USE_TLS=false

# Auto-detect (default)
# Don't set GRPC_USE_TLS
```

## Breaking Changes

1. **Import path changed**: Update all imports from old path to `github.com/dibbla-agents/sdk-go`
2. **Package name changed**: Update all `sdk.` references to `dibbla.`

## Non-Breaking Changes

All APIs remain the same except for the package name. Function signatures, configuration options, and behavior are unchanged.

## Compatibility

- **Go Version**: 1.23.1 or later (unchanged)
- **TLS**: Backward compatible - localhost addresses work without TLS as before
- **API**: All existing options and methods work identically

## Rollback

If you need to rollback:

```bash
go get github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk@v0.0.0
go mod tidy
```

Then revert your import and code changes.

## Support

For issues or questions:
- Check the updated [README.md](./README.md)
- Review TLS configuration in the README
- Ensure `GRPC_SERVER_ADDRESS` is correct for your environment


