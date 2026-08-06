package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
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

func TestMemoryFTSMigrationUpAndDownSchemas(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(context.Background(), db, migrations); err != nil {
		t.Fatal(err)
	}
	assertFTSColumns(t, db, []string{"memory_id", "project_id", "title", "content"})
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Down(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertFTSColumns(t, db, []string{"memory_id", "project_id", "title", "content", "tags"})
}

func assertFTSColumns(t *testing.T, db *sql.DB, want []string) {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(memory_fts)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("columns = %v, want %v", got, want)
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

func TestValidateSchemaRejectsUnknownAndGappedVersions(t *testing.T) {
	t.Parallel()
	migrations := fstest.MapFS{
		"00001_one.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;")},
		"00002_two.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;")},
	}
	for name, versions := range map[string][]int64{"unknown": {1, 3}, "gap": {2}} {
		t.Run(name, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if _, err = db.Exec(`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL, tstamp TEXT);`); err != nil {
				t.Fatal(err)
			}
			for i, version := range versions {
				if _, err = db.Exec(`INSERT INTO goose_db_version(id,version_id,is_applied) VALUES(?,?,1)`, i+1, version); err != nil {
					t.Fatal(err)
				}
			}
			if err = ValidateSchema(context.Background(), db, migrations); err == nil {
				t.Fatal("expected incompatible schema")
			}
		})
	}
}

func TestValidateExistingSchemaDoesNotChangeUnknownDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "unknown.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL, tstamp TEXT); INSERT INTO goose_db_version VALUES(1,99,1,'now')`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateExistingSchema(context.Background(), path, migrations); err == nil {
		t.Fatal("expected incompatible schema")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("validation changed incompatible database")
	}
}
