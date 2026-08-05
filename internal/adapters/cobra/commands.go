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
	Get(context.Context, string) (memory.Memory, error)
	List(context.Context, memory.ListFilter) ([]memory.Memory, error)
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
		decoder := json.NewDecoder(input)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			return errors.New("invalid input")
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return errors.New("invalid input")
		}
		created, err := service.Create(command.Context(), value)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(created)})
	}}
	createCommand.Flags().StringVar(&inputSource, "input", "", "read one JSON object from stdin")
	getCommand := &spf13cobra.Command{Use: "get <memory-id>", Args: func(_ *spf13cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("invalid input")
		}
		return nil
	}, RunE: func(command *spf13cobra.Command, args []string) error {
		value, err := service.Get(command.Context(), args[0])
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	var kind string
	var tags []string
	var minImportance, minConfidence int
	listCommand := &spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		values, err := service.List(command.Context(), memory.ListFilter{Type: kind, Tags: tags, MinImportance: minImportance, MinConfidence: minConfidence})
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
	root.AddCommand(createCommand, getCommand, listCommand)
}

type tagJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
	CreatedAt      string `json:"created_at"`
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

func toMemoryJSON(value memory.Memory) memoryJSON {
	tags := make([]tagJSON, 0, len(value.Tags))
	for _, tag := range value.Tags {
		tags = append(tags, tagJSON{ID: tag.ID, Name: tag.Name, NormalizedName: tag.NormalizedName, CreatedAt: tag.CreatedAt.UTC().Format(time.RFC3339)})
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
