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
