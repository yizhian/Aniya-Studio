package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRuntimeDir_UsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENTGO_TEST_DIR", dir)

	got := resolveRuntimeDir("AGENTGO_TEST_DIR", "missing")
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveRuntimeDir() = %q, want %q", got, want)
	}
}

func TestResolveRuntimeDir_FindsAgentgoDirFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "agentgo", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	got := resolveRuntimeDir("AGENTGO_TEST_DIR", "skills")
	want, err := filepath.Abs(skillsDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err = filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveRuntimeDir() = %q, want %q", got, want)
	}
}
