package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/search"
)

func TestSearchRepositoryWeightsFiltersTagsAndTextColumns(t *testing.T) {
	memories, _ := memoryTestRepository(t)
	ctx := context.Background()
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	goTag := memory.Tag{ID: "go", Name: "Go", NormalizedName: "go", CreatedAt: stamp, UpdatedAt: stamp}
	dbTag := memory.Tag{ID: "db", Name: "Database", NormalizedName: "database", CreatedAt: stamp, UpdatedAt: stamp}
	title := memoryRecord("title", "p1", strings64("1"), "needle", goTag, dbTag)
	title.Memory.Content = "plain"
	title.Memory.Importance = 5
	content := memoryRecord("content", "p1", strings64("2"), "plain", goTag)
	content.Memory.Content = "needle"
	tagOnly := memoryRecord("tag-only", "p1", strings64("3"), "plain", memory.Tag{ID: "needle-tag", Name: "needle", NormalizedName: "needle", CreatedAt: stamp, UpdatedAt: stamp})
	tagOnly.Memory.Content = "plain"
	otherTag := goTag
	otherTag.ID = "other-go"
	other := memoryRecord("other", "p2", strings64("4"), "needle", otherTag)
	for _, record := range []memory.CreateRecord{title, content, tagOnly, other} {
		if _, err := memories.Create(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSearchRepository(memories.db)
	values, err := repo.Candidates(ctx, "p1", search.CandidateFilter{Match: `"needle"`, Tags: []string{"go", "database"}, MinImportance: 4, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Memory.ID != "title" || len(values[0].Memory.Tags) != 2 {
		t.Fatalf("filtered candidates = %#v", values)
	}
	values, err = repo.Candidates(ctx, "p1", search.CandidateFilter{Match: `"needle"`, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].Memory.ID != "title" || values[0].BM25 > values[1].BM25 {
		t.Fatalf("weighted candidates = %#v", values)
	}
}

func TestSearchRepositoryCandidateCutoffIsDeterministic(t *testing.T) {
	memories, _ := memoryTestRepository(t)
	for i := range 105 {
		id := fmt.Sprintf("m%03d", i)
		record := memoryRecord(id, "p1", fmt.Sprintf("%064d", i), "common")
		if _, err := memories.Create(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	values, err := NewSearchRepository(memories.db).Candidates(context.Background(), "p1", search.CandidateFilter{Match: `"common"`, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 100 || values[0].Memory.ID != "m000" || values[99].Memory.ID != "m099" {
		t.Fatalf("unexpected window: len=%d first=%s last=%s", len(values), values[0].Memory.ID, values[99].Memory.ID)
	}
}
