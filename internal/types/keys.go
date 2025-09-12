package types

import "fmt"

// FunctionKey returns the canonical storage key for a function entry.
// Kept stable for cross-language consistency.
func FunctionKey(server, name, version string) string {
	return fmt.Sprintf("%s%s:%s:%s", FUNCTION_PREFIX, server, name, version)
}
