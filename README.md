# Codex Workflows Go Worker

A Go-based worker implementation for Codex Workflows that provides function execution capabilities via gRPC communication.

## Architecture

This project follows Go project layout conventions with clear separation of concerns:

- **SDK** (`/sdk`): Public Go library for building workflow functions - contains the high-level API for users
- **Internal** (`/internal`): Private implementation details - core types, communication, state management, and function infrastructure
- **Worker** (`/cmd/worker`): Runnable server executable that registers and executes functions using the SDK

## Quick Start

### Prerequisites

- Go 1.23.1 or later
- Access to a Codex Workflows gRPC server

### Installation & Usage

#### Option 1: Direct GitHub Installation (Recommended)

```bash
# Clone the repository
git clone https://github.com/FatsharkStudiosAB/codex.git
cd codex/workflows/workers/go

# Build the worker
cd cmd/worker
go build -o codex-worker .

# Run with environment variables
export SERVER_NAME="my-go-worker"
export GRPC_SERVER_ADDRESS="localhost:9090"
export SERVER_API_TOKEN="your-token-here"
./codex-worker
```

#### Option 2: Using Go Modules in Your Project

```bash
# In your Go project
go mod init my-worker
go get github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk@latest

# Create main.go (see example below)
go run main.go
```

### Example Usage

Create a simple worker with custom functions:

```go
package main

import (
    "log"
    
    sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"
    "github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
    "github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
)

type GreetingInput struct {
    Name string `json:"name"`
}

type GreetingOutput struct {
    Message string `json:"message"`
}

func main() {
    // Create server
    server, err := sdk.New(
        sdk.WithServerName("my-custom-worker"),
        sdk.WithGrpcServerAddress("localhost:9090"),
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

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_NAME` | `codex-go-worker` | Unique identifier for this worker |
| `GRPC_SERVER_ADDRESS` | `localhost:9090` | Address of the Codex Workflows server |
| `SERVER_API_TOKEN` | _(empty)_ | Authentication token (optional) |

### Docker Usage

```bash
# Build Docker image
cd cmd/worker
docker build -t my-codex-worker .

# Run with environment variables
docker run -e SERVER_NAME=my-worker \
           -e GRPC_SERVER_ADDRESS=host.docker.internal:9090 \
           -e SERVER_API_TOKEN=your-token \
           my-codex-worker
```

## Development

### Project Structure

Following Go project layout conventions, the codebase is organized as follows:

```
/
├── cmd/worker/              # Main application
│   ├── main.go             # Entry point (minimal, imports SDK)
│   ├── examples/           # Example functions
│   └── functions/          # Demo functions
├── sdk/                    # Public API for users
│   ├── sdk.go              # Main SDK interface
│   ├── function.go         # Function builder
│   └── config.go           # Configuration
└── internal/               # Private implementation
    ├── types/              # Core types (EventMessage, etc.)
    ├── basefunction/       # Function implementation infrastructure  
    ├── state/              # Global state management
    ├── communication/      # gRPC communication layer
    ├── handlers/           # Event handlers
    ├── dispatcher/         # Event dispatching
    ├── correlation/        # Request/response correlation
    ├── rpc/                # RPC client
    ├── grpccache/          # Cache client
    ├── grpcstore/          # Store client
    └── workflowsgrpc/      # gRPC protocol implementation
```

**Key Principles:**
- `cmd/worker/main.go` is minimal - only imports SDK and registers functions
- `sdk/` contains public APIs that external developers use
- `internal/` contains implementation details not exposed to users
- No circular dependencies between modules

## Original Development Notes

### Local Development Setup

```bash
# Clone and navigate
git clone https://github.com/FatsharkStudiosAB/codex.git
cd codex/workflows/workers/go

# Install dependencies for all modules
cd internal && go mod tidy
cd ../sdk && go mod tidy  
cd ../cmd/worker && go mod tidy

# Build and test
cd ../cmd/worker
go build ./...
go test ./...
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
fn := sdk.NewFunction[Input, Output](name, version, description)
    .WithHandler(func(input Input, event *types.EventMessage, gs *state.GlobalState) (Output, error) {
        // Access workflow info: event.Workflow, event.Node, etc.
        // Use RPC client: gs.RpcClient
        // Use caching: gs.GrpcCache
        return output, nil
    })
    .WithCacheTTL(5 * time.Minute)
```

## Module Structure

```
workflows/workers/go/
├── sdk/                           # Reusable SDK library
│   ├── go.mod                     # SDK module definition
│   ├── sdk.go                     # Main SDK interface
│   ├── function.go                # Function builders
│   ├── config.go                  # Configuration options
│   └── README.md                  # SDK documentation
└── cmd/worker/                    # Runnable worker server
    ├── go.mod                     # Worker module definition
    ├── main.go                    # Server entrypoint
    ├── examples/                  # Example functions
    ├── Dockerfile                 # Container build
    └── docker-compose.yml         # Local development
```

## Troubleshooting

### Common Issues

**Import Resolution Errors**: Make sure you're using the correct module paths:
- SDK: `github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk`
- Internal packages: `github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/...` (only for internal development)

**Connection Failures**: Verify your `GRPC_SERVER_ADDRESS` points to a running Codex Workflows server.

**Authentication Errors**: Ensure `SERVER_API_TOKEN` is set if the server requires authentication.

### Debug Mode

Set `CODEX_DEBUG=true` for verbose logging:

```bash
export CODEX_DEBUG=true
./codex-worker
```

## License

Part of the Codex Workflows project. See main repository for license details.
