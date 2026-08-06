package sqlite

import (
	"path/filepath"
	"testing"
)

func TestPathFromGitCommonDir(t *testing.T) {
	t.Parallel()

	commonDir := filepath.Join(string(filepath.Separator), "repos", "project", ".git")
	want := filepath.Join(string(filepath.Separator), "repos", "project", ".happy-memory", "memory.db")
	if got := PathFromGitCommonDir(commonDir); got != want {
		t.Fatalf("PathFromGitCommonDir() = %q, want %q", got, want)
	}
}

func TestDiagnosticFallbackPathIsRelative(t *testing.T) {
	t.Parallel()

	got := DiagnosticFallbackPath()
	if filepath.IsAbs(got) || got != filepath.Join(".happy-memory", "memory.db") {
		t.Fatalf("DiagnosticFallbackPath() = %q", got)
	}
}
