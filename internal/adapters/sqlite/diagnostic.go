package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"slices"

	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
)

// DiagnosticChecker performs non-destructive checks against SQLite.
type DiagnosticChecker struct{ migrations fs.FS }

// NewDiagnosticChecker creates a checker for the application's migrations.
func NewDiagnosticChecker(migrations fs.FS) *DiagnosticChecker {
	return &DiagnosticChecker{migrations: migrations}
}

// Check runs every stable check in order, including blocked dependants.
func (checker *DiagnosticChecker) Check(ctx context.Context, path string) ([]diagnostic.Check, error) {
	checks := make([]diagnostic.Check, 0, 6)
	access, database := checker.databaseAccess(ctx, path)
	checks = append(checks, access)
	if database == nil {
		for _, name := range []string{"schema_compatibility", "sqlite_integrity", "foreign_keys", "fts5", "fts_index_consistency"} {
			checks = append(checks, failedCheck(name, "check blocked by database access", map[string]any{"blocked_by": "database_access"}))
		}
		return checks, nil
	}
	defer func() { _ = database.Close() }()
	checks = append(checks,
		checker.schemaCompatibility(ctx, database.SQL()),
		sqliteIntegrity(ctx, database.SQL()),
		foreignKeys(ctx, database.SQL()),
		fts5Available(ctx, database.SQL()),
		ftsConsistency(ctx, database.SQL()),
	)
	return checks, nil
}

func (checker *DiagnosticChecker) databaseAccess(ctx context.Context, path string) (diagnostic.Check, *Database) {
	details := map[string]any{"exists": false, "readable": false, "writable": false}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return failedCheck("database_access", "database file does not exist", details), nil
		}
		return failedCheck("database_access", "database file is not accessible", details), nil
	}
	details["exists"] = true
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		details["writable"] = true
		_ = file.Close()
	}
	database, err := OpenReadOnly(ctx, path)
	if err != nil {
		return failedCheck("database_access", "database file is not readable", details), nil
	}
	details["readable"] = true
	if details["writable"] != true {
		_ = database.Close()
		return failedCheck("database_access", "database file is not writable", details), nil
	}
	return passedCheck("database_access", "database file is accessible", details), database
}

func (checker *DiagnosticChecker) schemaCompatibility(ctx context.Context, db *sql.DB) diagnostic.Check {
	known, err := migrationVersions(checker.migrations)
	if err != nil || len(known) == 0 {
		return failedCheck("schema_compatibility", "schema version is incompatible", map[string]any{})
	}
	target := known[len(known)-1]
	details := map[string]any{"target_version": target}
	if err = ValidateSchema(ctx, db, checker.migrations); err != nil {
		return failedCheck("schema_compatibility", "schema version is incompatible", details)
	}
	var exists int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='goose_db_version'`).Scan(&exists); err != nil || exists == 0 {
		details["applied_version"] = int64(0)
		return failedCheck("schema_compatibility", "schema is not initialized", details)
	}
	var applied sql.NullInt64
	err = db.QueryRowContext(ctx, `SELECT max(version_id) FROM goose_db_version WHERE is_applied=1`).Scan(&applied)
	if err != nil || !applied.Valid {
		details["applied_version"] = int64(0)
		return failedCheck("schema_compatibility", "schema metadata is invalid", details)
	}
	details["applied_version"] = applied.Int64
	if applied.Int64 != target {
		return failedCheck("schema_compatibility", "schema is not at the target version", details)
	}
	return passedCheck("schema_compatibility", "schema is compatible", details)
}

func sqliteIntegrity(ctx context.Context, db *sql.DB) diagnostic.Check {
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return failedCheck("sqlite_integrity", "SQLite integrity check failed", map[string]any{})
	}
	defer func() { _ = rows.Close() }()
	problems := make([]string, 0)
	for rows.Next() {
		var result string
		if err = rows.Scan(&result); err != nil {
			return failedCheck("sqlite_integrity", "SQLite integrity check failed", map[string]any{})
		}
		if result != "ok" {
			problems = append(problems, result)
		}
	}
	details := map[string]any{"problems": problems}
	if len(problems) != 0 || rows.Err() != nil {
		return failedCheck("sqlite_integrity", "SQLite integrity check found problems", details)
	}
	return passedCheck("sqlite_integrity", "SQLite integrity check passed", details)
}

func foreignKeys(ctx context.Context, db *sql.DB) diagnostic.Check {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return failedCheck("foreign_keys", "foreign key check failed", map[string]any{})
	}
	defer func() { _ = rows.Close() }()
	violations := make([]map[string]any, 0)
	for rows.Next() {
		var table, parent string
		var rowID sql.NullInt64
		var fkID int64
		if err = rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			return failedCheck("foreign_keys", "foreign key check failed", map[string]any{})
		}
		violations = append(violations, map[string]any{"table": table, "row_id": rowID.Int64, "parent": parent, "foreign_key_id": fkID})
	}
	details := map[string]any{"violations": violations}
	if len(violations) != 0 || rows.Err() != nil {
		return failedCheck("foreign_keys", "foreign key violations found", details)
	}
	return passedCheck("foreign_keys", "foreign keys are valid", details)
}

func fts5Available(ctx context.Context, db *sql.DB) diagnostic.Check {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_module_list WHERE name='fts5'`).Scan(&count); err != nil || count != 1 {
		return failedCheck("fts5", "FTS5 is not available", map[string]any{"available": false})
	}
	return passedCheck("fts5", "FTS5 is available", map[string]any{"available": true})
}

