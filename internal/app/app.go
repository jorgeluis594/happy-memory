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
	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
	"github.com/jorgeluis594/happy-memory/internal/search"
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
	path, err := sqlite.ResolvePath()
	if err != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	migrations, subErr := fs.Sub(sqlite.EmbeddedMigrations, "migrations")
	if subErr != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if len(args) > 0 && args[0] == "doctor" {
		diagnostics := diagnostic.NewService(path, sqlite.NewDiagnosticChecker(migrations))
		response, executeErr := commandadapter.ExecuteAll(ctx, args, stdin, nil, nil, nil, diagnostics)
		if executeErr != nil {
			return writeFailure(stderr, executeErr)
		}
		if _, err = stdout.Write(response); err != nil {
			return 1
		}
		return 0
	}
	if err = sqlite.ValidateExistingSchema(ctx, path, migrations); err != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	database, err := sqlite.Open(ctx)
	if err != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if err := sqlite.Migrate(ctx, database.SQL(), migrations); err != nil {
		_ = database.Close()
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	git := gitadapter.New()
	projectService := project.NewService(git, sqlite.NewProjectRepository(database.GORM()), systemClock{}, uuidGenerator{})
	memoryService := memory.NewService(currentProjectResolver{projects: projectService, git: git}, sqlite.NewMemoryRepository(database.GORM()), systemClock{}, uuidGenerator{})
	searchService := search.NewService(currentProjectResolver{projects: projectService, git: git}, sqlite.NewSearchRepository(database.GORM()))
	response, executeErr := commandadapter.ExecuteAll(ctx, args, stdin, projectService, memoryService, searchService, nil)
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
	type failure struct {
		OK    bool        `json:"ok"`
		Error PublicError `json:"error"`
	}
	payload := failure{OK: false, Error: publicError(err)}
	_ = json.NewEncoder(output).Encode(payload)
	return 1
}
