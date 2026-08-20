// Package app composes and runs the command-line application.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	agentconfigadapter "github.com/jorgeluis594/happy-memory/internal/adapters/agentconfig"
	commandadapter "github.com/jorgeluis594/happy-memory/internal/adapters/cobra"
	gitadapter "github.com/jorgeluis594/happy-memory/internal/adapters/git"
	"github.com/jorgeluis594/happy-memory/internal/adapters/sqlite"
	"github.com/jorgeluis594/happy-memory/internal/agentconfig"
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
	projects   *project.Service
	gitContext project.GitContext
}

func (resolver currentProjectResolver) Current(ctx context.Context) (memory.ProjectContext, error) {
	current, err := resolver.projects.ShowCurrent(ctx, resolver.gitContext)
	if err != nil {
		return memory.ProjectContext{}, err
	}
	return memory.ProjectContext{ID: current.ID, WorktreeRoot: resolver.gitContext.WorktreeRoot}, nil
}

type gitAdapter interface {
	project.GitIdentity
	Resolve(context.Context) (project.GitContext, error)
}

// runtime defers all Git and SQLite work until a validated Cobra handler calls
// a service method. This keeps invalid commands from creating local storage.
type runtime struct {
	git        gitAdapter
	gitContext project.GitContext
	migrations fs.FS
	database   *sqlite.Database
	projects   *project.Service
	memories   *memory.Service
	searches   *search.Service
	agents     *agentconfig.Service
}

func newRuntime(migrations fs.FS) *runtime {
	return &runtime{git: gitadapter.New(), migrations: migrations, agents: agentconfig.NewService(agentconfigadapter.New())}
}

func (r *runtime) open(ctx context.Context, create bool) error {
	if r.database != nil {
		return nil
	}
	gitContext, err := r.git.Resolve(ctx)
	if err != nil {
		return err
	}
	path := sqlite.PathFromGitCommonDir(gitContext.CommonDir)
	if err = sqlite.ValidateExistingSchema(ctx, path, r.migrations); err != nil {
		return storeError()
	}
	if create {
		r.database, err = sqlite.OpenOrCreate(ctx, path)
	} else {
		r.database, err = sqlite.OpenExisting(ctx, path)
		if errors.Is(err, os.ErrNotExist) {
			return project.NewError(project.CodeProjectNotInitialized, errors.New("project is not initialized"))
		}
	}
	if err != nil {
		return storeError()
	}
	if err = sqlite.Migrate(ctx, r.database.SQL(), r.migrations); err != nil {
		_ = r.database.Close()
		r.database = nil
		return storeError()
	}
	r.gitContext = gitContext
	r.projects = project.NewService(r.git, sqlite.NewProjectRepository(r.database.GORM()), systemClock{}, uuidGenerator{})
	resolver := currentProjectResolver{projects: r.projects, gitContext: gitContext}
	r.memories = memory.NewService(resolver, sqlite.NewMemoryRepository(r.database.GORM()), systemClock{}, uuidGenerator{})
	r.searches = search.NewService(resolver, sqlite.NewSearchRepository(r.database.GORM()))
	return nil
}

func (r *runtime) Initialize(ctx context.Context, name *string) (project.Project, error) {
	if name != nil && strings.TrimSpace(*name) == "" {
		return project.Project{}, project.NewError(project.CodeValidationError, errors.New("invalid input"))
	}
	if err := r.open(ctx, true); err != nil {
		return project.Project{}, err
	}
	return r.projects.Initialize(ctx, r.gitContext, name)
}

// InitializeWithAgents initializes project memory and then attempts every requested agent configuration.
func (r *runtime) InitializeWithAgents(ctx context.Context, name *string, agents []agentconfig.Agent) (project.Project, []agentconfig.Result, error) {
	value, err := r.Initialize(ctx, name)
	if err != nil {
		return project.Project{}, nil, err
	}
	sharedMemoryPath := filepath.Dir(sqlite.PathFromGitCommonDir(r.gitContext.CommonDir))
	return value, r.agents.Configure(ctx, agents, sharedMemoryPath), nil
}

func (r *runtime) ShowCurrent(ctx context.Context) (project.Project, error) {
	if err := r.open(ctx, false); err != nil {
		return project.Project{}, err
	}
	return r.projects.ShowCurrent(ctx, r.gitContext)
}

func (r *runtime) List(ctx context.Context) ([]project.Project, error) {
	if err := r.open(ctx, false); err != nil {
		return nil, err
	}
	return r.projects.List(ctx)
}

