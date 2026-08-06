// Package diagnostic defines non-destructive storage health diagnostics.
package diagnostic

import "fmt"

// Check is one stable diagnostic result.
type Check struct {
	Name    string         `json:"name"`
	OK      bool           `json:"ok"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// Report describes the global database health at one path.
type Report struct {
	Healthy      bool    `json:"healthy"`
	DatabasePath string  `json:"database_path"`
	Checks       []Check `json:"checks"`
}

// UnhealthyError exposes the complete safe report to presentation adapters.
type UnhealthyError struct{ Report Report }

func (err *UnhealthyError) Error() string { return "storage diagnostics failed" }

// Details returns the report as safe public error details.
func (err *UnhealthyError) Details() map[string]any {
	return map[string]any{
		"healthy":       err.Report.Healthy,
		"database_path": err.Report.DatabasePath,
		"checks":        err.Report.Checks,
	}
}

func validateChecks(checks []Check) error {
	want := [...]string{"database_access", "schema_compatibility", "sqlite_integrity", "foreign_keys", "fts5", "fts_index_consistency"}
	if len(checks) != len(want) {
		return fmt.Errorf("diagnostic checker returned %d checks", len(checks))
	}
	for i := range want {
		if checks[i].Name != want[i] || checks[i].Message == "" || checks[i].Details == nil {
			return fmt.Errorf("invalid diagnostic check %d", i)
		}
	}
	return nil
}
