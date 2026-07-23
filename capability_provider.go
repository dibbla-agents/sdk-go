package sdk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

// CapabilityProvider is a custom implementation of an agent capability seat
// (tool search scoring, memory policy, ...). Registered providers are
// announced to the workflow server alongside functions but live in a separate
// registry: they never appear in the function list and are only used when a
// workflow explicitly binds them to an agent node capability.
//
// Implemented by the typed per-seat structs ToolSearchProvider and
// MemoryProvider.
type CapabilityProvider interface {
	definition() types.CapabilityProviderDefinition
}

// ToolSearchProvider registers a custom tool_search implementation: it
// replaces the platform's built-in candidate scoring/selection for agent
// nodes that bind it. Providers own selection, never sourcing — the tool
// pool itself is still assembled by the platform.
//
// The scoring handler contract lands with the tool_search execution slice of
// DIB-131; registering the provider makes it visible for binding.
type ToolSearchProvider struct {
	Name        string
	Description string
	Version     string
	// WantsCatalogSync asks the server to push the function catalog ahead of
	// run time (e.g. embedding-based scorers pre-indexing the pool).
	WantsCatalogSync bool
	// ExtraInputsSchema / ExtraOutputsSchema optionally declare additional
	// node ports (JSON schema) surfaced when this provider is bound.
	ExtraInputsSchema  json.RawMessage
	ExtraOutputsSchema json.RawMessage
}

func (p ToolSearchProvider) definition() types.CapabilityProviderDefinition {
	return types.CapabilityProviderDefinition{
		Capability:         types.CapabilityToolSearch,
		Name:               p.Name,
		Description:        p.Description,
		Version:            p.Version,
		ContractVersion:    1,
		ExtraInputsSchema:  p.ExtraInputsSchema,
		ExtraOutputsSchema: p.ExtraOutputsSchema,
		WantsCatalogSync:   p.WantsCatalogSync,
	}
}

// MemoryProvider registers a custom memory policy implementation: it replaces
// the platform's built-in conversation replay policy for agent nodes that
// bind it (history_policy: custom). The platform keeps blob custody; the
// provider transforms turns-in to turns-out under a token budget.
//
// The transform handler contract lands with the memory execution slice of
// DIB-131; registering the provider makes it visible for binding.
type MemoryProvider struct {
	Name        string
	Description string
	Version     string
	// ExtraInputsSchema / ExtraOutputsSchema optionally declare additional
	// node ports (JSON schema) surfaced when this provider is bound.
	ExtraInputsSchema  json.RawMessage
	ExtraOutputsSchema json.RawMessage
}

func (p MemoryProvider) definition() types.CapabilityProviderDefinition {
	return types.CapabilityProviderDefinition{
		Capability:         types.CapabilityMemory,
		Name:               p.Name,
		Description:        p.Description,
		Version:            p.Version,
		ContractVersion:    1,
		ExtraInputsSchema:  p.ExtraInputsSchema,
		ExtraOutputsSchema: p.ExtraOutputsSchema,
	}
}

// RegisterCapabilityProvider adds a capability provider to the server. Like
// RegisterFunction, it must be called before Start(). Returns an error for
// definitions the workflow server would reject, so a misconfigured provider
// fails at startup instead of silently never appearing in the registry.
func (s *Server) RegisterCapabilityProvider(p CapabilityProvider) error {
	def := p.definition()
	if def.Name == "" {
		return fmt.Errorf("capability provider name must not be empty")
	}
	// ":" would break the server's org-prefixed registry key, "/" its
	// capability/name separator — the server skips such registrations.
	if strings.ContainsAny(def.Name, ":/") {
		return fmt.Errorf("capability provider name %q must not contain ':' or '/'", def.Name)
	}
	for _, existing := range s.capabilityProviders {
		if existing.Capability == def.Capability && existing.Name == def.Name {
			return fmt.Errorf("capability provider %s/%s already registered", def.Capability, def.Name)
		}
	}
	s.capabilityProviders = append(s.capabilityProviders, def)
	return nil
}
