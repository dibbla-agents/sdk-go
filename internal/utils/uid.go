package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// UID returns a random correlation ID in the format "d83af68e-1b2c3d4e",
// carrying the full 64 bits of entropy (birthday collision ~1 in 2^32 even
// after billions of IDs). Callers treat it as an opaque string.
func UID() string {
	b := make([]byte, 8)
	// crypto/rand.Read never returns an error on supported platforms
	// (it panics if randomness is unavailable, per its documentation).
	rand.Read(b)

	return fmt.Sprintf("%08x-%08x",
		binary.BigEndian.Uint32(b[:4]),
		binary.BigEndian.Uint32(b[4:8]))
}
