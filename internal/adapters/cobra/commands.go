// Package cobra adapts project use cases to the CLI contract.
package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
	spf13cobra "github.com/spf13/cobra"
)

type projectService interface {
	Initialize(context.Context, *string) (project.Project, error)
	ShowCurrent(context.Context) (project.Project, error)
	List(context.Context) ([]project.Project, error)
}

type memoryService interface {
	Create(context.Context, memory.CreateInput) (memory.Memory, error)
	Update(context.Context, string, int, memory.UpdateInput) (memory.Memory, error)
	Delete(context.Context, string, int) (memory.Memory, error)
	Restore(context.Context, string, int, int) (memory.Memory, error)
	Get(context.Context, string, bool) (memory.Memory, error)
	List(context.Context, memory.ListFilter) ([]memory.Memory, error)
	History(context.Context, string) ([]memory.Revision, error)
	TagsList(context.Context) ([]memory.Tag, error)
	TagsSearch(context.Context, string) ([]memory.Tag, error)
}

// Execute parses args and returns a buffered successful response.
func Execute(ctx context.Context, args []string, service projectService) ([]byte, error) {
	return execute(ctx, args, nil, service, nil)
}

// ExecuteWithMemory parses all project and memory commands, including stdin input.
func ExecuteWithMemory(ctx context.Context, args []string, input io.Reader, projects projectService, memories memoryService) ([]byte, error) {
	return execute(ctx, args, input, projects, memories)
}

func execute(ctx context.Context, args []string, input io.Reader, service projectService, memories memoryService) ([]byte, error) {
	var output bytes.Buffer
	root := &spf13cobra.Command{Use: "happy-memory", SilenceUsage: true, SilenceErrors: true, Args: invalidArgs, RunE: invalidCommand}
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)

	var name string
	initCommand := &spf13cobra.Command{Use: "init", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		var requested *string
		if command.Flags().Changed("name") {
			requested = &name
		}
		value, err := service.Initialize(command.Context(), requested)
		if err != nil {
			return err
		}
		return writeJSON(&output, successProject(value))
	}}
	initCommand.Flags().StringVar(&name, "name", "", "project name")

	projectCommand := &spf13cobra.Command{Use: "project", Args: invalidArgs, RunE: invalidCommand}
	projectCommand.AddCommand(&spf13cobra.Command{Use: "show", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		value, err := service.ShowCurrent(command.Context())
		if err != nil {
			return err
		}
		return writeJSON(&output, successProject(value))
	}})
	projectsCommand := &spf13cobra.Command{Use: "projects", Args: invalidArgs, RunE: invalidCommand}
	projectsCommand.AddCommand(&spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		values, err := service.List(command.Context())
		if err != nil {
			return err
		}
		items := make([]projectJSON, 0, len(values))
		for _, value := range values {
			items = append(items, toJSON(value))
		}
		return writeJSON(&output, listSuccess{OK: true, Data: projectList{Projects: items}})
	}})
	root.AddCommand(initCommand, projectCommand, projectsCommand)
	if memories != nil {
		addMemoryCommands(root, &output, input, memories)
	}

	if err := root.ExecuteContext(ctx); err != nil {
		var domainError *project.Error
		if errors.As(err, &domainError) {
			return nil, err
		}
		var memoryError *memory.Error
		if errors.As(err, &memoryError) {
			return nil, err
		}
		return nil, project.NewError(project.CodeValidationError, errors.New("invalid input"))
	}
	return output.Bytes(), nil
}

