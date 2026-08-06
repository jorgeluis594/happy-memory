package search

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
)

type projectStub struct{}

func (projectStub) Current(context.Context) (memory.ProjectContext, error) {
	return memory.ProjectContext{ID: "p"}, nil
}

type repositoryStub struct {
	filter CandidateFilter
	values []Candidate
	called bool
}

func (r *repositoryStub) Candidates(_ context.Context, _ string, filter CandidateFilter) ([]Candidate, error) {
	r.called = true
	r.filter = filter
	return r.values, nil
}

func TestSearchPreparesFixedWindowRanksThenLimits(t *testing.T) {
	now := time.Now()
	repo := &repositoryStub{values: []Candidate{
		{Memory: memory.Memory{ID: "low", Importance: 1, Confidence: 1, UpdatedAt: now}, BM25: -1},
		{Memory: memory.Memory{ID: "high", Importance: 5, Confidence: 5, UpdatedAt: now}, BM25: -2},
	}}
	response, err := NewService(projectStub{}, repo).Search(context.Background(), Input{Query: `git "worktree"`, Tags: []string{" Go ", "go"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.filter.Match != `"git" AND """worktree"""` || repo.filter.Limit != 100 || len(repo.filter.Tags) != 1 || repo.filter.Tags[0] != "go" {
		t.Fatalf("unexpected filter: %#v", repo.filter)
	}
	if len(response.Results) != 1 || response.Results[0].Memory.ID != "high" || math.Abs(response.Results[0].Score.Final-1) > 1e-12 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestSearchEqualBM25AndStableTies(t *testing.T) {
	now := time.Now()
	repo := &repositoryStub{values: []Candidate{
		{Memory: memory.Memory{ID: "b", Importance: 3, Confidence: 3, UpdatedAt: now}, BM25: -1},
		{Memory: memory.Memory{ID: "a", Importance: 3, Confidence: 3, UpdatedAt: now}, BM25: -1},
	}}
	response, err := NewService(projectStub{}, repo).Search(context.Background(), Input{Query: "term", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.Results[0].Score.Text != 1 || response.Results[0].Memory.ID != "a" {
		t.Fatalf("unexpected ranking: %#v", response.Results)
	}
}

func TestSearchValidationAndPunctuationOnly(t *testing.T) {
	for _, input := range []Input{{Query: "", Limit: 10}, {Query: "x", Limit: 0}, {Query: "x", Limit: 101}, {Query: "x", Type: "bad", Limit: 10}, {Query: "x", MinImportance: 6, Limit: 10}, {Query: "x", Tags: []string{"   "}, Limit: 10}} {
		if _, err := NewService(projectStub{}, &repositoryStub{}).Search(context.Background(), input); memory.ErrorCode(err) != memory.CodeValidationError {
			t.Fatalf("expected validation for %#v, got %v", input, err)
		}
	}
	repo := &repositoryStub{}
	response, err := NewService(projectStub{}, repo).Search(context.Background(), Input{Query: "!!!", Limit: 10})
	if err != nil || repo.called || response.RankingVersion != 1 || len(response.Results) != 0 {
		t.Fatalf("unexpected empty result: %#v %v", response, err)
	}
}
