# Go Tool Server SDK

This SDK provides a simplified interface for creating and registering functions with the Go Tool Server. It manages server lifecycle and gRPC connectivity.

## Features

- **Type-safe function creation** using Go generics
- **gRPC connectivity** with automatic retries
- **Fluent API** for clean, readable function definitions
- **Minimal boilerplate** - focus on business logic
- **Progressive enhancement** - access to full GlobalState when needed

## Quick Start

### 1. Basic Server Setup

```go
package main

import (
    "go-toolserver/sdk"
    "log"
)

func main() {
    // Create server with default configuration
    server, err := sdk.New()
    if err != nil {
        log.Fatal("Failed to create server:", err)
    }

    // Register your functions
    server.RegisterFunction(NewMyFunction())

    // Start server (blocks forever)
    if err := server.Start(); err != nil {
        log.Fatal("Server failed:", err)
    }
}
```

### 2. Simple Function Creation

```go
// Define input/output types
type GreetingInput struct {
    Name string `json:"name"`
}

type GreetingOutput struct {
    Message string `json:"message"`
}

// Create function using simple interface
func NewGreetingFunction() sdk.FunctionBuilder {
    return sdk.NewSimpleFunction[GreetingInput, GreetingOutput](
        "greeting",
        "1.0.0",
        "Generates a personalized greeting",
    ).WithHandler(greetingHandler).WithTags("utility", "greeting")
}

// Simple handler - just input -> output
func greetingHandler(input GreetingInput) (GreetingOutput, error) {
    if input.Name == "" {
        return GreetingOutput{}, fmt.Errorf("name cannot be empty")
    }
    
    return GreetingOutput{
        Message: fmt.Sprintf("Hello, %s!", input.Name),
    }, nil
}
```

### 3. Advanced Function with State Access

```go
// Create function with access to event state and global state
func NewAdvancedFunction() sdk.FunctionBuilder {
    return sdk.NewFunction[MyInput, MyOutput](
        "advanced",
        "1.0.0",
        "Advanced function with state access",
    ).WithHandler(advancedHandler).WithTags("advanced")
}

// Advanced handler with full access
func advancedHandler(input MyInput, event *types.EventMessage, gs *state.GlobalState) (MyOutput, error) {
    // Access workflow information
    log.Printf("Processing in workflow: %s", event.Workflow)
    
    // Access RPC client through global state
    // gs.RpcClient
    
    return MyOutput{...}, nil
}
```

## Configuration

### Environment Variables

The SDK reads configuration from environment variables by default:

```bash
SERVER_NAME=my-service
GRPC_SERVER_ADDRESS=localhost:9090
```

### Programmatic Configuration

```go
server, err := sdk.New(
    sdk.WithServerName("my-custom-service"),
)
```

## Function Types

### Simple Functions

Use `NewSimpleFunction` for functions that only need input/output transformation:

```go
sdk.NewSimpleFunction[Input, Output](name, version, description)
    .WithHandler(func(Input) (Output, error))
    .WithTags("tag1", "tag2")
```

### Full Functions

Use `NewFunction` for functions that need access to event state or global state:

```go
sdk.NewFunction[Input, Output](name, version, description)
    .WithHandler(func(Input, *types.EventMessage, *state.GlobalState) (Output, error))
    .WithTags("tag1", "tag2")
```

## Migration from Manual Registration

### Before (manual registration)

```go
// init_functions.go
func RegisterAndPublishFunctions(gs *state.GlobalState) {
    functionMap := map[string]basefunction.FunctionInterface{}
    registerFunction(gs, functionMap, input_function.NewFunction(gs))
    publishFunctions(gs, functionMap)
    for key, function := range functionMap {
        gs.Functions.Store(key, function)
    }
}

// main.go
func main() {
    globalState := state.NewGlobalState()
    RegisterAndPublishFunctions(globalState)
    handlers.Activate(globalState)
    select {}
}
```

### After (SDK)

```go
// main.go
func main() {
    server, err := sdk.New()
    if err != nil {
        log.Fatal(err)
    }
    
    server.RegisterFunction(input_function.NewSDKFunction())
    
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Examples

See the `examples/` directory for complete working examples:

- `examples/main_sdk_demo.go` - Basic server setup
- `examples/calculator_function.go` - Complete calculator service with multiple functions

## Advanced Usage

### Accessing Global State

```go
func advancedHandler(input MyInput, event *types.EventMessage, gs *state.GlobalState) (MyOutput, error) {
    // Access RPC client for calling other services
    response, err := gs.RpcClient.Call(...)
    _ = response
    // Access function cache via grpccache if configured on the workflow side
    return MyOutput{...}, nil
}
```

### Custom Configuration

```go
type MyConfig struct {
    DatabaseURL string
    APIKey     string
}

func NewMyFunction(config MyConfig) sdk.FunctionBuilder {
    return sdk.NewFunction[Input, Output](...).WithHandler(
        func(input Input, event *types.EventMessage, gs *state.GlobalState) (Output, error) {
            // Use your custom config
            db := connectTo(config.DatabaseURL)
            api := newClient(config.APIKey)
            // ...
        },
    )
}
```

## Best Practices

1. **Keep handlers pure** - minimize side effects
2. **Use simple functions** when you don't need state access
3. **Validate inputs** early in your handlers
4. **Use meaningful error messages** for better debugging
5. **Tag your functions** for easier discovery and organization
6. **Keep function names and versions consistent** across deployments

## Error Handling

Functions should return meaningful errors:

```go
func myHandler(input MyInput) (MyOutput, error) {
    if input.Value < 0 {
        return MyOutput{}, fmt.Errorf("value must be non-negative, got: %d", input.Value)
    }
    
    if input.Name == "" {
        return MyOutput{}, fmt.Errorf("name is required")
    }
    
    // ... business logic
    
    return output, nil
}
```

The SDK will automatically handle error serialization and transmission back to the workflow engine.

## Event Constants and Dispatcher

Event names are centralized in `types/events.go`. Use these canonical names when sending or handling events:

- Function: `function_request`, `function_response`
- Cache: `cache_get_request`, `cache_get_response`, `cache_set`, `cache_set_response`
- Store: `store_get_request`, `store_get_response`, `store_set_request`, `store_set_response`
- Discovery: `request_server_name`, `response_server_name`, `request_list_functions`, `response_list_functions`, `request_server_info`
- Misc: `status_message`, `error`, `client_registration`

The SDK initializes a dispatcher with a bounded queue and worker pool to process events concurrently, and handlers are registered in `handlers/` by event name. This keeps user handlers linear and avoids callback-heavy flows. For portability, replicate the dispatcher with the same behavior in other languages.

## Architecture Overview

See `ARCHITECTURE.md` for a high-level view of modules and how to port them to other languages.



