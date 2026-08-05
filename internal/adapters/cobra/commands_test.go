package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

type stubService struct{ value project.Project }

func (stub stubService) Initialize(context.Context, *string) (project.Project, error) {
	return stub.value, nil
}

type memoryStub struct {
	value  memory.Memory
	input  memory.CreateInput
	filter memory.ListFilter
	err    error
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
func (stub stubService) ShowCurrent(context.Context) (project.Project, error)    { return stub.value, nil }
func (stub stubService) List(context.Context) ([]project.Project, error) {
	return []project.Project{stub.value}, nil
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
