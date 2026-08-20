package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/agentconfig"
	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
	"github.com/jorgeluis594/happy-memory/internal/search"
)

type diagnosticStub struct {
	report diagnostic.Report
	err    error
}

func (stub diagnosticStub) Doctor(context.Context) (diagnostic.Report, error) {
	return stub.report, stub.err
}

func TestDoctorUsesExactJSONAndHelpIsValidationError(t *testing.T) {
	report := diagnostic.Report{Healthy: true, DatabasePath: "/db", Checks: []diagnostic.Check{}}
	output, err := ExecuteAll(context.Background(), []string{"doctor"}, nil, nil, nil, nil, diagnosticStub{report: report})
	if err != nil || string(output) != "{\"ok\":true,\"data\":{\"healthy\":true,\"database_path\":\"/db\",\"checks\":[]}}\n" {
		t.Fatalf("output=%s err=%v", output, err)
	}
	if output, err = Execute(context.Background(), []string{"--help"}, stubService{}); project.Code(err) != project.CodeValidationError || len(output) != 0 {
		t.Fatalf("help output=%q err=%v", output, err)
	}
}

type stubService struct{ value project.Project }

func (stub stubService) Initialize(context.Context, *string) (project.Project, error) {
	return stub.value, nil
}
func (stub stubService) InitializeWithAgents(_ context.Context, _ *string, agents []agentconfig.Agent) (project.Project, []agentconfig.Result, error) {
	results := make([]agentconfig.Result, 0, len(agents))
	for _, agent := range agents {
		results = append(results, agentconfig.Result{Agent: agent, Status: agentconfig.StatusConfigured, ConfigPath: "/config", SharedMemoryPath: "/shared"})
	}
	return stub.value, results, nil
}

func TestInitConfiguresCommaSeparatedAgentsAndRejectsInvalidLists(t *testing.T) {
	value := project.Project{ID: "p1", Name: "project", LastKnownGitCommonDir: "/repo/.git", CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}
	output, err := Execute(context.Background(), []string{"init", "--configure-agent", "codex, claude-code,codex"}, stubService{value: value})
	if err != nil || !strings.Contains(string(output), `"agent_configurations":[{"agent":"codex"`) || strings.Count(string(output), `"agent":"codex"`) != 1 || !strings.Contains(string(output), `"agent":"claude-code"`) {
		t.Fatalf("output=%s err=%v", output, err)
	}
	for _, input := range []string{"", "codex,", "unknown", "codex,,opencode"} {
		if _, err = Execute(context.Background(), []string{"init", "--configure-agent", input}, stubService{value: value}); project.Code(err) != project.CodeValidationError {
			t.Fatalf("input=%q err=%v", input, err)
		}
	}
}

type searchStub struct {
	input         search.Input
	batchInputs   []search.Input
	response      search.Response
	batchResponse search.BatchResponse
	err           error
}

func (s *searchStub) Batch(_ context.Context, inputs []search.Input) (search.BatchResponse, error) {
	s.batchInputs = inputs
	return s.batchResponse, s.err
}

func (s *searchStub) Search(_ context.Context, input search.Input) (search.Response, error) {
	s.input = input
	return s.response, s.err
}

