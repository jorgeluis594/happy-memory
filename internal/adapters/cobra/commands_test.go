package cobra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/project"
)

type stubService struct{ value project.Project }

func (stub stubService) Initialize(context.Context, *string) (project.Project, error) {
	return stub.value, nil
}
func (stub stubService) ShowCurrent(context.Context) (project.Project, error) { return stub.value, nil }
func (stub stubService) List(context.Context) ([]project.Project, error) {
	return []project.Project{stub.value}, nil
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
