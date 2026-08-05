package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMigrateAppliesFixture(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrations := fstest.MapFS{
		"00001_create_fixture.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
CREATE TABLE migration_fixture (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE migration_fixture;
`)},
	}
	if err := Migrate(context.Background(), db, migrations); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'migration_fixture'`).Scan(&count); err != nil {
		t.Fatalf("query fixture table: %v", err)
	}
	if count != 1 {
		t.Errorf("fixture table count = %d, want 1", count)
	}

	if err := db.PingContext(context.Background()); err != nil {
		t.Errorf("Migrate() closed caller-owned database: %v", err)
	}
}

func TestOpenDoesNotRunMigrations(t *testing.T) {
	t.Parallel()

	database, err := open(context.Background(), filepath.Join(t.TempDir(), databaseFilename))
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var gooseTables int
	if err := database.SQL().QueryRowContext(context.Background(), `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&gooseTables); err != nil {
		t.Fatalf("query goose metadata: %v", err)
	}
	if gooseTables != 0 {
		t.Errorf("goose metadata table count = %d, want 0", gooseTables)
	}
}