func TestSearchCommandDefaultsFlagsAndExactJSON(t *testing.T) {
	service := &searchStub{response: search.Response{RankingVersion: 1, Results: []search.Result{{Memory: memory.Memory{ID: "m1", Type: "decision", Title: "Git", Content: "Use worktrees", Importance: 5, Confidence: 4, Tags: []memory.Tag{{Name: "git"}, {Name: "worktrees"}}}, Score: search.Score{Final: 0.9, Text: 1, Importance: 1, Confidence: 0.75}}}}}
	output, err := ExecuteWithServices(context.Background(), []string{"search", "git worktrees", "--type", "decision", "--tag", "Git", "--tag", "Worktrees", "--min-importance", "4", "--min-confidence", "3"}, bytes.NewReader(nil), stubService{}, &memoryStub{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ok":true,"data":{"ranking_version":1,"results":[{"id":"m1","type":"decision","title":"Git","content":"Use worktrees","importance":5,"confidence":4,"tags":["git","worktrees"],"score":{"final":0.9,"text":1,"importance":1,"confidence":0.75}}]}}` + "\n"
	if string(output) != want {
		t.Fatalf("output = %s", output)
	}
	if service.input.Limit != 10 || service.input.Type != "decision" || len(service.input.Tags) != 2 || service.input.MinImportance != 4 || service.input.MinConfidence != 3 {
		t.Fatalf("input = %#v", service.input)
	}
}

func TestSearchCommandRequiresOneQuery(t *testing.T) {
	for _, args := range [][]string{{"search"}, {"search", "one", "two"}} {
		if _, err := ExecuteWithServices(context.Background(), args, bytes.NewReader(nil), stubService{}, &memoryStub{}, &searchStub{}); err == nil {
			t.Fatalf("expected validation for %v", args)
		}
	}
}

func TestSearchBatchUsesStrictJSONDefaultsAndOrderedItemErrors(t *testing.T) {
	item := search.Response{RankingVersion: 1, Results: []search.Result{}}
	service := &searchStub{batchResponse: search.BatchResponse{Total: 3, Succeeded: 2, Failed: 1, Results: []search.BatchResult{
		{Index: 0, Response: item},
		{Index: 1, Err: memory.NewError(memory.CodeValidationError, errors.New("invalid input"))},
		{Index: 2, Response: item},
	}}}
	body := `{"searches":[{"query":"sqlite"},{"query":"worktrees","type":"decision","tags":["git"],"specific_tags":["Orca","Codex"],"min_importance":4,"min_confidence":3,"limit":5},{"query":"!!!"}]}`
	output, err := ExecuteWithServices(context.Background(), []string{"search", "--input", "-"}, bytes.NewBufferString(body), stubService{}, &memoryStub{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"summary\":{\"total\":3,\"succeeded\":2,\"failed\":1},\"results\":[{\"index\":0,\"ok\":true,\"data\":{\"ranking_version\":1,\"results\":[]}},{\"index\":1,\"ok\":false,\"error\":{\"code\":\"VALIDATION_ERROR\",\"message\":\"invalid input\",\"details\":{}}},{\"index\":2,\"ok\":true,\"data\":{\"ranking_version\":1,\"results\":[]}}]}}\n"
	if string(output) != want {
		t.Fatalf("output=%s", output)
	}
	if len(service.batchInputs) != 3 || service.batchInputs[0].Limit != 10 || service.batchInputs[1].Limit != 5 || service.batchInputs[1].Type != "decision" || service.batchInputs[1].MinImportance != 4 || service.batchInputs[1].MinConfidence != 3 || len(service.batchInputs[1].Tags) != 1 || !slices.Equal(service.batchInputs[1].SpecificTags, []string{"Orca", "Codex"}) {
		t.Fatalf("inputs=%#v", service.batchInputs)
	}
}

func TestSearchCommandParsesSpecificTagsCSV(t *testing.T) {
	service := &searchStub{response: search.Response{RankingVersion: 1, Results: []search.Result{}}}
	if _, err := ExecuteWithServices(context.Background(), []string{"search", "query", "--specific-tags", "Orca,Codex"}, nil, stubService{}, &memoryStub{}, service); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(service.input.SpecificTags, []string{"Orca", "Codex"}) {
		t.Fatalf("specific tags=%v", service.input.SpecificTags)
	}
}

func TestSearchBatchRejectsInvalidWireAndIncompatibleArguments(t *testing.T) {
	invalidBodies := []string{
		`{"searches":[],"unknown":true}`,
		`{"searches":[{"query":"x","unknown":true}]}`,
		`{"searches":[]} trailing`,
		`[]`,
	}
	for _, body := range invalidBodies {
		if _, err := ExecuteWithServices(context.Background(), []string{"search", "--input", "-"}, bytes.NewBufferString(body), stubService{}, &memoryStub{}, &searchStub{}); project.Code(err) != project.CodeValidationError {
			t.Fatalf("accepted body %q: %v", body, err)
		}
	}
	for _, args := range [][]string{
		{"search", "query", "--input", "-"},
		{"search", "--input", "file.json"},
		{"search", "--input", "-", "--limit", "5"},
		{"search", "--input", "-", "--type", "fact"},
	} {
		if _, err := ExecuteWithServices(context.Background(), args, bytes.NewBufferString(`{"searches":[{"query":"x"}]}`), stubService{}, &memoryStub{}, &searchStub{}); project.Code(err) != project.CodeValidationError {
			t.Fatalf("accepted args %v: %v", args, err)
		}
	}
}

type memoryStub struct {
	value      memory.Memory
	input      memory.CreateInput
	operations []memory.BatchOperation
	batch      memory.BatchResponse
	filter     memory.ListFilter
	tagValues  []memory.Tag
	tagQuery   string
	tagLimit   int
	err        error
}

func (s *memoryStub) Batch(_ context.Context, operations []memory.BatchOperation) (memory.BatchResponse, error) {
	s.operations = operations
	return s.batch, s.err
}

func (s *memoryStub) Create(_ context.Context, input memory.CreateInput) (memory.Memory, error) {
	s.input = input
	return s.value, s.err
}
func (s *memoryStub) Update(_ context.Context, _ string, _ int, _ memory.UpdateInput) (memory.Memory, error) {
	return s.value, s.err
}
func (s *memoryStub) Delete(context.Context, string, int) (memory.Memory, error) {
	return s.value, s.err
}
func (s *memoryStub) Restore(context.Context, string, int, int) (memory.Memory, error) {
	return s.value, s.err
}
func (s *memoryStub) Get(context.Context, string, bool) (memory.Memory, error) { return s.value, s.err }
func (s *memoryStub) List(_ context.Context, filter memory.ListFilter) ([]memory.Memory, error) {
	s.filter = filter
	return []memory.Memory{s.value}, s.err
}
func (s *memoryStub) History(context.Context, string) ([]memory.Revision, error) { return nil, s.err }
func (s *memoryStub) TagsList(context.Context) ([]memory.Tag, error) {
	return s.tagValues, s.err
}
func (s *memoryStub) TagsSearch(_ context.Context, query string, limit int) ([]memory.Tag, error) {
	s.tagQuery = query
	s.tagLimit = limit
	return s.tagValues, s.err
}
func (stub stubService) ShowCurrent(context.Context) (project.Project, error) { return stub.value, nil }
func (stub stubService) List(context.Context) ([]project.Project, error) {
	return []project.Project{stub.value}, nil
}

func TestMemoryCreateAcceptsBothTagInputFormats(t *testing.T) {
	service := &memoryStub{value: memory.Memory{Tags: []memory.Tag{}}}
	input := bytes.NewBufferString(`{"type":"fact","title":"title","content":"content","importance":4,"confidence":3,"tags":["git",{"name":"database","description":"Storage"}]}`)
	if _, err := ExecuteWithMemory(context.Background(), []string{"create", "--input", "-"}, input, stubService{}, service); err != nil {
		t.Fatal(err)
	}
	if len(service.input.Tags) != 2 || service.input.Tags[0].Name != "git" || service.input.Tags[1].Description == nil || *service.input.Tags[1].Description != "Storage" {
		t.Fatalf("input tags=%#v", service.input.Tags)
	}
}

func TestBatchCommandUsesStrictSchemaAndReturnsOrderedItemErrors(t *testing.T) {
	stamp := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	value := memory.Memory{ID: "memory-id", Version: 1, Type: "fact", Title: "title", Content: "content", Importance: 4, Confidence: 3, Tags: []memory.Tag{}, ContentHash: "hash", CreatedAt: stamp, UpdatedAt: stamp}
	service := &memoryStub{batch: memory.BatchResponse{Total: 2, Succeeded: 1, Failed: 1, Results: []memory.BatchResult{{Index: 0, Operation: "create", Memory: value}, {Index: 1, Operation: "delete", Err: memory.NewError(memory.CodeNotFound, errors.New("missing"))}}}}
	body := `{"operations":[{"operation":"create","input":{"type":"fact","title":"title","content":"content","importance":4,"confidence":3,"tags":[]}},{"operation":"delete","memory_id":"28fef1e4-42c5-43ca-a0c8-0c731797c06f","expected_version":1}]}`
	output, err := ExecuteWithMemory(context.Background(), []string{"batch", "--input", "-"}, bytes.NewBufferString(body), stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"summary\":{\"total\":2,\"succeeded\":1,\"failed\":1},\"results\":[{\"index\":0,\"operation\":\"create\",\"ok\":true,\"data\":{\"id\":\"memory-id\",\"version\":1,\"type\":\"fact\",\"title\":\"title\",\"content\":\"content\",\"importance\":4,\"confidence\":3,\"attributes\":null,\"tags\":[],\"content_hash\":\"hash\",\"created_at\":\"2026-08-06T12:00:00Z\",\"updated_at\":\"2026-08-06T12:00:00Z\",\"deleted_at\":null}},{\"index\":1,\"operation\":\"delete\",\"ok\":false,\"error\":{\"code\":\"MEMORY_NOT_FOUND\",\"message\":\"memory not found\",\"details\":{}}}]}}\n"
	if string(output) != want || len(service.operations) != 2 || service.operations[0].CreateInput == nil {
		t.Fatalf("output=%s operations=%#v", output, service.operations)
	}
	for _, invalid := range []string{
		`{"operations":[{"operation":"create","input":{},"extra":true}]}`,
		`{"operations":[{"operation":"restore"}]}`,
		`{"operations":[{"operation":"delete","memory_id":"x"}]}`,
		`{"operations":[]} trailing`,
		`{"operations":[],"extra":true}`,
	} {
		if _, err := ExecuteWithMemory(context.Background(), []string{"batch", "--input", "-"}, bytes.NewBufferString(invalid), stubService{}, service); project.Code(err) != project.CodeValidationError {
			t.Fatalf("accepted %s: %v", invalid, err)
		}
	}
}

func TestBatchCommandIncludesStoreErrorRootCause(t *testing.T) {
	storeErr := memory.NewError(memory.CodeStoreError, fmt.Errorf("storage operation failed: %w", errors.New("constraint failed")))
	service := &memoryStub{batch: memory.BatchResponse{Total: 1, Failed: 1, Results: []memory.BatchResult{{Index: 0, Operation: "delete", Err: storeErr}}}}
	body := `{"operations":[{"operation":"delete","memory_id":"28fef1e4-42c5-43ca-a0c8-0c731797c06f","expected_version":1}]}`

	output, err := ExecuteWithMemory(context.Background(), []string{"batch", "--input", "-"}, bytes.NewBufferString(body), stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"summary\":{\"total\":1,\"succeeded\":0,\"failed\":1},\"results\":[{\"index\":0,\"operation\":\"delete\",\"ok\":false,\"error\":{\"code\":\"STORE_ERROR\",\"message\":\"storage operation failed\",\"details\":{\"cause\":\"constraint failed\"}}}]}}\n"
	if string(output) != want {
		t.Fatalf("output=%s", output)
	}
}

func TestTagVocabularyCommandsUseExactJSON(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	description := "Git operations"
	service := &memoryStub{tagValues: []memory.Tag{{ID: "tag-id", Name: "Git", NormalizedName: "git", Description: &description, CreatedAt: stamp, UpdatedAt: stamp, ActiveMemoryCount: 2}}}
	output, err := ExecuteWithMemory(context.Background(), []string{"tags", "search", "operations"}, bytes.NewReader(nil), stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"tags\":[{\"id\":\"tag-id\",\"name\":\"Git\",\"normalized_name\":\"git\",\"description\":\"Git operations\",\"created_at\":\"2026-08-05T12:00:00Z\",\"updated_at\":\"2026-08-05T12:00:00Z\",\"active_memory_count\":2}]}}\n"
	if string(output) != want || service.tagQuery != "operations" || service.tagLimit != 10 {
		t.Fatalf("output=%s query=%q limit=%d", output, service.tagQuery, service.tagLimit)
	}
	if _, err = ExecuteWithMemory(context.Background(), []string{"tags", "search", "operations", "--limit", "3"}, bytes.NewReader(nil), stubService{}, service); err != nil || service.tagLimit != 3 {
		t.Fatalf("explicit limit=%d err=%v", service.tagLimit, err)
	}
	if _, err = ExecuteWithMemory(context.Background(), []string{"tags", "search", "operations", "--limit", "many"}, bytes.NewReader(nil), stubService{}, service); project.Code(err) != project.CodeValidationError {
		t.Fatalf("non-integer limit error=%v", err)
	}
	output, err = ExecuteWithMemory(context.Background(), []string{"tags", "list"}, bytes.NewReader(nil), stubService{}, &memoryStub{})
	if err != nil || string(output) != "{\"ok\":true,\"data\":{\"tags\":[]}}\n" {
		t.Fatalf("empty list=%s err=%v", output, err)
	}
}

func TestMemoryCommandsUseStrictInputAndExactJSON(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	service := &memoryStub{value: memory.Memory{ID: "memory-id", Version: 1, Type: "fact", Title: "title", Content: "content", Importance: 4, Confidence: 3, Attributes: json.RawMessage(`{"key":"value"}`), Tags: []memory.Tag{}, ContentHash: "hash", CreatedAt: stamp, UpdatedAt: stamp}}
	input := bytes.NewBufferString(`{"type":"fact","title":"title","content":"content","importance":4,"confidence":3,"attributes":{"key":"value"},"tags":[]}`)
	output, err := ExecuteWithMemory(context.Background(), []string{"create", "--input", "-"}, input, stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"id\":\"memory-id\",\"version\":1,\"type\":\"fact\",\"title\":\"title\",\"content\":\"content\",\"importance\":4,\"confidence\":3,\"attributes\":{\"key\":\"value\"},\"tags\":[],\"content_hash\":\"hash\",\"created_at\":\"2026-08-05T12:00:00Z\",\"updated_at\":\"2026-08-05T12:00:00Z\",\"deleted_at\":null}}\n"
	if string(output) != want {
		t.Fatalf("output=%s want=%s", output, want)
	}
	for _, body := range []string{`{"type":"fact","unknown":true}`, `{} {}`, `[]`} {
		_, err = ExecuteWithMemory(context.Background(), []string{"create", "--input", "-"}, bytes.NewBufferString(body), stubService{}, service)
		if project.Code(err) != project.CodeValidationError {
			t.Errorf("body %q error=%v", body, err)
		}
	}
}

func TestMemoryListPassesRepeatedTagFilters(t *testing.T) {
	service := &memoryStub{}
	_, err := ExecuteWithMemory(context.Background(), []string{"list", "--tag", "Go", "--tag", "Database", "--min-importance", "4", "--min-confidence", "3"}, bytes.NewReader(nil), stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	if len(service.filter.Tags) != 2 || service.filter.MinImportance != 4 || service.filter.MinConfidence != 3 {
		t.Fatalf("filter=%#v", service.filter)
	}
}

func TestExecuteExactJSONAndValidation(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	service := stubService{value: project.Project{ID: "id", Name: "name", LastKnownGitCommonDir: "/repo/.git", CreatedAt: stamp, UpdatedAt: stamp}}
	output, err := Execute(context.Background(), []string{"project", "show"}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"id\":\"id\",\"name\":\"name\",\"last_known_git_common_dir\":\"/repo/.git\",\"created_at\":\"2026-08-05T12:00:00Z\",\"updated_at\":\"2026-08-05T12:00:00Z\"}}\n"
	if string(output) != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
	for _, args := range [][]string{{}, {"unknown"}, {"init", "extra"}, {"init", "--project-id", "x"}} {
		_, err = Execute(context.Background(), args, service)
		var domainError *project.Error
		if !errors.As(err, &domainError) || domainError.Code != project.CodeValidationError {
			t.Fatalf("args %v: error = %v", args, err)
		}
	}
}

func TestVersionUsesDevelopmentMetadata(t *testing.T) {
	t.Parallel()
	output, err := Execute(context.Background(), []string{"version"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"version\":\"dev\",\"commit\":\"unknown\",\"build_date\":\"unknown\"}}\n"
	if string(output) != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}
