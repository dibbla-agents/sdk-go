package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dibbla-agents/sdk-go/internal/state"
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

// ProviderStub is the trimmed tool metadata a tool_search Select handler
// receives: name + description only. The platform deliberately does not send
// the full tool schema, MCP routes, or anything credentials-adjacent across
// the provider boundary — a provider selects from names, it never sources or
// dispatches tools.
type ProviderStub = types.ProviderStub

// Memory-seat turn/part shapes a MemoryProvider.Transform handler works with
// (DIB-154). They mirror the platform's v2 ChatBlob model exactly, so a turn
// returned unchanged round-trips byte-for-byte. Construct a text part with
// Part{Type: PartTypeText, Text: &TextPart{Text: "..."}}.
type (
	Turn           = types.Turn
	Part           = types.Part
	PartType       = types.PartType
	TextPart       = types.TextPart
	ToolCallPart   = types.ToolCallPart
	AttachmentPart = types.AttachmentPart
	ReasoningPart  = types.ReasoningPart
	ThreadMeta     = types.ThreadMeta
)

// Part type discriminants, re-exported for handler ergonomics.
const (
	PartTypeText       = types.PartTypeText
	PartTypeToolCall   = types.PartTypeToolCall
	PartTypeAttachment = types.PartTypeAttachment
	PartTypeReasoning  = types.PartTypeReasoning
)

// ToolSearchProvider registers a custom tool_search implementation: it
// replaces the platform's built-in candidate selection for agent nodes that
// bind it. Providers own selection, never sourcing — the tool pool itself is
// still assembled by the platform, and any name a provider returns that is
// not in the offered set is dropped engine-side.
type ToolSearchProvider struct {
	Name        string
	Description string
	Version     string
	// Select is the selection handler (DIB-152). The engine calls it once per
	// tool_search invocation with:
	//
	//   query  — the raw search string the agent's model passed to tool_search,
	//            verbatim (not normalized or truncated).
	//   stubs  — the tools offered for this call: name + description only, drawn
	//            from the node's own org-scoped pool. This IS the candidate set;
	//            you select from it, you never source new tools.
	//   topN   — the maximum number of names to return. The engine also hard-caps
	//            the result at topN, so returning more is harmless (extras are cut).
	//
	// Return the ordered subset of stub names to activate (order is the ranking).
	// The method is unconstrained: keyword scoring, embeddings, an LLM re-rank, or
	// policy rules all fit — "selection", not "scoring". Returning an empty slice
	// is valid (activate nothing). A returned Name that was not in stubs is dropped
	// engine-side with a trace warning. Returning an error fails the node fail-fast
	// (no fallback to the built-in scorer). Leave nil to register the provider for
	// binding without a live handler yet.
	//
	// To see exactly what arrives at runtime, log inside this closure and run the
	// worker locally (e.g. `log.Printf("tool_search q=%q offered=%d", query, len(stubs))`);
	// the same call is also visible in the run-log dock as a capability_provider_call
	// entry. Select is a pure function, so it is straightforward to unit-test in
	// isolation with fabricated stubs.
	Select func(query string, stubs []ProviderStub, topN int) ([]string, error)
	// SelectFull is the struct-shaped variant of Select (DIB-449). It receives
	// the full request — including ExtraInputs, the resolved values of any
	// declared extra input ports — and may return ExtraOutputs for declared
	// extra output ports alongside the selection. When both SelectFull and
	// Select are set, SelectFull wins. Prefer it for new providers; Select
	// stays supported unchanged.
	SelectFull func(ctx context.Context, req SelectRequest) (SelectResponse, error)
	// WantsCatalogSync asks the server to push the function catalog ahead of
	// run time (e.g. embedding-based scorers pre-indexing the pool).
	WantsCatalogSync bool
	// ExtraInputsSchema / ExtraOutputsSchema optionally declare additional
	// node ports (JSON schema object with a top-level "properties" map;
	// property names become port names, and a "required" list marks ports
	// that gate execution). Surfaced as wireable ports on the capability
	// node when this provider is bound (DIB-449). Values arrive in
	// SelectRequest.ExtraInputs; output values are returned in
	// SelectResponse.ExtraOutputs and published with the owning agent
	// node's outputs.
	ExtraInputsSchema  json.RawMessage
	ExtraOutputsSchema json.RawMessage
}

