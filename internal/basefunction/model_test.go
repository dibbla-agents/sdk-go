package basefunction

import (
	"reflect"
	"testing"
)

// Test structure with nested arrays for testing getTypeSchema
type TestInnerInnerStruct struct {
	InnerInnerStructValue string
}

type TestInnerStruct struct {
	InnerInnerStructAttribute []TestInnerInnerStruct
}

type TestStruct struct {
	InnerStruct []TestInnerStruct
	// Add a nested array field
	NestedArrayField [][]string
}

// Test structure that matches print_text_function.go
type PrintTextInnerInnerStruct []struct {
	InnerInnerStructValue string
}

type PrintTextInnerStruct struct {
	InnerInnerStructAttribute []PrintTextInnerInnerStruct
}

type PrintTextInputs struct {
	InnerStruct []PrintTextInnerStruct
}

func TestGetTypeSchema(t *testing.T) {
	// Get the type schema for our test structure
	schema := getTypeSchema(reflect.TypeOf(TestStruct{}))

	// Print the schema for debugging
	t.Logf("Schema: %+v", schema)

	// Test cases
	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "Nested struct in array",
			key:      "InnerStruct[].InnerInnerStructAttribute[].InnerInnerStructValue",
			expected: "string",
		},
		{
			name:     "Nested array",
			key:      "NestedArrayField[][]",
			expected: "string",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, exists := schema[tc.key]
			if !exists {
				t.Errorf("Expected key '%s' not found in schema. Schema keys: %v",
					tc.key, getKeys(schema))
			} else {
				t.Logf("Found key '%s' with value '%v'", tc.key, value)
				if value != tc.expected {
					t.Errorf("Expected value '%s' for key '%s', got '%v'",
						tc.expected, tc.key, value)
				}
			}
		})
	}

	// Test the PrintTextInputs structure
	t.Run("PrintTextInputs structure", func(t *testing.T) {
		printTextSchema := getTypeSchema(reflect.TypeOf(PrintTextInputs{}))
		t.Logf("PrintTextSchema: %+v", printTextSchema)

		// The expected key for the print text function structure
		expectedKey := "InnerStruct[].InnerInnerStructAttribute[].InnerInnerStructValue"
		value, exists := printTextSchema[expectedKey]

		if !exists {
			t.Errorf("Expected key '%s' not found in schema. Schema keys: %v",
				expectedKey, getKeys(printTextSchema))
		} else {
			t.Logf("Found key '%s' with value '%v'", expectedKey, value)
			if value != "string" {
				t.Errorf("Expected value 'string' for key '%s', got '%v'",
					expectedKey, value)
			}
		}
	})
}

// Helper function to get all keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
