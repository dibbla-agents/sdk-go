package types

// Canonical event names used across the system.
// Keeping these centralized simplifies multi-language ports.
const (
	// Generic events
	EventError              = "error"
	EventStatusMessage      = "status_message"
	EventClientRegistration = "client_registration"

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
)
