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
| `GRPC_SERVER_ADDRESS` | `grpc.dibbla.com:443` | Address of the workflow server (the proxy gRPC endpoint) |
| `SERVER_API_TOKEN`    | _(empty)_            | Your Dibbla API token (`ak_…`, created in the console) — only needed for local development |
| `DIBBLA_IDENTITY_TOKEN_FILE` | _(platform-set)_ | Path to the workload-identity token; the Dibbla platform injects this automatically |
| `SERVER_ORG_ID`       | _(empty)_            | Optional: pin registration to a specific org (multi-org token owners) |
| `GRPC_USE_TLS`        | _(auto-detect)_      | Enable/disable TLS (`true`, `false`, or empty) |

#### Authentication & org scoping

**Deployed on the Dibbla platform: no token needed.** The platform mints a
workload identity for every deployment (a projected, auto-rotating
Kubernetes token) and the SDK picks it up automatically — your worker
authenticates as its tenant + deployment with zero configuration, and there
is no credential to provision, rotate, or leak.

**Local development:** set `SERVER_API_TOKEN` to a personal API token
(`ak_…`) generated in the Dibbla console. An explicit API token always wins
over the workload identity, so you can also use one in-cluster to override.
The connection is authenticated centrally by the proxy against the
auth-service — the same flow as the rest of the platform — and the functions
your tool server registers are scoped to your organization.

If your token's account belongs to multiple organizations, set `SERVER_ORG_ID`
(or `sdk.WithOrgID("org_…")`) to choose which one this tool server registers
under; otherwise the token's default organization is used.

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
├── jobs/                           # Jobs subpackage for long-running tasks
│   ├── types.go                    # JobHandler interface, JobParameter, JobEventMeta
│   ├── context.go                  # JobContext + type-safe arg helpers
│   ├── logger.go                   # Logger for job events via gRPC
│   ├── registry.go                 # Internal job handler registry
│   ├── progress.go                 # Terminal progress bar
│   └── helpers.go                  # Utility functions
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

### Long-Running Jobs

For long-running background tasks that need progress reporting and task tracking, use the Jobs subpackage. Jobs run asynchronously and can report progress, log messages, and track individual tasks.

#### Registering Jobs

Jobs are registered directly on the SDK server. There is no separate `JobHost` — `server.RegisterJob` is the only entry point, and `server.Start()` advertises every registered job in the workflow registration broadcast.

```go
import (
    "log"
    "os"

    "github.com/dibbla-agents/sdk-go"
)

func main() {
    server, err := sdk.New(
        sdk.WithServerName("my-worker"),
        sdk.WithServerApiToken(os.Getenv("SERVER_API_TOKEN")),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Register functions and jobs in any order, before Start().
    server.RegisterFunction(myFunctionBuilder)
    server.RegisterJob(&DataProcessingJob{})

    // Start handles connection, registration, and dispatch. Blocks forever.
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

#### Implementing a Job Handler

Jobs implement the `JobHandler` interface:

```go
import "github.com/dibbla-agents/sdk-go/jobs"

type DataProcessingJob struct{}

func (j *DataProcessingJob) GetJobID() string   { return "data_processing" }
func (j *DataProcessingJob) GetJobName() string { return "Data Processing Job" }

func (j *DataProcessingJob) GetParameters() []jobs.JobParameter {
    return []jobs.JobParameter{
        {Name: "source", Type: "string", Required: true},
        {Name: "batch_size", Type: "integer", Required: false, Default: 100},
    }
}

