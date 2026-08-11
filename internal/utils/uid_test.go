package utils

import (
	"regexp"
	"testing"
)

var uidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{8}$`)

// TestUIDFormat verifies the full-entropy format: 8+8 hex chars = 64 bits.
// (The previous format masked each half to 16 bits — 32 bits total — making
// birthday collisions likely around ~65k IDs.)
func TestUIDFormat(t *testing.T) {
	id := UID()
	if !uidPattern.MatchString(id) {
		t.Fatalf("UID %q does not match %s", id, uidPattern)
	}
}

// TestUIDNoCollisions samples enough IDs that the old 32-bit format would
// almost surely collide (100k draws from 2^32 ≈ 69% collision probability),
// while 64 bits collide with probability ~2.7e-10.
func TestUIDNoCollisions(t *testing.T) {
	seen := make(map[string]struct{}, 100_000)
	for i := 0; i < 100_000; i++ {
		id := UID()
		if _, dup := seen[id]; dup {
			t.Fatalf("collision after %d IDs: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}