func addMemoryCommands(root *spf13cobra.Command, output *bytes.Buffer, input io.Reader, service memoryService) {
	var inputSource string
	createCommand := &spf13cobra.Command{Use: "create", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		if !command.Flags().Changed("input") || inputSource != "-" || input == nil {
			return errors.New("invalid input")
		}
		var value memory.CreateInput
		if err := decodeStrict(input, &value); err != nil {
			return errors.New("invalid input")
		}
		created, err := service.Create(command.Context(), value)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(created)})
	}}
	createCommand.Flags().StringVar(&inputSource, "input", "", "read one JSON object from stdin")
	var includeDeletedGet bool
	getCommand := &spf13cobra.Command{Use: "get <memory-id>", Args: func(_ *spf13cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("invalid input")
		}
		return nil
	}, RunE: func(command *spf13cobra.Command, args []string) error {
		value, err := service.Get(command.Context(), args[0], includeDeletedGet)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	getCommand.Flags().BoolVar(&includeDeletedGet, "include-deleted", false, "include a deleted memory")
	var kind string
	var tags []string
	var minImportance, minConfidence int
	var includeDeletedList bool
	listCommand := &spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		values, err := service.List(command.Context(), memory.ListFilter{Type: kind, Tags: tags, MinImportance: minImportance, MinConfidence: minConfidence, IncludeDeleted: includeDeletedList})
		if err != nil {
			return err
		}
		items := make([]memoryJSON, 0, len(values))
		for _, value := range values {
			items = append(items, toMemoryJSON(value))
		}
		return writeJSON(output, memoryListSuccess{OK: true, Data: memoryListData{Memories: items}})
	}}
	listCommand.Flags().StringVar(&kind, "type", "", "memory type")
	listCommand.Flags().StringArrayVar(&tags, "tag", nil, "required tag (repeatable)")
	listCommand.Flags().IntVar(&minImportance, "min-importance", 0, "minimum importance")
	listCommand.Flags().IntVar(&minConfidence, "min-confidence", 0, "minimum confidence")
	listCommand.Flags().BoolVar(&includeDeletedList, "include-deleted", false, "include deleted memories")

	var updateInput string
	var updateExpected int
	updateCommand := &spf13cobra.Command{Use: "update <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("expected-version") || updateExpected < 1 || !command.Flags().Changed("input") || updateInput != "-" || input == nil {
			return errors.New("invalid input")
		}
		var patch memory.UpdateInput
		if err := decodeStrict(input, &patch); err != nil {
			return errors.New("invalid input")
		}
		value, err := service.Update(command.Context(), args[0], updateExpected, patch)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	updateCommand.Flags().IntVar(&updateExpected, "expected-version", 0, "current memory version")
	updateCommand.Flags().StringVar(&updateInput, "input", "", "read one JSON object from stdin")

	var deleteExpected int
	deleteCommand := &spf13cobra.Command{Use: "delete <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("expected-version") || deleteExpected < 1 {
			return errors.New("invalid input")
		}
		value, err := service.Delete(command.Context(), args[0], deleteExpected)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	deleteCommand.Flags().IntVar(&deleteExpected, "expected-version", 0, "current memory version")

	var restoreVersion, restoreExpected int
	restoreCommand := &spf13cobra.Command{Use: "restore <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("version") || restoreVersion < 1 || !command.Flags().Changed("expected-version") || restoreExpected < 1 {
			return errors.New("invalid input")
		}
		value, err := service.Restore(command.Context(), args[0], restoreVersion, restoreExpected)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	restoreCommand.Flags().IntVar(&restoreVersion, "version", 0, "revision version to restore")
	restoreCommand.Flags().IntVar(&restoreExpected, "expected-version", 0, "current memory version")

	historyCommand := &spf13cobra.Command{Use: "history <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		values, err := service.History(command.Context(), args[0])
		if err != nil {
			return err
		}
		items := make([]revisionJSON, 0, len(values))
		for _, value := range values {
			item, convertErr := toRevisionJSON(value)
			if convertErr != nil {
				return convertErr
			}
			items = append(items, item)
		}
		return writeJSON(output, historySuccess{OK: true, Data: historyData{Revisions: items}})
	}}
	tagsCommand := &spf13cobra.Command{Use: "tags", Args: invalidArgs, RunE: invalidCommand}
	tagsCommand.AddCommand(
		&spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
			values, err := service.TagsList(command.Context())
			if err != nil {
				return err
			}
			return writeJSON(output, tagListSuccess{OK: true, Data: tagListData{Tags: toVocabularyJSON(values)}})
		}},
		&spf13cobra.Command{Use: "search <query>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
			values, err := service.TagsSearch(command.Context(), args[0])
			if err != nil {
				return err
			}
			return writeJSON(output, tagListSuccess{OK: true, Data: tagListData{Tags: toVocabularyJSON(values)}})
		}},
	)
	root.AddCommand(createCommand, updateCommand, deleteCommand, restoreCommand, getCommand, listCommand, historyCommand, tagsCommand)
}

type tagJSON struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"normalized_name"`
	Description    *string `json:"description"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}
