package util

import (
	"testing"
)

func TestRandomHex_Length(t *testing.T) {
	for _, n := range []int{0, 1, 8, 32, 64} {
		got := RandomHex(n)
		if len(got) != n {
			t.Errorf("RandomHex(%d) len = %d, want %d", n, len(got), n)
		}
	}
}

func TestRandomHex_HexChars(t *testing.T) {
	got := RandomHex(100)
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("RandomHex contains non-hex char: %c", c)
		}
	}
}

func TestRandomHex_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := RandomHex(16)
		if seen[s] {
			t.Error("RandomHex produced duplicate value")
		}
		seen[s] = true
	}
}
