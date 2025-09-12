package basefunction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/hash"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"log"
	"os"
	"reflect"
	"strings"
	"time"
)

type FunctionInterface interface {
	GetName() string
	GetVersion() string
	GetFunctionDefinition() FunctionDefinition
	Execute(inputs *[]byte, eventState *types.EventMessage) (*[]byte, error)
	Serialize() ([]byte, error)
}

type FunctionDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	InputsType  string   `json:"inputs_type"`
	OutputsType string   `json:"outputs_type"`
	Server      string   `json:"server"`
	Tags        []string `json:"tags"`
}

type BaseFunctionDefinition struct {
	FunctionDefinition
	InputType  reflect.Type
	OutputType reflect.Type
}

// NewBaseFunctionDefinition creates a new base function with input/output type information
func NewBaseFunctionDefinition(name, version, description string, inputType, outputType reflect.Type, tags []string) *BaseFunctionDefinition {
	// Generate the type schemas using reflection
	inputTypeSchema := getTypeSchema(inputType)
	outputTypeSchema := getTypeSchema(outputType)

	// Convert schemas to JSON strings
	inputTypeJSON, _ := json.Marshal(inputTypeSchema)
	outputTypeJSON, _ := json.Marshal(outputTypeSchema)

	server := os.Getenv("SERVER_NAME")

	return &BaseFunctionDefinition{
		FunctionDefinition: FunctionDefinition{
			Name:        name,
			Version:     version,
			Description: description,
			InputsType:  string(inputTypeJSON),
			OutputsType: string(outputTypeJSON),
			Server:      server,
			Tags:        tags,
		},
		InputType:  inputType,
		OutputType: outputType,
	}
}

// SetServer allows injecting server name after construction
func (b *BaseFunctionDefinition) SetServer(name string) { b.Server = name }

func (b *BaseFunctionDefinition) GetName() string {
	return b.Name
}

func (b *BaseFunctionDefinition) GetVersion() string {
	return b.Version
}

func (b *BaseFunctionDefinition) GetFunctionDefinition() FunctionDefinition {
	return b.FunctionDefinition
}

func (b *BaseFunctionDefinition) Serialize() ([]byte, error) {
	return json.Marshal(b)
}

