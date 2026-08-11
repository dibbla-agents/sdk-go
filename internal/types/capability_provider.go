package types

import (
	"encoding/json"
	"time"
)

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
	// MaxHistoryFraction (DIB-154, memory seat) is the provider-declared share
	// of the model's context window its returned history may occupy. The engine
	// clamps it to a hard platform maximum and enforces it; 0/unset = default.
	MaxHistoryFraction float64 `json:"max_history_fraction,omitempty"`
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

// --- memory seat wire contract (DIB-154) ----------------------------------
// A custom memory provider transforms the stored conversation turns into the
// turns injected into the model's context, under a token budget. These shapes
// mirror go-toolserver's v2 ChatBlob Turn/Part model exactly (same JSON tags)
// so turns round-trip losslessly across the boundary — a provider that returns
// a turn unchanged returns it byte-for-byte. Keep in sync with
// go-toolserver's agents/agenttypes.
//
// NOTE: unlike tool_search (name + description only), the memory seat carries
// full conversation content — user/assistant text, tool args AND results,
// reasoning. Binding a memory provider sends that history to the provider's
// server (the same org-trust boundary as any worker function, just a larger
// payload). The platform keeps blob custody: returned turns are injection-only
// and never written back, so a provider cannot corrupt the stored history.

// PartType is the discriminant of a Part.
type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeToolCall   PartType = "tool_call"
	PartTypeAttachment PartType = "attachment"
	PartTypeReasoning  PartType = "reasoning"
)

// TextPart carries plain text produced by the user or the assistant.
type TextPart struct {
	Text string `json:"text"`
}

// ToolCallPart is self-contained: it carries both the call and its result.
type ToolCallPart struct {
	ToolName       string          `json:"tool_name"`
	Args           json.RawMessage `json:"args,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	Error          string          `json:"error,omitempty"`
	DurationMs     int64           `json:"duration_ms,omitempty"`
	ProviderToolID string          `json:"provider_tool_id,omitempty"`
}

// AttachmentPart stores a file reference only; bytes are never carried.
type AttachmentPart struct {
	FileHash    string `json:"file_hash"`
	Name        string `json:"name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

// ReasoningPart carries a provider-specific reasoning block round-tripped
// verbatim (Claude thinking blocks, Gemini thoughtSignature). Preserve it
// unchanged unless you deliberately intend to drop reasoning continuity.
type ReasoningPart struct {
	Provider      string          `json:"provider,omitempty"`
	OpaquePayload json.RawMessage `json:"opaque_payload,omitempty"`
	Summary       string          `json:"summary,omitempty"`
}

// Part is a discriminated union inside Turn.Parts; exactly one payload pointer
// is set, matching Type.
type Part struct {
	Type       PartType        `json:"type"`
	Text       *TextPart       `json:"text,omitempty"`
	ToolCall   *ToolCallPart   `json:"tool_call,omitempty"`
	Attachment *AttachmentPart `json:"attachment,omitempty"`
	Reasoning  *ReasoningPart  `json:"reasoning,omitempty"`
}

// Turn is one user or assistant exchange in a v2 chat thread. Role is "user"
// or "assistant"; the engine rejects a returned turn set with any other role.
type Turn struct {
	ID                 string    `json:"id"`
	Role               string    `json:"role"`
	Date               time.Time `json:"date"`
	RunID              string    `json:"run_id,omitempty"`
	CorrelationID      string    `json:"correlation_id,omitempty"`
	ProviderResponseID string    `json:"provider_response_id,omitempty"`
	Parts              []Part    `json:"parts"`
}

// ThreadMeta carries lightweight context about the thread being transformed.
type ThreadMeta struct {
	ThreadID  string `json:"thread_id"`
	TurnCount int    `json:"turn_count"`
}

// MemoryTransformRequest is the payload of a capability_provider_request for
// the memory seat: the stored turns, the incoming user message, and the token
// ceiling the returned turns must stay under. CurrentMessage is the message
// being answered this round — provided so a provider can do query-conditioned
// retrieval; it is not one of Turns and the engine always appends the real user
// message after the provider's history.
//
// TokenBudget is an ENFORCED safety ceiling (a generous share of the model's
// context window, leaving headroom for the system prompt, tools, message, and
// response), not a strategy the platform imposes — you own how you fill 0..budget.
// The engine does NOT truncate your output; instead it REJECTS a response whose
// estimated tokens exceed the ceiling (or whose raw size exceeds an absolute
// byte cap), failing the node with a coded error. Stay under it.
type MemoryTransformRequest struct {
	Capability     string     `json:"capability"`
	Provider       string     `json:"provider"`
	Turns          []Turn     `json:"turns"`
	CurrentMessage string     `json:"current_message"`
	TokenBudget    int        `json:"token_budget"`
	ThreadMeta     ThreadMeta `json:"thread_meta"`
}

// MemoryTransformResponse is the payload of the capability_provider_response
// for the memory seat: the turns to inject (validated engine-side to be v2
// ChatBlob shape). A non-empty Error (optionally with Code) fails the node
// fail-fast, with no fallback to a built-in policy.
type MemoryTransformResponse struct {
	Turns []Turn `json:"turns,omitempty"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}
