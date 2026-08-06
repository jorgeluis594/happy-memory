package sqlite

import "path/filepath"

// PathFromGitCommonDir returns the database shared by all worktrees attached
// to one Git common directory.
func PathFromGitCommonDir(commonDir string) string {
	return filepath.Join(filepath.Dir(commonDir), applicationDirectory, databaseFilename)
}

// DiagnosticFallbackPath is the non-creating path reported by doctor when the
// current directory is not inside a Git repository.
func DiagnosticFallbackPath() string {
	return filepath.Join(applicationDirectory, databaseFilename)
}
