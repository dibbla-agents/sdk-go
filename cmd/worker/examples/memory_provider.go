package examples

import (
	"log"

	sdk "github.com/dibbla-agents/sdk-go"
)

// MarkerMemoryMarker is the recognizable text a MarkerMemoryProvider prepends
// to the injected history. Its presence in a run's wire capture is proof the
// custom memory provider ran instead of a built-in history policy — if the
// built-in tiered/last-N replay had run, this string would never appear.
const MarkerMemoryMarker = "[marker-memory-provider] custom history injected"

// MarkerMemoryProvider is an example custom memory capability provider
// (DIB-154). Its transform is intentionally trivial and unmistakable so its
// effect is obvious in a run's wire capture:
//
//  1. It keeps only the single most recent stored turn (proving it, not the
//     built-in policy, chose what to inject).
//  2. It prepends one synthetic assistant turn whose text is MarkerMemoryMarker
//     (proving the injected history came from the provider).
//
// It is not a useful memory strategy — it is a visibility marker. A real
// provider would summarize, embed-rank, or budget-pack the turns. Bind it on an
// agent node with history_policy "custom" via:
//
//	"_capability_providers": { "memory": "marker-memory" }
func MarkerMemoryProvider() sdk.MemoryProvider {
	return sdk.MemoryProvider{
		Name:        "marker-memory",
		Description: "Example memory provider: injects a marker turn + the last stored turn.",
		Version:     "1.0.0",
		// Declare a history ceiling of half the model's context window (DIB-154):
		// the platform clamps this to its hard max and enforces it as the token
		// ceiling. Left at 0 it would use the platform default (0.75).
		MaxHistoryFraction: 0.5,
		Transform: func(currentMessage string, turns []sdk.Turn, tokenBudget int, meta sdk.ThreadMeta) ([]sdk.Turn, error) {
			// Handy while developing a provider: log what the engine actually
			// sends so you can confirm the incoming message, turn count, budget,
			// and thread before trusting your transform. The same call also shows
			// up in the run-log dock as a capability_provider_call entry.
			log.Printf("memory provider: thread=%q msg=%q turns_in=%d budget=%d",
				meta.ThreadID, currentMessage, len(turns), tokenBudget)

			marker := sdk.Turn{
				Role: "assistant",
				Parts: []sdk.Part{{
					Type: sdk.PartTypeText,
					Text: &sdk.TextPart{Text: MarkerMemoryMarker},
				}},
			}

			out := []sdk.Turn{marker}
			if len(turns) > 0 {
				// Keep the last stored turn verbatim (round-trips byte-for-byte).
				out = append(out, turns[len(turns)-1])
			}
			return out, nil
		},
	}
}
