package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
)

func memoryTestRepository(t *testing.T) (*MemoryRepository, *Database) {
	t.Helper()
	database, err := open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
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
	stamp := "2026-08-05T12:00:00Z"
	if err = database.GORM().Create(&projectRow{ID: "p1", Name: "one", LastKnownGitCommonDir: "/one/.git", CreatedAt: stamp, UpdatedAt: stamp}).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.GORM().Create(&projectRow{ID: "p2", Name: "two", LastKnownGitCommonDir: "/two/.git", CreatedAt: stamp, UpdatedAt: stamp}).Error; err != nil {
		t.Fatal(err)
	}
	return NewMemoryRepository(database.GORM()), database
}
func memoryRecord(id, projectID, hash, title string, tags ...memory.Tag) memory.CreateRecord {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	m := memory.Memory{ID: id, ProjectID: projectID, Version: 1, Type: "fact", Title: title, Content: "content", Importance: 4, Confidence: 3, Attributes: json.RawMessage(`{"a":1}`), Tags: tags, ContentHash: hash, CreatedAt: stamp, UpdatedAt: stamp}
	snapshot, _ := memory.SnapshotJSON(m, nil, "unknown", "/repo")
	return memory.CreateRecord{Memory: m, Revision: memory.Revision{MemoryID: id, ProjectID: projectID, Version: 1, Operation: "create", Snapshot: snapshot, AgentRole: "unknown", WorktreeRoot: "/repo", CreatedAt: stamp}}
}

func TestMemoryRepositoryCreateGetDuplicateIsolationAndFTS(t *testing.T) {
	repo, database := memoryTestRepository(t)
	tag := memory.Tag{ID: "tag-1", Name: "Go", NormalizedName: "go", CreatedAt: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	first, err := repo.Create(context.Background(), memoryRecord("m1", "p1", strings64("a"), "first", tag))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tags) != 1 || first.Tags[0].ID != "tag-1" {
		t.Fatal(first.Tags)
	}
	got, err := repo.Get(context.Background(), "p1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "first" || len(got.Tags) != 1 {
		t.Fatalf("got %#v", got)
	}
	_, err = repo.Get(context.Background(), "p2", "m1")
	if memory.ErrorCode(err) != memory.CodeNotFound {
		t.Fatalf("cross-project get = %v", err)
	}
	_, err = repo.Create(context.Background(), memoryRecord("m2", "p1", strings64("a"), "duplicate"))
	var domainErr *memory.Error
	if !errors.As(err, &domainErr) || domainErr.Code != memory.CodeDuplicate || domainErr.Details["memory_id"] != "m1" {
		t.Fatalf("duplicate error = %#v", err)
	}
	if _, err = repo.Create(context.Background(), memoryRecord("m3", "p2", strings64("a"), "other project")); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = database.GORM().Raw(`SELECT count(*) FROM memory_fts WHERE memory_id = ?`, "m1").Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("fts count=%d err=%v", count, err)
	}
}

func TestMemoryRepositoryReusesCanonicalTagAndFiltersWithAND(t *testing.T) {
	repo, database := memoryTestRepository(t)
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	goTag := memory.Tag{ID: "go-id", Name: "Go", NormalizedName: "go", CreatedAt: stamp}
	dbTag := memory.Tag{ID: "db-id", Name: "Database", NormalizedName: "database", CreatedAt: stamp}
	if _, err := repo.Create(context.Background(), memoryRecord("m1", "p1", strings64("b"), "one", goTag, dbTag)); err != nil {
		t.Fatal(err)
	}
	variant := memory.Tag{ID: "new-id", Name: "GO", NormalizedName: "go", CreatedAt: stamp}
	record := memoryRecord("m2", "p1", strings64("c"), "two", variant)
	record.Memory.Importance = 2
	if _, err := repo.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), "p1", "m2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tags[0].ID != "go-id" || got.Tags[0].Name != "Go" {
		t.Fatalf("canonical tag = %#v", got.Tags[0])
	}
	values, err := repo.List(context.Background(), "p1", memory.ListFilter{Tags: []string{"go", "database"}, MinImportance: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ID != "m1" {
		t.Fatalf("filtered = %#v", values)
	}
	var snapshot string
	if err = database.GORM().Table("memory_revisions").Select("snapshot_json").Where("memory_id = ?", "m2").Scan(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot, `"id":"go-id"`) {
		t.Fatalf("snapshot did not use canonical tag: %s", snapshot)
	}
}
func strings64(v string) string { return strings.Repeat(v, 64) }
