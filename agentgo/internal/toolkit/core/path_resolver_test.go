package core

import (
	"testing"
)

// TestWriteProtected_Skills verifies that skills is in the protectedNames set
// and that paths under skills/ are blocked by validateNotSystemPath.
func TestWriteProtected_Skills(t *testing.T) {
	if !protectedNames["skills"] {
		t.Error("expected skills to be in protectedNames")
	}

	// Resolve a path under skills/ — resolveWorkspacePath should succeed
	// but validateNotSystemPath should block it.
	err := validateNotSystemPath("/workspace/skills/coral-skill/example.html")
	if err == nil {
		t.Error("expected error when validating a path under skills/")
	}

	err = validateNotSystemPath("/workspace/skills")
	if err == nil {
		t.Error("expected error when validating the skills directory itself")
	}
}

// TestWriteProtected_SlideCraft verifies .slidecraft is also protected.
func TestWriteProtected_SlideCraft(t *testing.T) {
	if !protectedNames[".slidecraft"] {
		t.Error("expected .slidecraft to be in protectedNames")
	}

	err := validateNotSystemPath("/workspace/.slidecraft/config.json")
	if err == nil {
		t.Error("expected error for .slidecraft path")
	}
}

// TestValidateNotSystemPath_Allowed verifies normal paths are not blocked.
func TestValidateNotSystemPath_Allowed(t *testing.T) {
	tests := []string{
		"/workspace/index.html",
		"/workspace/deck.html",
		"/workspace/assets/style.css",
		"/workspace/.agentgo/memory/notes.md",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			if err := validateNotSystemPath(path); err != nil {
				t.Errorf("unexpected error for allowed path %q: %v", path, err)
			}
		})
	}
}