func (b *BaseFunctionDefinition) GetInputsReflection() string {
	schema := getTypeSchema(b.InputType)
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func (b *BaseFunctionDefinition) GetOutputsReflection() string {
	schema := getTypeSchema(b.OutputType)
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// getTypeSchema returns a flattened map of type information using dot notation
// Arrays are prefixed with [] and maps with [map]
func getTypeSchema(t reflect.Type) map[string]interface{} {
	result := make(map[string]interface{})
	buildSchema(t, "", result)
	// addInnerSructsOrArraysToTypeSchema(result)
	return result
}

// func addInnerSructsOrArraysToTypeSchema(schema map[string]interface{}) {
// 	// First, collect all keys to avoid modifying the map during iteration
// 	var keys []string
// 	for key := range schema {
// 		keys = append(keys, key)
// 	}

// 	// Process each key
// 	for _, key := range keys {
// 		// Handle array type keys
// 		if strings.HasPrefix(key, "[]") {
// 			if valueType, ok := schema[key].(string); ok {
// 				schema[key] = valueType[2:] // Remove the "[]" prefix from the type
// 			}
// 		}

// 		// Handle nested paths with dots
// 		if strings.Contains(key, ".") {
// 			parts := strings.Split(key, ".")

// 			// Build up intermediate paths
// 			currentPath := ""
// 			for _, part := range parts[:len(parts)-1] { // Process all but the last part
// 				if currentPath == "" {
// 					currentPath = part
// 				} else {
// 					currentPath = currentPath + "." + part
// 				}

// 				// Only add if it doesn't already exist
// 				if _, exists := schema[currentPath]; !exists {
// 					// If it's an array path, add the array type
// 					if strings.HasSuffix(part, "[]") {
// 						schema[currentPath] = "string" // TODO: change to the actual type: array
// 					} else {
// 						schema[currentPath] = "string" // TODO: change to the actual type: object
// 					}
// 				}
// 			}
// 		}
// 	}
// }

// buildSchema recursively builds a flattened type schema using dot notation
func buildSchema(t reflect.Type, path string, result map[string]interface{}) {

	if t == nil {
		return
	}

	// If it's a pointer, get the underlying type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle different kinds of types
	switch t.Kind() {
	case reflect.Struct:
		// For structs, process each field
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Get field name from json tag or struct field name
			jsonTag := field.Tag.Get("json")
			fieldName := field.Name
			if jsonTag != "" {
				tagParts := strings.Split(jsonTag, ",")
				if tagParts[0] != "" {
					fieldName = tagParts[0]
				}
			}

			// Skip fields marked with json:"-"
			if fieldName == "-" {
				continue
			}

			// Build the field path
			fieldPath := fieldName
			if path != "" {
				fieldPath = path + "." + fieldName
			}

			// Process the field type
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			// Handle the field based on its kind
			processField(fieldType, fieldPath, result)
		}
	default:
		// For non-struct types, just add the type directly
		if path != "" {
			result[path] = t.String()
		} else {
			result["type"] = t.String()
		}
	}
}

// processField handles a field based on its type
func processField(fieldType reflect.Type, path string, result map[string]interface{}) {
	switch fieldType.Kind() {
	case reflect.Struct:
		// For structs, recursively process all fields
		buildSchema(fieldType, path, result)
	case reflect.Slice, reflect.Array:
		// For arrays, add [] suffix and process element type
		processArrayField(fieldType, path, result)
	case reflect.Map:
		// For maps, add [map] suffix
		keyType := fieldType.Key().String()
		valueType := fieldType.Elem().String()
		result[path+"[map]"] = fmt.Sprintf("map[%s]%s", keyType, valueType)
	default:
		// For primitive types, just add the type
		result[path] = fieldType.String()
	}
}

// processArrayField handles array types
func processArrayField(arrayType reflect.Type, path string, result map[string]interface{}) {
	// Add [] suffix to the path
	arrayPath := path + "[]"

	// Get the element type
	elemType := arrayType.Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	// Process the element type
	switch elemType.Kind() {
	case reflect.Struct:
		// For struct elements, recursively process the struct fields
		buildSchema(elemType, arrayPath, result)
	case reflect.Slice, reflect.Array:
		// Special case: if the element type is a named type (like InnerInnerStruct in the example)
		// and it's a slice/array, we need to handle it differently to avoid double brackets
		if elemType.Name() != "" {
			// This is a named slice type (like type MySlice []int)
			// We should process its elements directly
			innerElemType := elemType.Elem()
			if innerElemType.Kind() == reflect.Struct {
				buildSchema(innerElemType, arrayPath, result)
			} else {
				// For primitive types in a named slice type
				result[arrayPath] = innerElemType.String()
			}
		} else {
			// For regular nested arrays, recursively process with the array path
			processArrayField(elemType, arrayPath, result)
		}
	default:
		// For primitive types, just add the type
		result[arrayPath] = elemType.String()
	}
}

// Function is a generic function type that makes it easier to create new functions
type Function[In any, Out any] struct {
	BaseFunctionDefinition
	Handler func(In, *types.EventMessage) (Out, error)
	Cache   FunctionCache
	TTL     time.Duration
}

// FunctionCache abstracts caching for functions (backed by Redis or gRPC cache)
type FunctionCache interface {
	Get(key uint64) ([]byte, bool)
	Set(key uint64, value []byte) error
	SetWithTTL(key uint64, value []byte, ttl time.Duration) error
}

// NewFunction creates a new function with a simpler interface
func NewFunction[In any, Out any](
	name string,
	version string,
	description string,
	handler func(In, *types.EventMessage) (Out, error),
	tags []string,
) *Function[In, Out] {
	base := NewBaseFunctionDefinition(
		name,
		version,
		description,
		reflect.TypeOf(*new(In)),
		reflect.TypeOf(*new(Out)),
		tags,
	)

	return &Function[In, Out]{
		BaseFunctionDefinition: *base,
		Handler:                handler,
		TTL:                    0, // TTL == 0 disables caching (no get/set)
	}
}

// SetCache sets the cache for the function
func (f *Function[In, Out]) SetCache(cache FunctionCache) {
	f.Cache = cache
}

// SetCacheTTL sets a custom TTL for this function's cache entries
func (f *Function[In, Out]) SetCacheTTL(ttl time.Duration) {
	f.TTL = ttl
}

// Execute implements the FunctionInterface
func (f *Function[In, Out]) Execute(inputs *[]byte, eventState *types.EventMessage) (*[]byte, error) {
	// If cache is available and TTL != 0, try to get the cached result
	if f.Cache != nil && f.TTL != 0 {
		cacheKey := hash.Generate(*inputs, f.Name, f.Version)

		// Pretty print the JSON input with proper indentation
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, *inputs, "", "  "); err == nil {
			log.Printf("Cache lookup [%s:%s]\nInput:\n%s\nKey: %d",
				f.Name, f.Version, prettyJSON.String(), cacheKey)
		} else {
			// Fallback to raw input if JSON indentation fails
			inputStr := string(*inputs)
			const maxLength = 100
			if len(inputStr) > maxLength {
				inputStr = inputStr[:maxLength] + "..."
			}
			log.Printf("Cache lookup [%s:%s] Input: %s Key: %d",
				f.Name, f.Version, inputStr, cacheKey)
		}

		if cachedResult, found := f.Cache.Get(cacheKey); found {
			var cachedResultOut Out
			if err := json.Unmarshal(cachedResult, &cachedResultOut); err != nil {
				return nil, fmt.Errorf("failed to unmarshal cached result: %w", err)
			}
			log.Printf("Cache HIT [%s:%s] Output: %v", f.Name, f.Version, cachedResultOut)
			return &cachedResult, nil
		}
		log.Printf("Cache MISS [%s:%s]", f.Name, f.Version)
	}

	var input In
	if err := json.Unmarshal(*inputs, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input: %w", err)
	}

	result, err := f.Handler(input, eventState)
	if err != nil {
		return nil, fmt.Errorf("handler error: %w", err)
	}

	output, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output: %w", err)
	}

	// Cache the result only when cache is available and TTL != 0
	if f.Cache != nil && f.TTL != 0 {
		cacheKey := hash.Generate(*inputs, f.Name, f.Version)

		// Use custom TTL if set, otherwise use default
		if f.TTL > 0 {
			f.Cache.SetWithTTL(cacheKey, output, f.TTL)
			log.Printf("Cache STORE [%s:%s] Key: %d (TTL: %v)", f.Name, f.Version, cacheKey, f.TTL)
		} else {
			f.Cache.Set(cacheKey, output)
			log.Printf("Cache STORE [%s:%s] Key: %d", f.Name, f.Version, cacheKey)
		}
	}

	return &output, nil
}
