# Dibbla SDK for Go

A Go SDK for building workflow functions with gRPC communication and automatic TLS support.

## Architecture

This project follows Go project layout conventions with clear separation of concerns:

- **Root Package** (`sdk`): Public Go library for building workflow functions - contains the high-level API for users
- **Internal** (`/internal`): Private implementation details - core types, communication, state management, and function infrastructure
- **Worker Example** (`/cmd/worker`): Runnable server executable demonstrating SDK usage

## Quick Start

### Prerequisites

- Go 1.23.1 or later
- Access to a gRPC workflow server

### Installation

```bash
# In your Go project
go get github.com/dibbla-agents/sdk-go@latest
```

### Example Usage

Create a simple worker with custom functions:

```go
package main

import (
    "fmt"
    "log"

    "github.com/dibbla-agents/sdk-go"
)

type GreetingInput struct {
    Name string `json:"name"`
}

type GreetingOutput struct {
    Message string `json:"message"`
}

func main() {
    // Create server with minimal configuration
    // (defaults to grpc.dibbla.com:443 with TLS enabled)
    server, err := sdk.New(
        sdk.WithServerName("my-custom-worker"),
        sdk.WithServerApiToken("your-api-token"),
    )
    if err != nil {
        log.Fatal("Failed to create server:", err)
    }

    // Register a simple function
    greetingFn := sdk.NewSimpleFunction[GreetingInput, GreetingOutput](
        "greeting", "1.0.0", "Generate a greeting message",
    ).WithHandler(func(input GreetingInput) (GreetingOutput, error) {
        return GreetingOutput{
            Message: fmt.Sprintf("Hello, %s!", input.Name),
        }, nil
    }).WithTags("utility", "greeting")

    server.RegisterFunction(greetingFn)

    // Start server (blocks forever)
    log.Println("Starting worker...")
    if err := server.Start(); err != nil {
        log.Fatal("Server failed:", err)
    }
}
```

## Configuration

### Environment Variables

| Variable              | Default              | Description                                    |
| --------------------- | -------------------- | ---------------------------------------------- |
| `SERVER_NAME`         | `codex-go-worker`    | Unique identifier for this worker              |
| `GRPC_SERVER_ADDRESS` | `grpc.dibbla.com:443` | Address of the workflow server                 |
| `SERVER_API_TOKEN`    | _(empty)_            | Authentication token                           |
| `GRPC_USE_TLS`        | _(auto-detect)_      | Enable/disable TLS (`true`, `false`, or empty) |

### TLS Configuration

The SDK defaults to `grpc.dibbla.com:443` with TLS enabled. It automatically detects when to use TLS based on the server address:

- **Production addresses** (default: `grpc.dibbla.com:443`): TLS enabled with system certificates
- **Localhost addresses** (`localhost:`, `127.0.0.1:`, `[::1]:`): No TLS (for local development)

#### Explicit TLS Control

You can override the auto-detection:

```go
// Minimal configuration - uses grpc.dibbla.com:443 with TLS (recommended)
server, _ := sdk.New(
    sdk.WithServerName("my-worker"),
    sdk.WithServerApiToken("your-token"),
)

// Local development - uses localhost without TLS
server, _ := sdk.New(
    sdk.WithServerName("my-worker"),
    sdk.WithGrpcServerAddress("localhost:50051"),
)

// Force TLS on for localhost (advanced)
server, _ := sdk.New(
    sdk.WithServerName("my-worker"),
    sdk.WithGrpcServerAddress("localhost:9090"),
    sdk.WithGrpcTLS(true),
)
```

#### Environment Variable Override

```bash
# Force TLS on (overrides auto-detection)
export GRPC_USE_TLS=true

# Force TLS off
export GRPC_USE_TLS=false

# Let SDK auto-detect (default)
# Don't set GRPC_USE_TLS or leave it empty
```

### Docker Usage

```bash
# Build Docker image
docker build -t my-worker .

# Run with minimal configuration (uses grpc.dibbla.com:443 by default)
docker run -e SERVER_NAME=my-worker \
           -e SERVER_API_TOKEN=your-token \
           my-worker

# Or with custom server address
docker run -e SERVER_NAME=my-worker \
           -e GRPC_SERVER_ADDRESS=custom.example.com:443 \
           -e SERVER_API_TOKEN=your-token \
           my-worker
```