func (j *DataProcessingJob) Execute(ctx *jobs.JobContext) error {
    source := ctx.GetStringArg("source", "default")
    batchSize := ctx.GetIntArg("batch_size", 100)

    ctx.Logger.Info(fmt.Sprintf("Starting data processing from %s (batch: %d)", source, batchSize))

    // Task 1: Fetch data
    ctx.Logger.TaskStarted("fetch_data")
    // ... fetch logic
    ctx.Logger.TaskCompleted()

    // Task 2: Process with progress reporting
    ctx.Logger.TaskStarted("process_data")
    total := 150
    for i := 0; i < total; i++ {
        ctx.Logger.Progress(i+1, total, "Processing records")
        // ... processing logic
    }
    ctx.Logger.CompleteProgress()
    ctx.Logger.TaskCompleted()

    ctx.Logger.Info("Data processing completed successfully")
    return nil
}
```

#### Job Context

The `JobContext` provides:

- `RunID`, `JobID`, `JobName` - Identifiers for the current job execution
- `Args` - Arguments passed when the job was triggered
- `Logger` - Logger for sending events via gRPC
- Helper methods: `GetStringArg()`, `GetIntArg()`, `GetBoolArg()`, `GetFloat64Arg()`

#### Logger Methods

The job logger sends events via gRPC and outputs to console:

```go
ctx.Logger.Info("message")           // Info log
ctx.Logger.Warn("message")           // Warning log
ctx.Logger.Error("message")          // Error log

ctx.Logger.TaskStarted("task_name")  // Start a task
ctx.Logger.TaskCompleted()           // Complete current task
ctx.Logger.TaskFailed(err)           // Fail current task
ctx.Logger.TaskSkipped("reason")     // Skip current task

ctx.Logger.Progress(50, 100, "msg")  // Progress with total
ctx.Logger.CompleteProgress()        // Finish progress bar
```

### Capability Providers

Capability providers let a worker replace a platform-built-in behavior for agent
nodes that bind it — by capability "seat". A provider is registered alongside
functions and announced automatically on startup and reconnect; a workflow binds
it on an agent node via `capability_providers: {<seat>: "<provider-name>"}` in
slim YAML. Registering providers is fully additive: workers that register none
behave exactly as before.

#### tool_search — custom tool selection

Replaces the built-in candidate selection for the agent's `tool_search` tool.
The platform owns sourcing (you always select from the offered stubs, never add
tools); the provider owns ranking/selection:

```go
server.RegisterCapabilityProvider(sdk.ToolSearchProvider{
    Name:        "my-search",
    Description: "Ranks tools by my own scoring.",
    Version:     "1.0.0",
    Select: func(query string, stubs []sdk.ProviderStub, topN int) ([]string, error) {
        // stubs carry Name + Description; return up to topN names from the
        // offered set, in order. Unknown names are dropped engine-side.
        return rank(query, stubs, topN), nil
    },
    // WantsCatalogSync: true  // ask for a catalog pre-push (e.g. to pre-index)
})
```

A provider registered with a nil `Select` still appears for binding and answers
calls with an explicit "not supported" error. See `cmd/worker/examples/tool_search_provider.go`
for a runnable example, and the doc comments on `ToolSearchProvider` for the full
contract.

#### memory — custom history policy

Replaces the built-in conversation replay for agent nodes with
`history_policy: custom`. The platform keeps blob custody; your transform picks
what gets injected into the model's context, under an enforced token budget:

```go
server.RegisterCapabilityProvider(sdk.MemoryProvider{
    Name:               "my-memory",
    Version:            "1.0.0",
    MaxHistoryFraction: 0.5, // optional share of the context window (clamped)
    Transform: func(currentMessage string, turns []sdk.Turn, tokenBudget int, meta sdk.ThreadMeta) ([]sdk.Turn, error) {
        // turns = the thread's stored history (v2 Turn/Part shapes).
        // Return the turns to inject, in order, under tokenBudget.
        // currentMessage is for query-conditioned retrieval only — the engine
        // appends the real user message itself.
        return selectRelevant(currentMessage, turns, tokenBudget), nil
    },
})
```

Returned turns must carry `user`/`assistant` roles with known part types;
exceeding the token ceiling (or an absolute byte cap) fails the node fail-fast —
there is no fallback policy. Note the data boundary: unlike tool_search, the
memory seat sends full conversation content (text, tool args/results, reasoning)
to the provider's server. Returned turns are injection-only for that run and are
never written back to the stored thread. See `cmd/worker/examples/memory_provider.go`
and the `MemoryProvider` doc comments for the full contract.

### Removal of `JobHost`

The `JobHost` abstraction has been removed. Jobs now register directly on the server via `server.RegisterJob(handler)`, and the server handles gRPC initialization, the trigger dispatcher, and job lifecycle events as part of `server.Start()`.

```go
// Old — no longer compiles
jobHost, _ := server.NewJobHost("my-job-host")
jobHost.RegisterJob(&MyJob{})
jobHost.Start()

