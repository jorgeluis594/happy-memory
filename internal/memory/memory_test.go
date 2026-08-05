package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type projectStub struct{ value ProjectContext }

func (s projectStub) Current(context.Context) (ProjectContext, error) { return s.value, nil }

type clockStub struct{ value time.Time }

func (s clockStub) Now() time.Time { return s.value }

type idsStub struct {
	values []string
	index  int
}

func (s *idsStub) New() string { value := s.values[s.index]; s.index++; return value }

type repoStub struct{ record CreateRecord }

func (r *repoStub) Create(_ context.Context, record CreateRecord) (Memory, error) {
	r.record = record
	return record.Memory, nil
}
func (*repoStub) Get(context.Context, string, string) (Memory, error)        { return Memory{}, nil }
func (*repoStub) List(context.Context, string, ListFilter) ([]Memory, error) { return nil, nil }

func TestCreateNormalizesAndBuildsInitialRevision(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 987, time.FixedZone("x", -5*60*60))
	repo := &repoStub{}
	ids := &idsStub{values: []string{"memory-id", "tag-id"}}
	service := NewService(projectStub{ProjectContext{ID: "project-id", WorktreeRoot: "/repo/worktree"}}, repo, clockStub{stamp}, ids)
	name, role := " Agent One ", " database-specialist "
	value, err := service.Create(context.Background(), CreateInput{Type: "decision", Title: "  title\r\n", Content: "body\rline  ", Importance: 5, Confidence: 4, Attributes: json.RawMessage(`{"z":1,"a":true}`), Tags: []string{" DataBase "}, Agent: &Agent{Name: &name, Role: &role}})
	if err != nil {
		t.Fatal(err)
	}
	if value.Title != "title" || value.Content != "body\nline" || string(value.Attributes) != `{"a":true,"z":1}` {
		t.Fatalf("unexpected normalized value: %#v", value)
	}
	if value.ContentHash != ContentHash("decision", "title", "body\nline") {
		t.Fatal("unexpected hash")
	}
	if !value.CreatedAt.Equal(time.Date(2026, 8, 5, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("timestamp = %v", value.CreatedAt)
	}
	if repo.record.Revision.AgentRole != "database-specialist" || repo.record.Revision.AgentName == nil || *repo.record.Revision.AgentName != "Agent One" || repo.record.Revision.WorktreeRoot != "/repo/worktree" {
		t.Fatalf("revision provenance = %#v", repo.record.Revision)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(repo.record.Revision.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["project_id"] != "project-id" || snapshot["agent_role"] != "database-specialist" {
		t.Fatalf("snapshot = %s", repo.record.Revision.Snapshot)
	}
}

func TestCreateValidation(t *testing.T) {
	base := CreateInput{Type: "fact", Title: "title", Content: "content", Importance: 1, Confidence: 1, Tags: []string{}}
	cases := []CreateInput{base, base, base, base, base}
	cases[0].Type = "other"
	cases[1].Title = " \r\n "
	cases[2].Importance = 6
	cases[3].Attributes = json.RawMessage(`[]`)
	cases[4].Tags = []string{"Tag", " tag "}
	for i, value := range cases {
		err := NormalizeAndValidate(&value)
		if ErrorCode(err) != CodeValidationError {
			t.Errorf("case %d error = %v", i, err)
		}
	}
	name := " "
	value := base
	value.Agent = &Agent{Name: &name}
	if ErrorCode(NormalizeAndValidate(&value)) != CodeValidationError {
		t.Fatal("empty agent name accepted")
	}
}

func TestMissingRoleUsesUnknown(t *testing.T) {
	repo := &repoStub{}
	ids := &idsStub{values: []string{"memory-id"}}
	service := NewService(projectStub{ProjectContext{ID: "project", WorktreeRoot: "/repo"}}, repo, clockStub{time.Now()}, ids)
	_, err := service.Create(context.Background(), CreateInput{Type: "fact", Title: "t", Content: "c", Importance: 1, Confidence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.record.Revision.AgentRole != "unknown" {
		t.Fatal(repo.record.Revision.AgentRole)
	}
}

func TestErrorDetailsDefaultEmpty(t *testing.T) {
	if len(ErrorDetails(errors.New("x"))) != 0 {
		t.Fatal("details should be empty")
	}
}
