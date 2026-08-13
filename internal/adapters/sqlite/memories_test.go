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
	database, err := OpenOrCreate(context.Background(), filepath.Join(t.TempDir(), "test.db"))
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
	got, err := repo.Get(context.Background(), "p1", "m1", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "first" || len(got.Tags) != 1 {
		t.Fatalf("got %#v", got)
	}
	_, err = repo.Get(context.Background(), "p2", "m1", false)
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
	firstDescription, ignoredDescription := "Git language", "must not replace"
	goTag := memory.Tag{ID: "go-id", Name: "Go", NormalizedName: "go", Description: &firstDescription, CreatedAt: stamp, UpdatedAt: stamp}
	dbTag := memory.Tag{ID: "db-id", Name: "Database", NormalizedName: "database", CreatedAt: stamp, UpdatedAt: stamp}
	if _, err := repo.Create(context.Background(), memoryRecord("m1", "p1", strings64("b"), "one", goTag, dbTag)); err != nil {
		t.Fatal(err)
	}
	variant := memory.Tag{ID: "new-id", Name: "GO", NormalizedName: "go", Description: &ignoredDescription, CreatedAt: stamp, UpdatedAt: stamp}
	record := memoryRecord("m2", "p1", strings64("c"), "two", variant)
	record.Memory.Importance = 2
	if _, err := repo.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), "p1", "m2", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tags[0].ID != "go-id" || got.Tags[0].Name != "Go" || got.Tags[0].Description == nil || *got.Tags[0].Description != firstDescription {
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
	tags, err := repo.SearchTags(context.Background(), "p1", "language", 10)
	if err != nil || len(tags) != 1 || tags[0].ID != "go-id" || tags[0].ActiveMemoryCount != 2 {
		t.Fatalf("searched tags=%#v err=%v", tags, err)
	}
	tags, err = repo.SearchTags(context.Background(), "p1", "a", 10)
	if err != nil || len(tags) != 2 || tags[0].ID != "go-id" || tags[1].ID != "db-id" {
		t.Fatalf("ordered tags=%#v err=%v", tags, err)
	}
	tags, err = repo.SearchTags(context.Background(), "p1", "a", 1)
	if err != nil || len(tags) != 1 || tags[0].ID != "go-id" {
		t.Fatalf("limited ordered tags=%#v err=%v", tags, err)
	}
	otherProjectTag := memory.Tag{ID: "other-go", Name: "Go", NormalizedName: "go", CreatedAt: stamp, UpdatedAt: stamp}
	if _, err = repo.Create(context.Background(), memoryRecord("p2-memory", "p2", strings64("z"), "other", otherProjectTag)); err != nil {
		t.Fatal(err)
	}
	result := database.GORM().Exec("INSERT INTO memory_tags (project_id,memory_id,tag_id,created_at) VALUES (?,?,?,?)", "p1", "m1", "other-go", stamp.Format(time.RFC3339))
	if result.Error == nil {
		t.Fatal("cross-project association was accepted")
	}
	var crossCount int
	if err = database.GORM().Raw("SELECT count(*) FROM memory_tags WHERE memory_id = ? AND tag_id = ?", "m1", "other-go").Scan(&crossCount).Error; err != nil || crossCount != 0 {
		t.Fatalf("cross-project association count=%d err=%v", crossCount, err)
	}
	tags, err = repo.ListTags(context.Background(), "p1")
	if err != nil || len(tags) != 2 || tags[0].NormalizedName != "database" || tags[1].ID != "go-id" {
		t.Fatalf("listed tags=%#v err=%v", tags, err)
	}
}
func strings64(v string) string { return strings.Repeat(v, 64) }

