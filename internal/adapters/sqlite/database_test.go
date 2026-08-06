package sqlite

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenOrCreateCreatesConfiguredEmptyDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "data", databaseFilename)
	database, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	if database.Path() != path {
		t.Errorf("Path() = %q, want %q", database.Path(), path)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)

	var tableCount int
	if err := database.SQL().QueryRowContext(context.Background(), `SELECT count(*) FROM sqlite_schema WHERE type = 'table'`).Scan(&tableCount); err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if tableCount != 0 {
		t.Errorf("new database has %d tables, want 0", tableCount)
	}

	assertPragma(t, database, "foreign_keys", 1)
	assertPragma(t, database, "journal_mode", "wal")
	assertPragma(t, database, "busy_timeout", 100)
}

func TestOpenOrCreateReusesExistingDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "data", databaseFilename)
	first, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatalf("first open() error = %v", err)
	}
	_, insertErr := first.SQL().ExecContext(context.Background(), `CREATE TABLE sentinel (value TEXT NOT NULL); INSERT INTO sentinel VALUES ('preserved')`)
	if insertErr != nil {
		t.Fatalf("insert sentinel: %v", insertErr)
	}
	closeErr := first.Close()
	if closeErr != nil {
		t.Fatalf("first Close() error = %v", closeErr)
	}

	second, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatalf("second open() error = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	var value string
	if err := second.SQL().QueryRowContext(context.Background(), `SELECT value FROM sentinel`).Scan(&value); err != nil {
		t.Fatalf("read sentinel: %v", err)
	}
	if value != "preserved" {
		t.Errorf("sentinel value = %q, want preserved", value)
	}
}

func TestGORMUsesSharedSQLConnection(t *testing.T) {
	t.Parallel()

	database, err := OpenOrCreate(context.Background(), filepath.Join(t.TempDir(), databaseFilename))
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.SQL().ExecContext(context.Background(), `CREATE TEMP TABLE shared_connection (value TEXT); INSERT INTO shared_connection VALUES ('visible')`); err != nil {
		t.Fatalf("create temporary table: %v", err)
	}

	var value string
	if err := database.GORM().WithContext(context.Background()).Raw(`SELECT value FROM shared_connection`).Scan(&value).Error; err != nil {
		t.Fatalf("query temporary table with GORM: %v", err)
	}
	if value != "visible" {
		t.Errorf("GORM value = %q, want visible", value)
	}
}

func TestOpenExistingDoesNotCreateMissingDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", databaseFilename)
	if _, err := OpenExisting(context.Background(), path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenExisting() error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database directory was created: %v", err)
	}
}

func assertMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("mode for %q = %04o, want %04o", path, got, want)
	}
}

func assertPragma[T comparable](t *testing.T, database *Database, pragma string, want T) {
	t.Helper()

	var got T
	if err := database.SQL().QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&got); err != nil {
		t.Fatalf("query PRAGMA %s: %v", pragma, err)
	}
	if got != want {
		t.Errorf("PRAGMA %s = %v, want %v", pragma, got, want)
	}
}