// Current
server.RegisterJob(&MyJob{})
server.Start()
```

If you have code calling `server.NewJobHost(...)` or `jobs.NewJobHost(...)`, replace it with `server.RegisterJob(...)` calls and remove the separate `Start()` on the host — the SDK server now drives everything.

### Migrating from FatsharkStudiosAB/codex

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

### Long-Running Jobs

- Direct job registration on the SDK server (`server.RegisterJob`) for background task execution via gRPC
- Progress reporting with terminal progress bar
- Task tracking (started, completed, failed, skipped)
- Structured logging sent as gRPC events
- Full integration with existing SDK infrastructure

### OAuth Access Tokens

The SDK provides built-in support for requesting OAuth access tokens on behalf of the user running the workflow. This allows your functions to call third-party APIs (Google, Microsoft, GitHub) using the user's connected accounts.

#### Supported Providers

- `oauth.ProviderGoogle` - Google APIs (Gmail, Calendar, Drive, etc.)
- `oauth.ProviderMicrosoft` - Microsoft APIs (Outlook, OneDrive, Teams, etc.)
- `oauth.ProviderGitHub` - GitHub API

#### Requesting an Access Token

```go
import (
    "context"
    "time"

    sdk "github.com/dibbla-agents/sdk-go"
    "github.com/dibbla-agents/sdk-go/internal/oauth"
    "github.com/dibbla-agents/sdk-go/internal/state"
    "github.com/dibbla-agents/sdk-go/internal/types"
)

func NewGoogleAPIFunction() sdk.FunctionBuilder {
    return sdk.NewFunction[MyInput, MyOutput](
        "call_google_api",
        "1.0.0",
        "Calls a Google API on behalf of the user",
    ).WithHandler(func(input MyInput, event *types.EventMessage, gs *state.GlobalState) (MyOutput, error) {
        // Get an access token for Google
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        token, err := gs.OAuth.GetAccessToken(ctx, oauth.ProviderGoogle, event.Run)
        if err != nil {
            return MyOutput{}, fmt.Errorf("failed to get Google token: %w", err)
        }

        // Use the token to call Google APIs
        // token.AccessToken - the bearer token
        // token.TokenType   - typically "Bearer"
        // token.ExpiresAt   - Unix timestamp when token expires

        return MyOutput{...}, nil
    })
}
```

#### Checking Connected Providers

You can check which OAuth providers the user has connected before attempting to request tokens:

```go
func NewCheckConnectionsFunction() sdk.FunctionBuilder {
    return sdk.NewFunction[EmptyInput, ProvidersOutput](
        "check_connections",
        "1.0.0",
        "Checks which OAuth providers the user has connected",
    ).WithHandler(func(input EmptyInput, event *types.EventMessage, gs *state.GlobalState) (ProvidersOutput, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        providers, err := gs.OAuth.GetConnectedProviders(ctx, event.Run)
        if err != nil {
            return ProvidersOutput{}, err
        }

        // providers is a map[string]*oauth.ProviderStatus
        // Each status contains: Email, LastUsed, Scopes

        return ProvidersOutput{Providers: providers}, nil
    })
}
```

#### OAuth Error Handling

When a user hasn't connected a provider, the OAuth request will return an error. Handle this gracefully:

```go
token, err := gs.OAuth.GetAccessToken(ctx, oauth.ProviderGoogle, event.Run)
if err != nil {
    // Check if it's an OAuth-specific error
    if oauthErr, ok := err.(*oauth.OAuthError); ok {
        switch oauthErr.Code {
        case "not_connected":
            return Output{}, fmt.Errorf("Please connect your Google account first")
        case "token_expired":
            return Output{}, fmt.Errorf("Your Google connection needs to be refreshed")
        default:
            return Output{}, fmt.Errorf("OAuth error: %s", oauthErr.Message)
        }
    }
    return Output{}, err
}
```

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
