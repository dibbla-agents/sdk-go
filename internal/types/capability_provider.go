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