func TestMemoryRepositoryLifecycleCASHistoryAndFTS(t *testing.T) {
	repo, database := memoryTestRepository(t)
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	tag := memory.Tag{ID: "tag-life", Name: "Go", NormalizedName: "go", CreatedAt: stamp, UpdatedAt: stamp}
	created, err := repo.Create(context.Background(), memoryRecord("life", "p1", strings64("d"), "first", tag))
	if err != nil {
		t.Fatal(err)
	}
	updated := created
	updated.Version = 2
	updated.Title = "second"
	updated.Tags = []memory.Tag{{ID: "tag-codex", Name: "Codex", NormalizedName: "codex", CreatedAt: stamp, UpdatedAt: stamp}}
	updated.ContentHash = strings64("e")
	updated.UpdatedAt = stamp.Add(time.Minute)
	revision := memory.Revision{MemoryID: "life", ProjectID: "p1", Version: 2, Operation: "update", AgentRole: "unknown", WorktreeRoot: "/repo", CreatedAt: updated.UpdatedAt}
	updated, err = repo.Mutate(context.Background(), memory.MutationRecord{Memory: updated, Revision: revision, ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	var specificTags string
	if err = database.GORM().Raw("SELECT specific_tags FROM memory_fts WHERE project_id = ? AND memory_id = ?", "p1", "life").Scan(&specificTags).Error; err != nil || specificTags != "codex" {
		t.Fatalf("updated specific tags=%q err=%v", specificTags, err)
	}
	_, err = repo.Mutate(context.Background(), memory.MutationRecord{Memory: updated, Revision: revision, ExpectedVersion: 1})
	if memory.ErrorCode(err) != memory.CodeVersionConflict || memory.ErrorDetails(err)["current_version"] != 2 {
		t.Fatalf("stale error=%#v", err)
	}
	deleted := updated
	deleted.Version = 3
	deleted.UpdatedAt = stamp.Add(2 * time.Minute)
	deleted.DeletedAt = &deleted.UpdatedAt
	revision = memory.Revision{MemoryID: "life", ProjectID: "p1", Version: 3, Operation: "delete", AgentRole: "unknown", WorktreeRoot: "/repo", CreatedAt: deleted.UpdatedAt}
	if _, err = repo.Mutate(context.Background(), memory.MutationRecord{Memory: deleted, Revision: revision, ExpectedVersion: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Get(context.Background(), "p1", "life", false); memory.ErrorCode(err) != memory.CodeNotFound {
		t.Fatalf("active get=%v", err)
	}
	if _, err = repo.Get(context.Background(), "p1", "life", true); err != nil {
		t.Fatal(err)
	}
	var ftsCount int
	if err = database.GORM().Raw("SELECT count(*) FROM memory_fts WHERE project_id = ? AND memory_id = ?", "p1", "life").Scan(&ftsCount).Error; err != nil || ftsCount != 0 {
		t.Fatalf("deleted fts=%d err=%v", ftsCount, err)
	}
	tags, err := repo.ListTags(context.Background(), "p1")
	if err != nil || len(tags) != 2 || tags[0].ActiveMemoryCount != 0 || tags[1].ActiveMemoryCount != 0 {
		t.Fatalf("deleted tag count=%#v err=%v", tags, err)
	}
	restored := created
	restored.Version = 4
	restored.UpdatedAt = stamp.Add(3 * time.Minute)
	revision = memory.Revision{MemoryID: "life", ProjectID: "p1", Version: 4, Operation: "restore", AgentRole: "unknown", WorktreeRoot: "/repo", CreatedAt: restored.UpdatedAt}
	if _, err = repo.Mutate(context.Background(), memory.MutationRecord{Memory: restored, Revision: revision, ExpectedVersion: 3}); err != nil {
		t.Fatal(err)
	}
	history, err := repo.History(context.Background(), "p1", "life")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 || history[0].Version != 1 || history[3].Version != 4 {
		t.Fatalf("history=%#v", history)
	}
	if err = database.GORM().Raw("SELECT count(*) FROM memory_fts WHERE project_id = ? AND memory_id = ?", "p1", "life").Scan(&ftsCount).Error; err != nil || ftsCount != 1 {
		t.Fatalf("restored fts=%d err=%v", ftsCount, err)
	}
	if err = database.GORM().Raw("SELECT specific_tags FROM memory_fts WHERE project_id = ? AND memory_id = ?", "p1", "life").Scan(&specificTags).Error; err != nil || specificTags != "go" {
		t.Fatalf("restored specific tags=%q err=%v", specificTags, err)
	}
	tags, err = repo.ListTags(context.Background(), "p1")
	if err != nil || len(tags) != 2 || tags[0].ActiveMemoryCount != 0 || tags[1].NormalizedName != "go" || tags[1].ActiveMemoryCount != 1 {
		t.Fatalf("restored tag count=%#v err=%v", tags, err)
	}
}
