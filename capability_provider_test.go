package sdk

import (
	"encoding/json"
	"testing"
)

func TestRegisterCapabilityProviderValidation(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	if err := s.RegisterCapabilityProvider(ToolSearchProvider{Name: "acme-scorer", Version: "1.0", WantsCatalogSync: true}); err != nil {
		t.Fatalf("valid tool_search provider rejected: %v", err)
	}
	if err := s.RegisterCapabilityProvider(MemoryProvider{Name: "acme-vector", Version: "1.0"}); err != nil {
		t.Fatalf("valid memory provider rejected: %v", err)
	}

	// The workflow server skips names carrying its key delimiters; the SDK
	// must fail fast at registration instead.
	if err := s.RegisterCapabilityProvider(ToolSearchProvider{Name: "bad:name"}); err == nil {
		t.Error("name with ':' accepted, want error")
	}
	if err := s.RegisterCapabilityProvider(ToolSearchProvider{Name: "bad/name"}); err == nil {
		t.Error("name with '/' accepted, want error")
	}
	if err := s.RegisterCapabilityProvider(ToolSearchProvider{Name: ""}); err == nil {
		t.Error("empty name accepted, want error")
	}
	// Same seat + name twice → duplicate.
	if err := s.RegisterCapabilityProvider(ToolSearchProvider{Name: "acme-scorer"}); err == nil {
		t.Error("duplicate provider accepted, want error")
	}
	// Same name on a different seat is fine.
	if err := s.RegisterCapabilityProvider(MemoryProvider{Name: "acme-scorer"}); err != nil {
		t.Errorf("same name on different capability rejected: %v", err)
	}

	if len(s.capabilityProviders) != 3 {
		t.Errorf("registered %d providers, want 3", len(s.capabilityProviders))
	}
}

// TestCapabilityProviderWireFormat pins the registration payload's JSON keys
// to the workflow-server contract (types.CapabilityProviderDefinition there).
func TestCapabilityProviderWireFormat(t *testing.T) {
	def := ToolSearchProvider{
		Name:              "acme-scorer",
		Description:       "d",
		Version:           "1.2",
		WantsCatalogSync:  true,
		ExtraInputsSchema: json.RawMessage(`{"type":"object"}`),
	}.definition()
	def.Server = "worker-1"

	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for key, want := range map[string]any{
		"capability":         "tool_search",
		"name":               "acme-scorer",
		"description":        "d",
		"version":            "1.2",
		"server":             "worker-1",
		"contract_version":   float64(1),
		"wants_catalog_sync": true,
	} {
		if got := m[key]; got != want {
			t.Errorf("wire key %q = %v, want %v", key, got, want)
		}
	}
	if _, ok := m["extra_inputs_schema"]; !ok {
		t.Error("extra_inputs_schema missing from wire payload")
	}
	if _, ok := m["extra_outputs_schema"]; ok {
		t.Error("unset extra_outputs_schema serialized, want omitted")
	}

	memDef := MemoryProvider{Name: "m"}.definition()
	if memDef.Capability != "memory" {
		t.Errorf("memory seat = %q, want %q", memDef.Capability, "memory")
	}
	if memDef.ContractVersion != 1 {
		t.Errorf("memory contract_version = %d, want 1", memDef.ContractVersion)
	}
}
