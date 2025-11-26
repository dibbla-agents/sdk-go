package types

// Canonical event names used across the system.
// Keeping these centralized simplifies multi-language ports.
const (
	// Generic events
	EventError              = "error"
	EventStatusMessage      = "status_message"
	EventClientRegistration = "client_registration"

	// Ping/Pong - Keep-alive mechanism between SDK client and workflow server
	//
	// PING (sent by SDK client every 30 seconds by default):
	//   - event: "ping"
	//   - server: "<client_server_name>"  (e.g., "codex-go-worker")
	//   - text: "ping"
	//   - correlation_id: "" (empty)
	//   - All other fields empty (function, version, node, workflow, run, meta, payload)
	//
	// PONG (expected response from workflow server):
	//   - event: "pong"
	//   - server: "<workflow_server_name>"
	//   - text: "pong"
	//   - correlation_id: "" (empty, or echo the ping's if provided)
	//
	// The SDK client does not currently require or process pong responses,
	// but the workflow server should send them for protocol completeness
	// and potential future use (e.g., latency measurement, connection validation).
	EventPing = "ping"
	EventPong = "pong"

	// Function invocation
	EventFunctionRequest  = "function_request"
	EventFunctionResponse = "function_response"

	// Flow invocation
	EventFlowNodeRequest = "flow_node_request"

	// Cache events
	EventCacheGetRequest  = "cache_get_request"
	EventCacheGetResponse = "cache_get_response"
	EventCacheSet         = "cache_set"
	EventCacheSetResponse = "cache_set_response"

	// Store events
	EventStoreGetRequest  = "store_get_request"
	EventStoreGetResponse = "store_get_response"
	EventStoreSetRequest  = "store_set_request"
	EventStoreSetResponse = "store_set_response"

	// Server discovery/listing
	EventRequestListFunctions  = "request_list_functions"
	EventResponseListFunctions = "response_list_functions"
	EventRequestServerName     = "request_server_name"
	EventResponseServerName    = "response_server_name"
	EventRequestServerInfo     = "request_server_info"

	// OAuth events - Request access tokens for external services (Google, Microsoft, GitHub)
	EventOAuthTokenRequest   = "oauth_token_request"
	EventOAuthTokenResponse  = "oauth_token_response"
	EventOAuthStatusRequest  = "oauth_status_request"
	EventOAuthStatusResponse = "oauth_status_response"
	EventOAuthError          = "oauth_error"
)