// SelectRequest bundles everything the engine sends for one tool_search call
// (DIB-449). Query/Stubs/TopN carry exactly what the positional Select handler
// receives; ExtraInputs holds the resolved values of the provider's declared
// extra input ports, keyed by port name (absent ports are simply missing keys).
// ExtraInputs values are caller-wired and unauthenticated.
type SelectRequest struct {
	Query       string
	Stubs       []ProviderStub
	TopN        int
	ExtraInputs map[string]any
}

// SelectResponse is what a SelectFull handler returns: the ordered subset of
// offered stub names, plus optional ExtraOutputs values for the provider's
// declared extra output ports. Keys not present in ExtraOutputsSchema are
// dropped engine-side with a trace warning.
type SelectResponse struct {
	Selected     []string
	ExtraOutputs map[string]any
}

// handler builds the generic invoke closure the dispatcher calls for a
// capability_provider_request targeting this provider. Returns nil when no
// Select handler is declared, so the dispatcher can reply "not supported".
func (p ToolSearchProvider) handler() state.CapabilityProviderHandler {
	// Normalize to the struct-shaped form once, so there is exactly one call
	// path: a positional Select becomes a SelectFull that ignores extra ports.
	full := p.SelectFull
	if full == nil {
		if p.Select == nil {
			return nil
		}
		full = func(_ context.Context, req SelectRequest) (SelectResponse, error) {
			selected, err := p.Select(req.Query, req.Stubs, req.TopN)
			return SelectResponse{Selected: selected}, err
		}
	}
	return func(ctx context.Context, reqPayload *[]byte) (*[]byte, error) {
		var req types.CapabilityProviderRequest
		if reqPayload != nil {
			if err := json.Unmarshal(*reqPayload, &req); err != nil {
				return nil, fmt.Errorf("tool_search provider request decode failed: %w", err)
			}
		}
		var resp types.CapabilityProviderResponse
		result, err := full(ctx, SelectRequest{
			Query:       req.Query,
			Stubs:       req.Stubs,
			TopN:        req.TopN,
			ExtraInputs: req.ExtraInputs,
		})
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Selected = result.Selected
			resp.ExtraOutputs = result.ExtraOutputs
		}
		out, mErr := json.Marshal(resp)
		if mErr != nil {
			return nil, fmt.Errorf("tool_search provider response encode failed: %w", mErr)
		}
		return &out, nil
	}
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
// the platform's built-in conversation replay policy for agent nodes that bind
// it (history_policy: custom). The platform keeps blob custody; the provider
// transforms turns-in to turns-out under a token budget.
//
// Data boundary: unlike tool_search, the memory seat carries FULL conversation
// content — user/assistant text, tool args and results, reasoning — to the
// provider's server. Binding a memory provider therefore sends a thread's
// history off-platform to that (org-owned) server, the same trust boundary as
// any worker function but a larger payload. The platform keeps blob custody:
// the turns you return are injection-only for that one run and are never
// written back, so a provider cannot corrupt or persist over the stored blob.
type MemoryProvider struct {
	Name        string
	Description string
	Version     string
	// Transform is the memory handler (DIB-154). The engine calls it once per
	// run, before the model turn, with:
	//
	//   currentMessage — the user message being answered this round. Provided so
	//                    you can do query-conditioned retrieval (select the
	//                    history relevant to what was just asked). It is NOT one
	//                    of turns, and you do NOT return it — the engine always
	//                    appends the real user message after your history.
	//   turns          — the thread's stored turns in chronological order, full
	//                    v2 detail. This is the entire history the platform
	//                    holds; you select/transform, you do not fetch more.
	//   tokenBudget    — an ENFORCED token ceiling your returned history must stay
	//                    under (a generous share of the model's context window,
	//                    leaving room for the system prompt, tools, message, and
	//                    response). It is NOT a strategy the platform imposes — you
	//                    decide how to fill 0..budget. The engine does not truncate
	//                    your output; it REJECTS a return that exceeds the ceiling
	//                    (or an absolute byte cap), failing the node. Stay under it.
	//   meta           — thread id and turn count.
	//
	// Return the turns to inject, in order. You may drop, reorder, summarize, or
	// prepend turns freely; every returned turn must carry a "user"/"assistant"
	// role and parts with known types, or the engine fails the node. Returning
	// an empty slice injects no history. Returning an error — or a return that
	// exceeds the token/byte guardrails — fails the node fail-fast; there is no
	// built-in fallback for history_policy: custom. Leave nil to register the
	// provider for binding without a live handler yet (a call then replies with
	// an explicit "not supported" error).
	//
	// ctx is cancelled when the engine abandons the call (DIB-443): the
	// platform's hard per-call budget (~15 s) expired, or the run was
	// terminated. Once ctx fires, no one will read your return value — stop
	// work and, above all, do not commit side effects (a store write after a
	// timeout lands in a thread the platform already failed). Handlers that
	// ignore ctx keep working exactly as before, they just waste the effort.
	Transform func(ctx context.Context, currentMessage string, turns []Turn, tokenBudget int, meta ThreadMeta) ([]Turn, error)
	// TransformFull is the struct-shaped variant of Transform (DIB-449). It
	// receives the full request — including ExtraInputs, the resolved values
	// of any declared extra input ports — and may return ExtraOutputs for
	// declared extra output ports alongside the turns. When both TransformFull
	// and Transform are set, TransformFull wins. Prefer it for new providers;
	// Transform stays supported unchanged. The same ctx-cancellation contract
	// as Transform applies.
	TransformFull func(ctx context.Context, req TransformRequest) (TransformResponse, error)
	// MaxHistoryFraction optionally declares the share of the model's context
	// window your returned history may occupy (DIB-154), e.g. 0.5 for half.
	// The platform clamps it to a hard maximum (you may tune down freely, up
	// only to that cap) and enforces the resulting token ceiling — the
	// tokenBudget passed to Transform already reflects your clamped choice. Leave
	// 0 to accept the platform default. This tunes the token ceiling ONLY; it
	// never lifts the absolute byte cap, which is platform-owned and non-tunable.
	MaxHistoryFraction float64
	// ExtraInputsSchema / ExtraOutputsSchema optionally declare additional
	// node ports (JSON schema object with a top-level "properties" map;
	// property names become port names, and a "required" list marks ports
	// that gate execution). Surfaced as wireable ports on the capability
	// node when this provider is bound (DIB-449). Values arrive in
	// TransformRequest.ExtraInputs; output values are returned in
	// TransformResponse.ExtraOutputs and published with the owning agent
	// node's outputs.
	ExtraInputsSchema  json.RawMessage
	ExtraOutputsSchema json.RawMessage
}