func (r *runtime) Batch(ctx context.Context, operations []memory.BatchOperation) (memory.BatchResponse, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.BatchResponse{}, err
	}
	return r.memories.Batch(ctx, operations)
}

func (r *runtime) Create(ctx context.Context, input memory.CreateInput) (memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.Memory{}, err
	}
	return r.memories.Create(ctx, input)
}

func (r *runtime) Update(ctx context.Context, id string, expected int, input memory.UpdateInput) (memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.Memory{}, err
	}
	return r.memories.Update(ctx, id, expected, input)
}

func (r *runtime) Delete(ctx context.Context, id string, expected int) (memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.Memory{}, err
	}
	return r.memories.Delete(ctx, id, expected)
}

func (r *runtime) Restore(ctx context.Context, id string, version, expected int) (memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.Memory{}, err
	}
	return r.memories.Restore(ctx, id, version, expected)
}

func (r *runtime) Get(ctx context.Context, id string, includeDeleted bool) (memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return memory.Memory{}, err
	}
	return r.memories.Get(ctx, id, includeDeleted)
}

func (r *runtime) ListMemories(ctx context.Context, filter memory.ListFilter) ([]memory.Memory, error) {
	if err := r.open(ctx, false); err != nil {
		return nil, err
	}
	return r.memories.List(ctx, filter)
}

func (r *runtime) History(ctx context.Context, id string) ([]memory.Revision, error) {
	if err := r.open(ctx, false); err != nil {
		return nil, err
	}
	return r.memories.History(ctx, id)
}

func (r *runtime) TagsList(ctx context.Context) ([]memory.Tag, error) {
	if err := r.open(ctx, false); err != nil {
		return nil, err
	}
	return r.memories.TagsList(ctx)
}

func (r *runtime) TagsSearch(ctx context.Context, query string, limit int) ([]memory.Tag, error) {
	if err := r.open(ctx, false); err != nil {
		return nil, err
	}
	return r.memories.TagsSearch(ctx, query, limit)
}

func (r *runtime) Search(ctx context.Context, input search.Input) (search.Response, error) {
	if err := r.open(ctx, false); err != nil {
		return search.Response{}, err
	}
	return r.searches.Search(ctx, input)
}

type searchRuntime struct{ *runtime }

func (r searchRuntime) Batch(ctx context.Context, inputs []search.Input) (search.BatchResponse, error) {
	if err := r.open(ctx, false); err != nil {
		return search.BatchResponse{}, err
	}
	return r.searches.Batch(ctx, inputs)
}

func (r *runtime) Close() error {
	if r.database == nil {
		return nil
	}
	return r.database.Close()
}

type memoryRuntime struct{ *runtime }

func (r memoryRuntime) List(ctx context.Context, filter memory.ListFilter) ([]memory.Memory, error) {
	return r.ListMemories(ctx, filter)
}

// Run composes and executes one CLI invocation.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return RunWithInput(ctx, args, os.Stdin, stdout, stderr)
}

// RunWithInput composes one invocation with an explicit stdin stream.
func RunWithInput(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	migrations, err := fs.Sub(sqlite.EmbeddedMigrations, "migrations")
	if err != nil {
		return writeFailure(stderr, storeError())
	}
	if len(args) > 0 && args[0] == "doctor" {
		path := sqlite.DiagnosticFallbackPath()
		gitContext, resolveErr := gitadapter.New().Resolve(ctx)
		if resolveErr == nil {
			path = sqlite.PathFromGitCommonDir(gitContext.CommonDir)
		}
		diagnostics := diagnostic.NewService(path, sqlite.NewDiagnosticChecker(migrations))
		response, executeErr := commandadapter.ExecuteAll(ctx, args, stdin, nil, nil, nil, diagnostics)
		return writeResult(stdout, stderr, response, executeErr, nil)
	}
	runtime := newRuntime(migrations)
	response, executeErr := commandadapter.ExecuteAll(ctx, args, stdin, runtime, memoryRuntime{runtime}, searchRuntime{runtime}, nil)
	return writeResult(stdout, stderr, response, executeErr, runtime.Close)
}

func writeResult(stdout, stderr io.Writer, response []byte, executeErr error, closeStore func() error) int {
	if closeStore != nil {
		closeErr := closeStore()
		if executeErr == nil && closeErr != nil {
			executeErr = storeError()
		}
	}
	if executeErr != nil {
		return writeFailure(stderr, executeErr)
	}
	if _, err := stdout.Write(response); err != nil {
		return 1
	}
	return 0
}

func storeError() error {
	return project.NewError(project.CodeStoreError, errors.New("storage operation failed"))
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
