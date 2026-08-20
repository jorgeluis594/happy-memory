package sqlite

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosticCheckerHealthyAndDoesNotMutateDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), databaseFilename)
	database, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(context.Background(), database.SQL(), migrations); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := NewDiagnosticChecker(migrations).Check(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if !check.OK {
			t.Errorf("check %s failed: %#v", check.Name, check.Details)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("diagnostic changed database file")
	}
}

func TestFTSConsistencyReportsSpecificTagsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), databaseFilename)
	database, err := OpenOrCreate(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(context.Background(), database.SQL(), migrations); err != nil {
		t.Fatal(err)
	}
	if _, err = database.SQL().Exec(`INSERT INTO projects(id,name,last_known_git_common_dir,created_at,updated_at) VALUES('p','p','/p','2026-08-05T12:00:00Z','2026-08-05T12:00:00Z');
INSERT INTO memories(id,project_id,current_version,type,title,content,importance,confidence,content_hash,created_at,updated_at) VALUES('m','p',1,'fact','title','content',3,3,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','2026-08-05T12:00:00Z','2026-08-05T12:00:00Z');
INSERT INTO memory_fts(memory_id,project_id,title,content,specific_tags) VALUES('m','p','title','content','wrong');`); err != nil {
		t.Fatal(err)
	}
	check := ftsConsistency(context.Background(), database.SQL())
	if check.OK {
		t.Fatalf("check=%#v", check)
	}
	values, ok := check.Details["specific_tags_mismatches"].([]string)
	if !ok || len(values) != 1 || values[0] != "m" {
		t.Fatalf("details=%#v", check.Details)
	}
}

func TestDiagnosticCheckerReportsMissingDatabaseWithoutCreatingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	checks, err := NewDiagnosticChecker(migrations).Check(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if checks[0].OK || len(checks) != 6 {
		t.Fatalf("checks=%#v", checks)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database was created: %v", err)
	}
}
