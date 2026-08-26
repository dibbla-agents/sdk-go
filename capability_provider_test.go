package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	respBytes, err := h(context.Background(), &reqBytes)
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
	respBytes, err := p.handler()(context.Background(), &reqBytes)
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

// The Transform handler round-trips a memory request to a response: decode
// turns+budget+meta, run the transform, encode the returned turns (DIB-154).
func TestMemoryProvider_HandlerRoundTrip(t *testing.T) {
	p := MemoryProvider{
		Name: "marker",
		Transform: func(_ context.Context, currentMessage string, turns []Turn, tokenBudget int, meta ThreadMeta) ([]Turn, error) {
			marker := Turn{Role: "assistant", Parts: []Part{{Type: PartTypeText, Text: &TextPart{Text: "[marker:" + currentMessage + "]"}}}}
			out := []Turn{marker}
			if len(turns) > 0 {
				out = append(out, turns[len(turns)-1])
			}
			return out, nil
		},
	}
	h := p.handler()
	if h == nil {
		t.Fatal("expected a handler for a provider with Transform set")
	}

	reqBytes, _ := json.Marshal(types.MemoryTransformRequest{
		Capability:     "memory",
		Provider:       "marker",
		CurrentMessage: "q",
		TokenBudget:    60000,
		ThreadMeta:     types.ThreadMeta{ThreadID: "t1", TurnCount: 2},
		Turns: []Turn{
			{Role: "user", Parts: []Part{{Type: PartTypeText, Text: &TextPart{Text: "one"}}}},
			{Role: "assistant", Parts: []Part{{Type: PartTypeText, Text: &TextPart{Text: "two"}}}},
		},
	})
	respBytes, err := h(context.Background(), &reqBytes)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var resp types.MemoryTransformResponse
	if err := json.Unmarshal(*respBytes, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error in response: %s", resp.Error)
	}
	if len(resp.Turns) != 2 || resp.Turns[0].Parts[0].Text.Text != "[marker:q]" || resp.Turns[1].Parts[0].Text.Text != "two" {
		t.Fatalf("returned turns = %+v, want [marker:q, two]", resp.Turns)
	}
}

// A Transform handler returning an error is encoded into the response's Error
// field (fail-fast is enforced engine-side, not by dropping the reply).
func TestMemoryProvider_HandlerError(t *testing.T) {
	p := MemoryProvider{
		Name: "boom",
		Transform: func(context.Context, string, []Turn, int, ThreadMeta) ([]Turn, error) {
			return nil, errString("kaboom")
		},
	}
	reqBytes, _ := json.Marshal(types.MemoryTransformRequest{Capability: "memory", Provider: "boom"})
	respBytes, err := p.handler()(context.Background(), &reqBytes)
	if err != nil {
		t.Fatalf("handler should encode the error into the payload, not return it: %v", err)
	}
	var resp types.MemoryTransformResponse
	_ = json.Unmarshal(*respBytes, &resp)
	if resp.Error != "kaboom" {
		t.Fatalf("expected error 'kaboom' in response, got %q", resp.Error)
	}
}

// A memory provider without a Transform handler declares no invoke handler
// (registered for binding only) and records nothing in the handler map.
func TestMemoryProvider_NoTransformNoHandler(t *testing.T) {
	p := MemoryProvider{Name: "inert"}
	if p.handler() != nil {
		t.Fatal("expected nil handler when Transform is unset")
	}
	s := &Server{}
	if err := s.RegisterCapabilityProvider(p); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, ok := s.capabilityHandlers["memory/inert"]; ok {
		t.Fatal("handler recorded for a provider without Transform")
	}
}

// RegisterCapabilityProvider records a memory Transform handler under
// "<capability>/<name>".
func TestRegisterCapabilityProvider_RecordsMemoryHandler(t *testing.T) {
	s := &Server{}
	err := s.RegisterCapabilityProvider(MemoryProvider{
		Name:      "marker",
		Transform: func(context.Context, string, []Turn, int, ThreadMeta) ([]Turn, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, ok := s.capabilityHandlers["memory/marker"]; !ok {
		t.Fatalf("handler not recorded; keys=%v", s.capabilityHandlers)
	}
}

// TestMemoryTurnWireFormat pins the v2 Turn/Part JSON keys the memory seat
// exchanges with go-toolserver's agenttypes model — a drift here silently
// corrupts round-tripped history.
func TestMemoryTurnWireFormat(t *testing.T) {
	turn := Turn{
		ID:   "id1",
		Role: "assistant",
		Parts: []Part{
			{Type: PartTypeText, Text: &TextPart{Text: "hi"}},
			{Type: PartTypeToolCall, ToolCall: &ToolCallPart{ToolName: "search", ProviderToolID: "toolu_1"}},
		},
	}
	raw, _ := json.Marshal(turn)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["role"] != "assistant" || m["id"] != "id1" {
		t.Errorf("turn keys drifted: %v", m)
	}
	parts, ok := m["parts"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("parts = %v, want 2 entries", m["parts"])
	}
	p0 := parts[0].(map[string]any)
	if p0["type"] != "text" || p0["text"].(map[string]any)["text"] != "hi" {
		t.Errorf("text part drifted: %v", p0)
	}
	p1 := parts[1].(map[string]any)
	tc := p1["tool_call"].(map[string]any)
	if p1["type"] != "tool_call" || tc["tool_name"] != "search" || tc["provider_tool_id"] != "toolu_1" {
		t.Errorf("tool_call part drifted: %v", p1)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// The context handed to Transform is the one the engine's abandon notice
// cancels (DIB-443): a handler that watches ctx observes the cancellation and
// can abort its work (e.g. a store write) instead of completing it blind.
func TestMemoryProvider_TransformObservesCancellation(t *testing.T) {
	observed := make(chan error, 1)
	p := MemoryProvider{
		Name: "ctx-probe",
		Transform: func(ctx context.Context, _ string, _ []Turn, _ int, _ ThreadMeta) ([]Turn, error) {
			<-ctx.Done()
			observed <- ctx.Err()
			return nil, ctx.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	reqBytes, _ := json.Marshal(types.MemoryTransformRequest{Capability: "memory", Provider: "ctx-probe"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		respBytes, err := p.handler()(ctx, &reqBytes)
		if err != nil {
			t.Errorf("handler should encode the error into the payload, not return it: %v", err)
			return
		}
		var resp types.MemoryTransformResponse
		if json.Unmarshal(*respBytes, &resp) != nil || resp.Error == "" {
			t.Errorf("cancelled transform should surface an error in the response payload")
		}
	}()

	cancel()
	select {
	case err := <-observed:
		if err != context.Canceled {
			t.Fatalf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-timeoutAfter(t):
		t.Fatal("Transform never observed the cancellation")
	}
	<-done
}

// timeoutAfter gives blocking assertions a bounded wait without importing time
// at every call site.
func timeoutAfter(t *testing.T) <-chan time.Time {
	t.Helper()
	return time.After(2 * time.Second)
}
