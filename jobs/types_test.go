package jobs

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSchemaKeepsTheOriginalKeysAndAddsOnlyWhatIsSet(t *testing.T) {
	plain := JobParameter{Name: "space", Type: "string", Required: true}
	if got, want := plain.Schema(), map[string]interface{}{"type": "string", "required": true, "default": nil}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plain parameter: got %v, want %v", got, want)
	}

	full := JobParameter{
		Name: "limit", Type: "number", Default: 10,
		Description: "Pages per run", Options: []interface{}{10, 50}, Format: FormatList,
		Min: Bound(1), Max: Bound(100),
	}
	got := full.Schema()
	want := map[string]interface{}{
		"type": "number", "required": false, "default": 10,
		"description": "Pages per run", "options": []interface{}{10, 50}, "format": "list",
		"min": 1.0, "max": 100.0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full parameter: got %v, want %v", got, want)
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("schema must marshal: %v", err)
	}
}