type ftsEntry struct{ title, content string }

func ftsConsistency(ctx context.Context, db *sql.DB) diagnostic.Check {
	details := map[string]any{"missing": []string{}, "duplicates": []string{}, "extra": []string{}, "title_mismatches": []string{}, "content_mismatches": []string{}}
	memories, err := queryText(ctx, db, `SELECT id,title,content FROM memories WHERE deleted_at IS NULL`)
	if err != nil {
		return failedCheck("fts_index_consistency", "FTS index consistency check failed", details)
	}
	indexed, duplicates, err := queryFTS(ctx, db)
	if err != nil {
		return failedCheck("fts_index_consistency", "FTS index consistency check failed", details)
	}
	missing, extra, titles, contents := make([]string, 0), make([]string, 0), make([]string, 0), make([]string, 0)
	for id, memory := range memories {
		entry, ok := indexed[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		if entry.title != memory.title {
			titles = append(titles, id)
		}
		if entry.content != memory.content {
			contents = append(contents, id)
		}
	}
	for id := range indexed {
		if _, ok := memories[id]; !ok {
			extra = append(extra, id)
		}
	}
	slices.Sort(missing)
	slices.Sort(duplicates)
	slices.Sort(extra)
	slices.Sort(titles)
	slices.Sort(contents)
	details["missing"], details["duplicates"], details["extra"] = missing, duplicates, extra
	details["title_mismatches"], details["content_mismatches"] = titles, contents
	if len(missing)+len(duplicates)+len(extra)+len(titles)+len(contents) != 0 {
		return failedCheck("fts_index_consistency", "FTS index is inconsistent", details)
	}
	return passedCheck("fts_index_consistency", "FTS index is consistent", details)
}

func queryText(ctx context.Context, db *sql.DB, query string) (map[string]ftsEntry, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	values := map[string]ftsEntry{}
	for rows.Next() {
		var id string
		var entry ftsEntry
		if err = rows.Scan(&id, &entry.title, &entry.content); err != nil {
			return nil, err
		}
		values[id] = entry
	}
	return values, rows.Err()
}

func queryFTS(ctx context.Context, db *sql.DB) (map[string]ftsEntry, []string, error) {
	rows, err := db.QueryContext(ctx, `SELECT memory_id,title,content FROM memory_fts`)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	values, duplicates := map[string]ftsEntry{}, make([]string, 0)
	for rows.Next() {
		var id string
		var entry ftsEntry
		if err = rows.Scan(&id, &entry.title, &entry.content); err != nil {
			return nil, nil, err
		}
		if _, ok := values[id]; ok {
			duplicates = append(duplicates, id)
		}
		values[id] = entry
	}
	return values, duplicates, rows.Err()
}

func passedCheck(name, message string, details map[string]any) diagnostic.Check {
	return diagnostic.Check{Name: name, OK: true, Message: message, Details: details}
}

func failedCheck(name, message string, details map[string]any) diagnostic.Check {
	return diagnostic.Check{Name: name, OK: false, Message: message, Details: details}
}
