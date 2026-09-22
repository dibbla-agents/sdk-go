package basefunction

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The schema a function publishes is the contract a caller codes against, and
// the only thing that reads the payload is json.Unmarshal into the input
// struct. So every key the schema advertises must be a key Unmarshal would
// actually read. For slices it was not: the schema said "kinds[]" while the
// decoder looked for "kinds", so a caller who followed the published contract
// passed validation and had their value silently dropped.
//
// Found 2026-09-21 against garden's list_documents (a kinds filter that never
// filtered) and remember (entity links never stored). Neither errored.
type arrayInput struct {
	Kinds  []string `json:"kinds,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

func TestSchemaKeysAreKeysTheDecoderReads(t *testing.T) {
	schema := getTypeSchema(reflect.TypeOf(arrayInput{}))

	for key := range schema {
		// Round-trip the published key through the real decoder. If the
		// decoder ignores it, the schema is advertising something that cannot
		// work.
		payload, err := json.Marshal(map[string]any{key: []string{"memory"}})
		if err != nil {
			t.Fatal(err)
		}
		var got arrayInput
		if err := json.Unmarshal(payload, &got); err != nil {
			continue // not a slice-shaped key; other tests cover those
		}
		if key == "kinds" && len(got.Kinds) == 0 {
			t.Errorf("schema publishes %q but the decoder read nothing into Kinds", key)
		}
		if key == "kinds[]" {
			t.Errorf("schema publishes %q, which json.Unmarshal will never read — "+
				"a caller following the contract loses the value silently", key)
		}
	}
}

func TestSliceFieldIsDeclaredUnderItsJSONName(t *testing.T) {
	schema := getTypeSchema(reflect.TypeOf(arrayInput{}))
	if _, ok := schema["kinds"]; !ok {
		t.Errorf("no %q in schema; got keys %v", "kinds", keysOf(schema))
	}
	if _, ok := schema["kinds[]"]; ok {
		t.Errorf("schema still carries the bracketed path %q as an input key", "kinds[]")
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
