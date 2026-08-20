package search

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
)

type projectStub struct {
	calls *int
	err   error
}

func (p projectStub) Current(context.Context) (memory.ProjectContext, error) {
	if p.calls != nil {
		*p.calls++
	}
	if p.err != nil {
		return memory.ProjectContext{}, p.err
	}
	return memory.ProjectContext{ID: "p"}, nil
}

type repositoryStub struct {
	filter CandidateFilter
	values []Candidate
	called bool
	calls  []CandidateFilter
	errors []error
	sets   [][]Candidate
}

func (r *repositoryStub) Candidates(_ context.Context, _ string, filter CandidateFilter) ([]Candidate, error) {
	r.called = true
	r.filter = filter
	r.calls = append(r.calls, filter)
	if len(r.errors) >= len(r.calls) && r.errors[len(r.calls)-1] != nil {
		return nil, r.errors[len(r.calls)-1]
	}
	if len(r.sets) >= len(r.calls) {
		return r.sets[len(r.calls)-1], nil
	}
	return r.values, nil
}

func TestSearchPreparesFixedWindowRanksThenLimits(t *testing.T) {
	now := time.Now()
	repo := &repositoryStub{values: []Candidate{
		{Memory: memory.Memory{ID: "low", Importance: 1, Confidence: 1, UpdatedAt: now}, BM25: -1},
		{Memory: memory.Memory{ID: "high", Importance: 5, Confidence: 5, UpdatedAt: now}, BM25: -2},
	}}
	response, err := NewService(projectStub{}, repo).Search(context.Background(), Input{Query: `git "worktree"`, Tags: []string{" Go ", "go"}, SpecificTags: []string{" Agent Orchestration ", "agent-orchestration"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.filter.Match != `"git" AND """worktree"""` || repo.filter.SpecificTagsMatch != `"agent-orchestration"` || repo.filter.Limit != 100 || len(repo.filter.Tags) != 1 || repo.filter.Tags[0] != "go" {
		t.Fatalf("unexpected filter: %#v", repo.filter)
	}
	if len(response.Results) != 1 || response.Results[0].Memory.ID != "high" || math.Abs(response.Results[0].Score.Final-1) > 1e-12 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestBatchBoundariesResolveOnceAndPreservePartialResults(t *testing.T) {
	service := NewService(projectStub{}, &repositoryStub{})
	for _, size := range []int{0, 101} {
		if _, err := service.Batch(context.Background(), make([]Input, size)); memory.ErrorCode(err) != memory.CodeValidationError {
			t.Fatalf("size %d error=%v", size, err)
		}
	}

	projectCalls := 0
	repoErr := memory.NewError(memory.CodeStoreError, context.Canceled)
	repo := &repositoryStub{values: []Candidate{{Memory: memory.Memory{ID: "one", Importance: 5, Confidence: 5}, BM25: -1}}, errors: []error{nil, repoErr}}
	response, err := NewService(projectStub{calls: &projectCalls}, repo).Batch(context.Background(), []Input{
		{Query: "sqlite", Type: "decision", Tags: []string{" DB "}, MinImportance: 4, Limit: 1},
		{Query: "retry", MinConfidence: 3, Limit: 5},
		{Query: "!!!", Limit: 10},
		{Query: "invalid", Type: "unknown", Limit: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if projectCalls != 1 || len(repo.calls) != 2 {
		t.Fatalf("project calls=%d repository calls=%d", projectCalls, len(repo.calls))
	}
	if response.Total != 4 || response.Succeeded != 2 || response.Failed != 2 || len(response.Results) != 4 {
		t.Fatalf("response=%#v", response)
	}
	if response.Results[0].Index != 0 || len(response.Results[0].Response.Results) != 1 || response.Results[0].Response.Results[0].Memory.ID != "one" {
		t.Fatalf("first=%#v", response.Results[0])
	}
	if response.Results[1].Err != repoErr || response.Results[2].Response.RankingVersion != RankingVersion || response.Results[2].Response.Results == nil || memory.ErrorCode(response.Results[3].Err) != memory.CodeValidationError {
		t.Fatalf("ordered results=%#v", response.Results)
	}
	if repo.calls[0].Type != "decision" || repo.calls[0].MinImportance != 4 || repo.calls[0].Tags[0] != "db" || repo.calls[1].MinConfidence != 3 {
		t.Fatalf("filters=%#v", repo.calls)
	}
}

func TestBatchAcceptsOneAndOneHundredSearches(t *testing.T) {
	for _, size := range []int{1, 100} {
		inputs := make([]Input, size)
		for index := range inputs {
			inputs[index] = Input{Query: "!!!", Limit: 10}
		}
		response, err := NewService(projectStub{}, &repositoryStub{}).Batch(context.Background(), inputs)
		if err != nil || response.Total != size || response.Succeeded != size {
			t.Fatalf("size=%d response=%#v err=%v", size, response, err)
		}
	}
}

func TestBatchFailsGloballyOnProjectResolutionAndRanksEachWindowIndependently(t *testing.T) {
	projectErr := memory.NewError(memory.CodeStoreError, context.Canceled)
	if _, err := NewService(projectStub{err: projectErr}, &repositoryStub{}).Batch(context.Background(), []Input{{Query: "x", Limit: 10}}); err != projectErr {
		t.Fatalf("project error=%v", err)
	}

	first := memory.Memory{ID: "first", Importance: 1, Confidence: 1}
	second := memory.Memory{ID: "second", Importance: 1, Confidence: 1}
	repo := &repositoryStub{sets: [][]Candidate{
		{{Memory: first, BM25: -2}, {Memory: second, BM25: -1}},
		{{Memory: first, BM25: -1}, {Memory: second, BM25: -2}},
	}}
	response, err := NewService(projectStub{}, repo).Batch(context.Background(), []Input{{Query: "one", Limit: 10}, {Query: "two", Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Results[0].Response.Results[0].Memory.ID != "first" || response.Results[1].Response.Results[0].Memory.ID != "second" || response.Results[0].Response.Results[0].Score.Text != 1 || response.Results[1].Response.Results[0].Score.Text != 1 {
		t.Fatalf("responses=%#v", response.Results)
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
	for _, input := range []Input{{Query: "", Limit: 10}, {Query: "x", Limit: 0}, {Query: "x", Limit: 101}, {Query: "x", Type: "bad", Limit: 10}, {Query: "x", MinImportance: 6, Limit: 10}, {Query: "x", Tags: []string{"   "}, Limit: 10}, {Query: "x", SpecificTags: []string{"   "}, Limit: 10}} {
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
