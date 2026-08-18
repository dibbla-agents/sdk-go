package sdk

import (
	"encoding/json"
	"testing"

	"github.com/dibbla-agents/sdk-go/internal/types"
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

// The Select handler round-trips a request payload to a response payload:
// decode query+stubs+topN, run selection, encode the ordered names (DIB-152).
func TestToolSearchProvider_HandlerRoundTrip(t *testing.T) {
	p := ToolSearchProvider{
		Name: "rev",
		Select: func(query string, stubs []ProviderStub, topN int) ([]string, error) {
			out := make([]string, 0, len(stubs))
			for i := len(stubs) - 1; i >= 0; i-- {
				out = append(out, stubs[i].Name)
			}
			return out, nil
		},
	}
	h := p.handler()
	if h == nil {
		t.Fatal("expected a handler for a provider with Select set")
	}

	reqBytes, _ := json.Marshal(types.CapabilityProviderRequest{
		Capability: "tool_search",
		Provider:   "rev",
		Query:      "q",
		Stubs:      []ProviderStub{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		TopN:       5,
	})
	respBytes, err := h(&reqBytes)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var resp types.CapabilityProviderResponse
	if err := json.Unmarshal(*respBytes, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error in response: %s", resp.Error)
	}
	if len(resp.Selected) != 3 || resp.Selected[0] != "c" || resp.Selected[2] != "a" {
		t.Fatalf("expected [c b a], got %v", resp.Selected)
	}
}

// A Select handler returning an error is encoded into the response's Error
// field (fail-fast is enforced engine-side, not by dropping the reply).
func TestToolSearchProvider_HandlerError(t *testing.T) {
	p := ToolSearchProvider{
		Name: "boom",
		Select: func(query string, stubs []ProviderStub, topN int) ([]string, error) {
			return nil, errString("kaboom")
		},
	}
	reqBytes, _ := json.Marshal(types.CapabilityProviderRequest{Capability: "tool_search", Provider: "boom"})
	respBytes, err := p.handler()(&reqBytes)
	if err != nil {
		t.Fatalf("handler should encode the error into the payload, not return it: %v", err)
	}
	var resp types.CapabilityProviderResponse
	_ = json.Unmarshal(*respBytes, &resp)
	if resp.Error != "kaboom" {
		t.Fatalf("expected error 'kaboom' in response, got %q", resp.Error)
	}
}

// A provider without a Select handler declares no invoke handler (registered
// for binding only) and records nothing in the handler map.
func TestToolSearchProvider_NoSelectNoHandler(t *testing.T) {
	p := ToolSearchProvider{Name: "inert"}
	if p.handler() != nil {
		t.Fatal("expected nil handler when Select is unset")
	}
	s := &Server{}
	if err := s.RegisterCapabilityProvider(p); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, ok := s.capabilityHandlers["tool_search/inert"]; ok {
		t.Fatal("handler recorded for a provider without Select")
	}
}

// RegisterCapabilityProvider records the handler under "<capability>/<name>".
func TestRegisterCapabilityProvider_RecordsHandler(t *testing.T) {
	s := &Server{}
	err := s.RegisterCapabilityProvider(ToolSearchProvider{
		Name:   "rev",
		Select: func(string, []ProviderStub, int) ([]string, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, ok := s.capabilityHandlers["tool_search/rev"]; !ok {
		t.Fatalf("handler not recorded; keys=%v", s.capabilityHandlers)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