type vocabularyTagJSON struct {
	tagJSON
	ActiveMemoryCount int `json:"active_memory_count"`
}
type tagListData struct {
	Tags []vocabularyTagJSON `json:"tags"`
}
type tagListSuccess struct {
	OK   bool        `json:"ok"`
	Data tagListData `json:"data"`
}
type memoryJSON struct {
	ID          string          `json:"id"`
	Version     int             `json:"version"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Content     string          `json:"content"`
	Importance  int             `json:"importance"`
	Confidence  int             `json:"confidence"`
	Attributes  json.RawMessage `json:"attributes"`
	Tags        []tagJSON       `json:"tags"`
	ContentHash string          `json:"content_hash"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	DeletedAt   *string         `json:"deleted_at"`
}
type memorySuccess struct {
	OK   bool       `json:"ok"`
	Data memoryJSON `json:"data"`
}
type memoryListData struct {
	Memories []memoryJSON `json:"memories"`
}
type memoryListSuccess struct {
	OK   bool           `json:"ok"`
	Data memoryListData `json:"data"`
}
type revisionJSON struct {
	Version      int        `json:"version"`
	Operation    string     `json:"operation"`
	AgentName    *string    `json:"agent_name"`
	AgentRole    string     `json:"agent_role"`
	WorktreeRoot string     `json:"worktree_root"`
	CreatedAt    string     `json:"created_at"`
	Snapshot     memoryJSON `json:"snapshot"`
}
type historyData struct {
	Revisions []revisionJSON `json:"revisions"`
}
type historySuccess struct {
	OK   bool        `json:"ok"`
	Data historyData `json:"data"`
}

func toMemoryJSON(value memory.Memory) memoryJSON {
	tags := make([]tagJSON, 0, len(value.Tags))
	for _, tag := range value.Tags {
		tags = append(tags, toTagJSON(tag))
	}
	attributes := value.Attributes
	if len(attributes) == 0 {
		attributes = json.RawMessage("null")
	}
	var deleted *string
	if value.DeletedAt != nil {
		formatted := value.DeletedAt.UTC().Format(time.RFC3339)
		deleted = &formatted
	}
	return memoryJSON{ID: value.ID, Version: value.Version, Type: value.Type, Title: value.Title, Content: value.Content, Importance: value.Importance, Confidence: value.Confidence, Attributes: attributes, Tags: tags, ContentHash: value.ContentHash, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339), DeletedAt: deleted}
}
func toTagJSON(tag memory.Tag) tagJSON {
	return tagJSON{ID: tag.ID, Name: tag.Name, NormalizedName: tag.NormalizedName, Description: tag.Description, CreatedAt: tag.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: tag.UpdatedAt.UTC().Format(time.RFC3339)}
}
func toVocabularyJSON(values []memory.Tag) []vocabularyTagJSON {
	tags := make([]vocabularyTagJSON, 0, len(values))
	for _, tag := range values {
		tags = append(tags, vocabularyTagJSON{tagJSON: toTagJSON(tag), ActiveMemoryCount: tag.ActiveMemoryCount})
	}
	return tags
}
func toRevisionJSON(value memory.Revision) (revisionJSON, error) {
	snapshot, err := memory.FromSnapshot(value.Snapshot)
	if err != nil {
		return revisionJSON{}, memory.NewError(memory.CodeStoreError, err)
	}
	return revisionJSON{Version: value.Version, Operation: value.Operation, AgentName: value.AgentName, AgentRole: value.AgentRole, WorktreeRoot: value.WorktreeRoot, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), Snapshot: toMemoryJSON(snapshot)}, nil
}

func decodeStrict(input io.Reader, value any) error {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid input")
	}
	return nil
}
func oneArg(_ *spf13cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid input")
	}
	return nil
}

func invalidArgs(_ *spf13cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("invalid input")
	}
	return nil
}
func invalidCommand(_ *spf13cobra.Command, _ []string) error { return errors.New("invalid input") }

type projectJSON struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	LastKnownGitCommonDir string `json:"last_known_git_common_dir"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

func toJSON(value project.Project) projectJSON {
	return projectJSON{ID: value.ID, Name: value.Name, LastKnownGitCommonDir: value.LastKnownGitCommonDir, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339)}
}

type projectSuccess struct {
	OK   bool        `json:"ok"`
	Data projectJSON `json:"data"`
}

type projectList struct {
	Projects []projectJSON `json:"projects"`
}

type listSuccess struct {
	OK   bool        `json:"ok"`
	Data projectList `json:"data"`
}

func successProject(value project.Project) any { return projectSuccess{OK: true, Data: toJSON(value)} }
func writeJSON(output *bytes.Buffer, value any) error {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
