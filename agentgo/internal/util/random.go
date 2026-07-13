package util

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomHex returns a hex-encoded random string of length n.
func RandomHex(n int) string {
	// hex.EncodeToString produces 2 hex chars per byte, so we need n/2 bytes.
	byteLen := n / 2
	if n%2 != 0 {
		byteLen++
	}
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
