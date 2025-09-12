package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Generate a random UID in the format "d83a-f68e"
func UID() string {
	// Pre-allocate buffer with exact size needed
	b := make([]byte, 9)

	// Generate 4 random bytes for each part
	rand.Read(b[:4])
	rand.Read(b[4:8])

	// Format as hex with dash
	return fmt.Sprintf("%04x-%04x",
		binary.BigEndian.Uint32(b[:4])&0xffff,
		binary.BigEndian.Uint32(b[4:8])&0xffff)
}
