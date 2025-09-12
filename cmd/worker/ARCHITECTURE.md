## Architecture Overview

This repository implements a tool server that connects to a workflow engine via gRPC and exposes typed functions that can be invoked remotely. The system is designed to be portable across languages by keeping the protocol and core abstractions simple and explicit.

### Core Modules

- `workflowsgrpc/`
  - Protobuf API (`events.proto`) and generated code. The single bidirectional stream carries all events.
  - Conversion helpers between domain `types.EventMessage` and gRPC `GrpcEventMessage` live in `workflowsgrpc/converters.go`.

- `communication/`
  - `GrpcCommunicator` encapsulates connection management, stream handling, reconnection (default 5s), and health checks (default 30s). It exposes a simple `WorkflowCommunicator` interface:
    - `SendEvent(*types.EventMessage) error`
    - `ReceiveEvents() <-chan *types.EventMessage`
    - `Close() error`
    - `IsConnected() bool`

- `dispatcher/`
  - A small worker-pool dispatcher that routes events to registered handlers by event name and executes them concurrently with bounded queue size.

- `correlation/`
  - Provides a `Router` for request/response correlation by ID. It stores per-request channels and delivers responses back to the awaiting goroutine. Used by `rpc`, `grpccache`, and `grpcstore`.

- `handlers/`
  - Registers handlers for core events: function requests/responses, cache/store responses, and discovery (server name/list functions). Receives messages from the `GrpcCommunicator` and dispatches them via the `dispatcher`.

- `grpccache/` and `grpcstore/`
  - Thin clients that send request events and await correlated responses via `correlation.Router`.

- `rpc/`
  - Sends function requests or flow node requests and awaits correlated responses.

- `basefunction/`
  - Generic typed function implementation with optional caching. `BaseFunctionDefinition` now gets server attribution injected via `SetServer` by the SDK.

- `sdk/`
  - High-level server lifecycle: environment loading, global state initialization, communicator setup, RPC client, cache/store clients, dispatcher setup, function registration, server registration broadcast, and activation of handlers.

- `types/`
  - `EventMessage`: language-agnostic event struct.
  - `events.go`: canonical event name constants used system-wide.
  - `keys.go`: `FunctionKey(server, name, version)` helper, ensuring consistent keys.

### Event Protocol

All communication happens over a single gRPC bidirectional stream with messages converted to/from `types.EventMessage`.

Key events (see `types/events.go`):
- Function: `function_request`, `function_response`
- Cache: `cache_get_request`, `cache_get_response`, `cache_set`, `cache_set_response`
- Store: `store_get_request`, `store_get_response`, `store_set_request`, `store_set_response`
- Discovery: `request_server_name`, `response_server_name`, `request_list_functions`, `response_list_functions`, `request_server_info`
- Misc: `status_message`, `error`, `client_registration`

### Global State

`state.GlobalState` holds:
- ServerName
- `WorkflowComm` (gRPC communicator)
- `RpcClient`, `GrpcCache`, `GrpcStore`
- `Functions` registry
- `Dispatcher`

### Porting to Other Languages

Port the following minimal abstractions:
1) Communicator: connect/reconnect loop, health checks, `SendEvent`, `ReceiveEvents`, `Close`, `IsConnected`.
2) Dispatcher: register handlers by event name; process via a worker pool.
3) Correlation Router: map correlation ID to a response channel/future; deliver responses.
4) Cache/Store/RPC clients: thin wrappers on top of the communicator + correlation router.
5) Function Builder: typed handlers; optional caching via a simple interface.

Keep `types.EventMessage` fields and event names identical across languages. Use the protobuf schema as the transport contract.