## Development

### Project Structure

```
sdk-go/
├── sdk.go, config.go, function.go  # Public API (package sdk)
├── go.mod                          # Single module for entire project
├── internal/                       # Private implementation
│   ├── communication/              # gRPC communication with TLS
│   ├── state/                      # Global state management
│   ├── handlers/                   # Event handlers
│   ├── basefunction/               # Function infrastructure
│   └── ...                         # Other internal packages
└── cmd/worker/                     # Example application
    └── main.go                     # Demonstration server
```

### Creating Custom Functions

The SDK provides two types of functions:

#### Simple Functions

For basic input → output transformations:

```go
fn := sdk.NewSimpleFunction[Input, Output](name, version, description)
    .WithHandler(func(input Input) (Output, error) {
        // Your logic here
        return output, nil
    })
    .WithTags("tag1", "tag2")
```

#### Advanced Functions

For functions needing access to workflow context:

```go
import (
    "github.com/dibbla-agents/sdk-go"
    "github.com/dibbla-agents/sdk-go/internal/types"
    "github.com/dibbla-agents/sdk-go/internal/state"
)

fn := sdk.NewFunction[Input, Output](name, version, description)
    .WithHandler(func(input Input, event *types.EventMessage, gs *state.GlobalState) (Output, error) {
        // Access workflow info: event.Workflow, event.Node, etc.
        // Use RPC client: gs.RpcClient
        // Use caching: gs.GrpcCache
        return output, nil
    })
    .WithCacheTTL(5 * time.Minute)
```

## Migration from Previous Versions

If you're migrating from `github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk`:

### Import Path Changes

```go
// Old import
import sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"

// New import - clean and simple!
import "github.com/dibbla-agents/sdk-go"
```

### API Changes

```go
// Old (from FatsharkStudiosAB/codex)
import sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"
server := sdk.New()

// New (dibbla-agents/sdk-go)
import "github.com/dibbla-agents/sdk-go"
server := sdk.New()  // package name is still "sdk"
```

### Steps to Migrate

1. Update your imports:
   ```bash
   # Update go.mod
   go get github.com/dibbla-agents/sdk-go@latest
   go mod tidy
   ```

2. Update your code:
   - Change imports from old path to `github.com/dibbla-agents/sdk-go`
   - The package name remains `sdk`, so `sdk.New()` stays the same
   - All other APIs remain the same!

3. (Optional) Enable TLS if connecting to production:
   ```bash
   export GRPC_USE_TLS=true
   ```

## Features

### Automatic TLS Support

- Smart auto-detection based on server address
- System certificate validation for production
- Easy override for testing scenarios
- Cloudflare-compatible TLS termination

### Type-Safe Functions

- Generic function builders with full type safety
- Compile-time type checking for inputs and outputs
- Automatic JSON schema generation

### Built-in Caching

- Per-function cache TTL configuration
- Automatic cache key generation
- gRPC-based distributed cache

### Robust Connection Management

- Automatic reconnection on failure
- Configurable health checks
- Connection state monitoring

## Troubleshooting

### Common Issues

**Import Resolution Errors**: Make sure you're using the correct module path:
```
github.com/dibbla-agents/sdk-go
```

**Connection Failures**: 
- By default, the SDK connects to `grpc.dibbla.com:443` with TLS enabled
- For local development, set `GRPC_SERVER_ADDRESS=localhost:50051`
- Verify your server address points to a running workflow server
- Check TLS configuration matches your server setup

**Authentication Errors**: 
- Ensure `SERVER_API_TOKEN` is set if the server requires authentication
- Check token is valid and not expired

**TLS Certificate Errors**:
- Ensure system CA certificates are up to date
- For self-signed certificates, you may need to disable TLS verification (not recommended for production)

### Debug Mode

Enable verbose logging:

```bash
export CODEX_DEBUG=true
./your-worker
```

## License

Part of the Dibbla project.