// TransformRequest bundles everything the engine sends for one memory
// transform call (DIB-449). CurrentMessage/Turns/TokenBudget/Meta carry
// exactly what the positional Transform handler receives; ExtraInputs holds
// the resolved values of the provider's declared extra input ports, keyed by
// port name. ExtraInputs values are caller-wired and unauthenticated —
// identity stays in Meta (engine-asserted), never in a port.
type TransformRequest struct {
	CurrentMessage string
	Turns          []Turn
	TokenBudget    int
	Meta           ThreadMeta
	ExtraInputs    map[string]any
}

// TransformResponse is what a TransformFull handler returns: the turns to
// inject (same contract as Transform's return), plus optional ExtraOutputs
// values for the provider's declared extra output ports. Keys not present in
// ExtraOutputsSchema are dropped engine-side with a trace warning.
type TransformResponse struct {
	Turns        []Turn
	ExtraOutputs map[string]any
}

// handler builds the generic invoke closure the dispatcher calls for a
// capability_provider_request targeting this memory provider. Returns nil when
// no Transform handler is declared, so the dispatcher can reply "not supported".
func (p MemoryProvider) handler() state.CapabilityProviderHandler {
	// Normalize to the struct-shaped form once, so there is exactly one call
	// path: a positional Transform becomes a TransformFull that ignores
	// extra ports.
	full := p.TransformFull
	if full == nil {
		if p.Transform == nil {
			return nil
		}
		full = func(ctx context.Context, req TransformRequest) (TransformResponse, error) {
			turns, err := p.Transform(ctx, req.CurrentMessage, req.Turns, req.TokenBudget, req.Meta)
			return TransformResponse{Turns: turns}, err
		}
	}
	return func(ctx context.Context, reqPayload *[]byte) (*[]byte, error) {
		var req types.MemoryTransformRequest
		if reqPayload != nil {
			if err := json.Unmarshal(*reqPayload, &req); err != nil {
				return nil, fmt.Errorf("memory provider request decode failed: %w", err)
			}
		}
		var resp types.MemoryTransformResponse
		result, err := full(ctx, TransformRequest{
			CurrentMessage: req.CurrentMessage,
			Turns:          req.Turns,
			TokenBudget:    req.TokenBudget,
			Meta:           req.ThreadMeta,
			ExtraInputs:    req.ExtraInputs,
		})
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Turns = result.Turns
			resp.ExtraOutputs = result.ExtraOutputs
		}
		out, mErr := json.Marshal(resp)
		if mErr != nil {
			return nil, fmt.Errorf("memory provider response encode failed: %w", mErr)
		}
		return &out, nil
	}
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
		MaxHistoryFraction: p.MaxHistoryFraction,
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
	// Declared extra ports (DIB-449) only work through the struct-shaped
	// handlers: a positional Select/Transform can neither receive extra
	// inputs nor return extra outputs, so a provider declaring schemas while
	// implementing only the positional handler would have its wired inputs
	// silently discarded and its output ports never produce — fail at
	// startup instead. (No handler at all stays allowed: registered for
	// binding, calls reply "not supported".)
	declaresPorts := len(def.ExtraInputsSchema) > 0 || len(def.ExtraOutputsSchema) > 0
	if declaresPorts {
		switch sp := p.(type) {
		case ToolSearchProvider:
			if sp.Select != nil && sp.SelectFull == nil {
				return fmt.Errorf("capability provider %s/%s declares extra ports but implements only the positional Select — implement SelectFull", def.Capability, def.Name)
			}
		case MemoryProvider:
			if sp.Transform != nil && sp.TransformFull == nil {
				return fmt.Errorf("capability provider %s/%s declares extra ports but implements only the positional Transform — implement TransformFull", def.Capability, def.Name)
			}
		}
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

	// Register the seat's invoke handler, if the provider declares one. The
	// key matches the request routing: "<capability>/<name>". Providers
	// registered without a live handler (Select nil) still appear for
	// binding; a call to them replies with an explicit "not supported" error.
	if hp, ok := p.(capabilityHandlerProvider); ok {
		if h := hp.handler(); h != nil {
			if s.capabilityHandlers == nil {
				s.capabilityHandlers = make(map[string]state.CapabilityProviderHandler)
			}
			s.capabilityHandlers[def.Capability+"/"+def.Name] = h
		}
	}
	return nil
}

// capabilityHandlerProvider is implemented by seat structs that carry a live
// invoke handler: ToolSearchProvider.Select and MemoryProvider.Transform. A
// provider registered with a nil handler still appears for binding; a call to
// it replies with an explicit "not supported" error.
type capabilityHandlerProvider interface {
	handler() state.CapabilityProviderHandler
}
