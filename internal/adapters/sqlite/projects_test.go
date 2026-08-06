package sqlite

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectRepositoryReconcilesAndOrders(t *testing.T) {
	t.Parallel()
	database, err := OpenOrCreate(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	migrations, err := fs.Sub(EmbeddedMigrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if migrateErr := Migrate(context.Background(), database.SQL(), migrations); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	if migrateErr := Migrate(context.Background(), database.SQL(), migrations); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	repository := NewProjectRepository(database.GORM())
	first := time.Date(2026, 8, 5, 1, 2, 3, 0, time.UTC)
	created, err := repository.Reconcile(context.Background(), "b", "Zulu", "/old", first)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := repository.Reconcile(context.Background(), "b", "Zulu", "/old", first.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.UpdatedAt != created.UpdatedAt {
		t.Fatal("unchanged reconciliation changed timestamp")
	}
	updated, err := repository.Reconcile(context.Background(), "b", "Alpha", "/new", first.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if updated.CreatedAt != created.CreatedAt || !updated.UpdatedAt.Equal(first.Add(time.Hour)) {
		t.Fatal("reconciliation timestamps are incorrect")
	}
	if _, reconcileErr := repository.Reconcile(context.Background(), "a", "Alpha", "/a", first); reconcileErr != nil {
		t.Fatal(reconcileErr)
	}
	values, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].ID != "a" || values[1].ID != "b" {
		t.Fatalf("unexpected order: %#v", values)
	}
}
