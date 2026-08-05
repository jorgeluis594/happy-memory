// Package app composes and runs the command-line application.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/google/uuid"
	commandadapter "github.com/jorgeluis594/happy-memory/internal/adapters/cobra"
	gitadapter "github.com/jorgeluis594/happy-memory/internal/adapters/git"
	"github.com/jorgeluis594/happy-memory/internal/adapters/sqlite"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) New() string { return uuid.NewString() }

type currentProjectResolver struct {
	projects *project.Service
	git      *gitadapter.Adapter
}

func (resolver currentProjectResolver) Current(ctx context.Context) (memory.ProjectContext, error) {
	current, err := resolver.projects.ShowCurrent(ctx)
	if err != nil {
		return memory.ProjectContext{}, err
	}
	gitContext, err := resolver.git.Resolve(ctx)
	if err != nil {
		return memory.ProjectContext{}, err
	}
	return memory.ProjectContext{ID: current.ID, WorktreeRoot: gitContext.WorktreeRoot}, nil
}

// Run composes and executes one CLI invocation.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return RunWithInput(ctx, args, os.Stdin, stdout, stderr)
}

// RunWithInput composes one invocation with an explicit stdin stream.
func RunWithInput(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	database, err := sqlite.Open(ctx)
	if err != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	migrations, subErr := fs.Sub(sqlite.EmbeddedMigrations, "migrations")
	if subErr != nil {
		_ = database.Close()
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if err := sqlite.Migrate(ctx, database.SQL(), migrations); err != nil {
		_ = database.Close()
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	git := gitadapter.New()
	projectService := project.NewService(git, sqlite.NewProjectRepository(database.GORM()), systemClock{}, uuidGenerator{})
	memoryService := memory.NewService(currentProjectResolver{projects: projectService, git: git}, sqlite.NewMemoryRepository(database.GORM()), systemClock{}, uuidGenerator{})
	response, executeErr := commandadapter.ExecuteWithMemory(ctx, args, stdin, projectService, memoryService)
	closeErr := database.Close()
	if executeErr != nil {
		return writeFailure(stderr, executeErr)
	}
	if closeErr != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if _, err := stdout.Write(response); err != nil {
		return 1
	}
	return 0
}

func writeFailure(output io.Writer, err error) int {
	code := project.Code(err)
	details := map[string]any{}
	var memoryError *memory.Error
	if errors.As(err, &memoryError) {
		code = memoryError.Code
		details = memory.ErrorDetails(err)
	}
	message := map[string]string{
		project.CodeGitRepositoryNotFound: "git repository not found",
		project.CodeProjectNotInitialized: "project is not initialized",
		project.CodeValidationError:       "invalid input",
		project.CodeStoreError:            "storage operation failed",
		memory.CodeNotFound:               "memory not found",
		memory.CodeDuplicate:              "duplicate memory",
		memory.CodeVersionConflict:        "version conflict",
	}[code]
	if message == "" {
		code, message = project.CodeStoreError, "storage operation failed"
	}
	type errorBody struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	}
	type failure struct {
		OK    bool      `json:"ok"`
		Error errorBody `json:"error"`
	}
	payload := failure{OK: false, Error: errorBody{Code: code, Message: message, Details: details}}
	_ = json.NewEncoder(output).Encode(payload)
	return 1
}
