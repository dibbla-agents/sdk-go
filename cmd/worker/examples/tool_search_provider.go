package examples

import (
	"log"
	"sort"
	"strings"

	sdk "github.com/dibbla-agents/sdk-go"
)

// DeterministicToolSearchProvider is an example custom tool_search capability
// provider (DIB-152). Its selection is intentionally dumb and deterministic so
// its effect is unmistakable in a run's wire capture and tools_activated
// trace: if the platform's built-in scorer ran instead, the ordering would
// differ immediately.
//
// Selection algorithm, over the offered stubs (name + description):
//  1. Stubs whose lowercased name starts with the lowercased query come first
//     (the "prefix matches"), in reverse-alphabetical order.
//  2. Then all remaining stubs, also reverse-alphabetical.
//  3. The combined list is truncated to topN.
//
// It sources nothing and dispatches nothing — it only orders the names the
// engine offered. Bind it on an agent node via:
//
//	"_capability_providers": { "tool_search": "deterministic-reverse" }
func DeterministicToolSearchProvider() sdk.ToolSearchProvider {
	return sdk.ToolSearchProvider{
		Name:        "deterministic-reverse",
		Description: "Example tool_search provider: prefix matches first, then reverse-alphabetical.",
		Version:     "1.0.0",
		Select: func(query string, stubs []sdk.ProviderStub, topN int) ([]string, error) {
			// Handy while developing a provider: log what the engine actually
			// sends so you can confirm the query and offered-tool shape before
			// trusting your ranking. The same call also shows up in the run-log
			// dock as a capability_provider_call entry.
			log.Printf("tool_search provider: query=%q offered=%d topN=%d", query, len(stubs), topN)

			q := strings.ToLower(strings.TrimSpace(query))

			var prefix, rest []string
			for _, s := range stubs {
				if q != "" && strings.HasPrefix(strings.ToLower(s.Name), q) {
					prefix = append(prefix, s.Name)
				} else {
					rest = append(rest, s.Name)
				}
			}

			// Reverse-alphabetical within each group (deliberately the
			// opposite of any relevance ranking).
			revAlpha := func(xs []string) {
				sort.Sort(sort.Reverse(sort.StringSlice(xs)))
			}
			revAlpha(prefix)
			revAlpha(rest)

			selected := append(prefix, rest...)
			if topN > 0 && len(selected) > topN {
				selected = selected[:topN]
			}
			return selected, nil
		},
	}
}
