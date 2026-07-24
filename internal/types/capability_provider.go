package types

import "encoding/json"

// Capability seats a provider can register for.
const (
	CapabilityToolSearch = "tool_search"
	CapabilityMemory     = "memory"
)

// CapabilityProviderDefinition is the wire format for registering a custom
// capability provider with the workflow server. It is sent as the JSON
// payload of a "response_list_capability_providers" event and must stay in
// sync with the workflow-server's types.CapabilityProviderDefinition.
type CapabilityProviderDefinition struct {
	Capability      string `json:"capability"` // the seat: "tool_search" | "memory"
	Name            string `json:"name"`
	Description     string `json:"description"`
	Version         string `json:"version"`
	Server          string `json:"server"`
	ContractVersion int    `json:"contract_version"`
	// Provider-declared extra ports (dynamic-ports slice of DIB-131);
	// carried from day one so the registration message never needs a
	// breaking change.
	ExtraInputsSchema  json.RawMessage `json:"extra_inputs_schema,omitempty"`
	ExtraOutputsSchema json.RawMessage `json:"extra_outputs_schema,omitempty"`
	// tool_search providers that want the function catalog pushed ahead of
	// run time (e.g. embedding-based scorers pre-indexing the pool).
	WantsCatalogSync bool `json:"wants_catalog_sync"`
}

// --- tool_search seat wire contract (DIB-152) -----------------------------
// These payload shapes are the request/response and catalog envelopes carried
// across the gRPC bridge for the tool_search selection seat. They must stay
// in sync with go-toolserver's function-layer producer.

// ProviderStub is the trimmed tool metadata a tool_search provider sees:
// name + description only. Deliberately NOT the full internal stub — no tool
// JSON, no MCP routes, nothing credentials-adjacent crosses the boundary.
type ProviderStub struct {
	// Name is the tool's unique identifier — exactly the string to return
	// from Select to activate it. Tools sourced from an MCP server carry an
	// "mcp__<server>__<tool>" prefix; all other tools use their plain
	// function name. Returning a Name that was not in the offered set is
	// dropped engine-side (with a trace warning), so only echo names you
	// received.
	Name string `json:"name"`
	// Description is the tool's full human-readable description — the same
	// text the agent's model sees for that tool. Use it for semantic or
	// keyword matching. May be empty for tools that declared no description.
	Description string `json:"description,omitempty"`
}

// CapabilityProviderRequest is the payload of a capability_provider_request
// event. For tool_search the provider ranks/filters Stubs for Query and
// returns an ordered subset (by name) capped at TopN.
type CapabilityProviderRequest struct {
	Capability string         `json:"capability"`
	Provider   string         `json:"provider"`
	Query      string         `json:"query"`
	Stubs      []ProviderStub `json:"stubs"`
	TopN       int            `json:"top_n"`
}

// CapabilityProviderResponse is the payload of a capability_provider_response
// event. Selected is the ordered subset of offered stub names; names outside
// the offered set are filtered engine-side with a trace warning. A non-empty
// Error (optionally with Code) marks a provider-side failure — the engine
// fails the node fail-fast, never falling back to the built-in.
type CapabilityProviderResponse struct {
	Selected []string `json:"selected,omitempty"`
	Error    string   `json:"error,omitempty"`
	Code     string   `json:"code,omitempty"`
}

// CapabilityCatalog is the payload of the one-way capability_catalog pre-sync
// event: the full offered stub set for a bound tool_search provider, pushed
// before the first query.
type CapabilityCatalog struct {
	Capability string         `json:"capability"`
	Provider   string         `json:"provider"`
	Stubs      []ProviderStub `json:"stubs"`
}
