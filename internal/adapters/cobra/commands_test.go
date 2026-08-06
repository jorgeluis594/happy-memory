package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

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

type searchStub struct {
	input    search.Input
	response search.Response
	err      error
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

type memoryStub struct {
	value      memory.Memory
	input      memory.CreateInput
	operations []memory.BatchOperation
	batch      memory.BatchResponse
	filter     memory.ListFilter
	tagValues  []memory.Tag
	tagQuery   string
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
func (s *memoryStub) TagsSearch(_ context.Context, query string) ([]memory.Tag, error) {
	s.tagQuery = query
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

func TestTagVocabularyCommandsUseExactJSON(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	description := "Git operations"
	service := &memoryStub{tagValues: []memory.Tag{{ID: "tag-id", Name: "Git", NormalizedName: "git", Description: &description, CreatedAt: stamp, UpdatedAt: stamp, ActiveMemoryCount: 2}}}
	output, err := ExecuteWithMemory(context.Background(), []string{"tags", "search", "operations"}, bytes.NewReader(nil), stubService{}, service)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"data\":{\"tags\":[{\"id\":\"tag-id\",\"name\":\"Git\",\"normalized_name\":\"git\",\"description\":\"Git operations\",\"created_at\":\"2026-08-05T12:00:00Z\",\"updated_at\":\"2026-08-05T12:00:00Z\",\"active_memory_count\":2}]}}\n"
	if string(output) != want || service.tagQuery != "operations" {
		t.Fatalf("output=%s query=%q", output, service.tagQuery)
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
