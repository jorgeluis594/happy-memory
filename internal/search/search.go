// Package search implements ranked textual memory search.
package search

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/jorgeluis594/happy-memory/internal/memory"
)

const (
	// RankingVersion identifies the stable scoring contract.
	RankingVersion = 1
	candidateLimit = 100
)

// Input contains the query and optional search filters.
type Input struct {
	Query                        string
	Type                         string
	Tags                         []string
	MinImportance, MinConfidence int
	Limit                        int
}

// CandidateFilter is the persistence-level candidate query.
type CandidateFilter struct {
	Match                        string
	Type                         string
	Tags                         []string
	MinImportance, MinConfidence int
	Limit                        int
}

// Candidate is one BM25-ranked memory returned by the repository.
type Candidate struct {
	Memory memory.Memory
	BM25   float64
}

// Repository selects the fixed candidate window for a project.
type Repository interface {
	Candidates(context.Context, string, CandidateFilter) ([]Candidate, error)
}

// ProjectResolver identifies the current project.
type ProjectResolver interface {
	Current(context.Context) (memory.ProjectContext, error)
}

// Score exposes every component of ranking version 1.
type Score struct {
	Final, Text, Importance, Confidence float64
}

// Result is one ranked memory with its score.
type Result struct {
	Memory memory.Memory
	Score  Score
}

// Response is the complete public search result.
type Response struct {
	RankingVersion int
	Results        []Result
}

// Service coordinates candidate retrieval and deterministic ranking.
type Service struct {
	projects ProjectResolver
	repo     Repository
}

// NewService creates a ranked search service.
func NewService(projects ProjectResolver, repo Repository) *Service {
	return &Service{projects: projects, repo: repo}
}

// Search validates, retrieves the fixed window, scores it, and then applies the requested limit.
func (s *Service) Search(ctx context.Context, input Input) (Response, error) {
	filter, empty, err := prepare(input)
	if err != nil {
		return Response{}, err
	}
	if empty {
		return Response{RankingVersion: RankingVersion, Results: []Result{}}, nil
	}
	project, err := s.projects.Current(ctx)
	if err != nil {
		return Response{}, err
	}
	candidates, err := s.repo.Candidates(ctx, project.ID, filter)
	if err != nil {
		return Response{}, err
	}
	results := rank(candidates)
	if len(results) > input.Limit {
		results = results[:input.Limit]
	}
	return Response{RankingVersion: RankingVersion, Results: results}, nil
}

func prepare(input Input) (CandidateFilter, bool, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" || input.Limit < 1 || input.Limit > candidateLimit ||
		(input.Type != "" && !validType(input.Type)) || !validMinimum(input.MinImportance) || !validMinimum(input.MinConfidence) {
		return CandidateFilter{}, false, memory.NewError(memory.CodeValidationError, errors.New("invalid input"))
	}
	tags := make([]string, 0, len(input.Tags))
	seen := make(map[string]bool, len(input.Tags))
	for _, tag := range input.Tags {
		normalized := memory.NormalizeTag(tag)
		if normalized == "" {
			return CandidateFilter{}, false, memory.NewError(memory.CodeValidationError, errors.New("invalid input"))
		}
		if !seen[normalized] {
			seen[normalized] = true
			tags = append(tags, normalized)
		}
	}
	terms := strings.Fields(query)
	searchable := false
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		if strings.IndexFunc(term, unicode.IsLetter) >= 0 || strings.IndexFunc(term, unicode.IsNumber) >= 0 {
			searchable = true
		}
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	return CandidateFilter{Match: strings.Join(quoted, " AND "), Type: input.Type, Tags: tags, MinImportance: input.MinImportance, MinConfidence: input.MinConfidence, Limit: candidateLimit}, !searchable, nil
}

func rank(candidates []Candidate) []Result {
	results := make([]Result, 0, len(candidates))
	if len(candidates) == 0 {
		return results
	}
	best, worst := candidates[0].BM25, candidates[0].BM25
	for _, candidate := range candidates[1:] {
		best = min(best, candidate.BM25)
		worst = max(worst, candidate.BM25)
	}
	for _, candidate := range candidates {
		text := 1.0
		if worst != best {
			text = (worst - candidate.BM25) / (worst - best)
		}
		importance := float64(candidate.Memory.Importance-1) / 4
		confidence := float64(candidate.Memory.Confidence-1) / 4
		results = append(results, Result{Memory: candidate.Memory, Score: Score{Final: 0.70*text + 0.20*importance + 0.10*confidence, Text: text, Importance: importance, Confidence: confidence}})
	}
	slices.SortStableFunc(results, func(a, b Result) int {
		switch {
		case a.Score.Final != b.Score.Final:
			return compareDesc(a.Score.Final, b.Score.Final)
		case a.Memory.Importance != b.Memory.Importance:
			return b.Memory.Importance - a.Memory.Importance
		case a.Memory.Confidence != b.Memory.Confidence:
			return b.Memory.Confidence - a.Memory.Confidence
		case !a.Memory.UpdatedAt.Equal(b.Memory.UpdatedAt):
			if a.Memory.UpdatedAt.After(b.Memory.UpdatedAt) {
				return -1
			}
			return 1
		default:
			return strings.Compare(a.Memory.ID, b.Memory.ID)
		}
	})
	return results
}

func validMinimum(value int) bool { return value == 0 || value >= 1 && value <= 5 }
func validType(value string) bool {
	return value == "fact" || value == "decision" || value == "constraint" || value == "preference" || value == "procedure" || value == "lesson"
}
func compareDesc(a, b float64) int {
	if a > b {
		return -1
	}
	return 1
}
