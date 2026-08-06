// Package agentconfig adapts coding-agent configuration files.
package agentconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jorgeluis594/happy-memory/internal/agentconfig"
	"github.com/tailscale/hujson"
)

const (
	warningRead        = "AGENT_CONFIG_READ_FAILED"
	warningInvalid     = "AGENT_CONFIG_INVALID"
	warningUnsupported = "AGENT_CONFIG_UNSUPPORTED"
	warningWrite       = "AGENT_CONFIG_WRITE_FAILED"
)

// Configurator updates persistent user configuration for supported agents.
type Configurator struct {
	homeDir   func() (string, error)
	lookupEnv func(string) (string, bool)
}

// New creates a configurator backed by the process environment.
func New() *Configurator {
	return &Configurator{homeDir: os.UserHomeDir, lookupEnv: os.LookupEnv}
}

// Configure updates one agent and converts every failure into a warning result.
func (configurator *Configurator) Configure(_ context.Context, agent agentconfig.Agent, sharedMemoryPath string) agentconfig.Result {
	path, err := configurator.configPath(agent)
	result := agentconfig.Result{Agent: agent, ConfigPath: path, SharedMemoryPath: sharedMemoryPath}
	if err != nil {
		return warned(result, warningRead, "agent configuration path is unavailable", err)
	}

	contents, mode, err := readConfig(path)
	if err != nil {
		return warned(result, warningRead, "agent configuration could not be read", err)
	}
	var updated []byte
	var changed bool
	switch agent {
	case agentconfig.AgentCodex:
		updated, changed, err = configureCodex(contents, sharedMemoryPath)
	case agentconfig.AgentClaudeCode:
		updated, changed, err = configureClaude(contents, sharedMemoryPath)
	case agentconfig.AgentOpenCode:
		updated, changed, err = configureOpenCode(contents, sharedMemoryPath)
	}
	if err != nil {
		code := warningInvalid
		if errors.Is(err, errUnsupported) {
			code = warningUnsupported
		}
		return warned(result, code, "agent configuration could not be updated", err)
	}
	if !changed {
		result.Status = agentconfig.StatusAlreadyConfigured
		return result
	}
	if err = writeAtomic(path, updated, mode); err != nil {
		return warned(result, warningWrite, "agent configuration could not be written", err)
	}
	result.Status = agentconfig.StatusConfigured
	return result
}

var errUnsupported = errors.New("unsupported configuration")

func warned(result agentconfig.Result, code, message string, err error) agentconfig.Result {
	result.Status = agentconfig.StatusWarning
	result.Warning = &agentconfig.Warning{Code: code, Message: message, Details: map[string]any{"cause": err.Error()}}
	return result
}

func (configurator *Configurator) configPath(agent agentconfig.Agent) (string, error) {
	home, err := configurator.homeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case agentconfig.AgentCodex:
		if codexHome, found := configurator.lookupEnv("CODEX_HOME"); found && strings.TrimSpace(codexHome) != "" {
			return filepath.Join(codexHome, "config.toml"), nil
		}
		return filepath.Join(home, ".codex", "config.toml"), nil
	case agentconfig.AgentClaudeCode:
		return filepath.Join(home, ".claude", "settings.json"), nil
	case agentconfig.AgentOpenCode:
		if custom, found := configurator.lookupEnv("OPENCODE_CONFIG"); found && strings.TrimSpace(custom) != "" {
			return custom, nil
		}
		base := filepath.Join(home, ".config")
		if xdg, found := configurator.lookupEnv("XDG_CONFIG_HOME"); found && strings.TrimSpace(xdg) != "" {
			base = xdg
		} else if runtime.GOOS == "windows" {
			if value, configErr := os.UserConfigDir(); configErr == nil {
				base = value
			}
		}
		directory := filepath.Join(base, "opencode")
		for _, name := range []string{"opencode.jsonc", "opencode.json", "config.json"} {
			candidate := filepath.Join(directory, name)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate, nil
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return candidate, statErr
			}
		}
		return filepath.Join(directory, "opencode.jsonc"), nil
	default:
		return "", errors.New("unsupported agent")
	}
}

func readConfig(path string) ([]byte, os.FileMode, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0o600, nil
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return contents, info.Mode().Perm(), nil
}

func configureCodex(contents []byte, sharedMemoryPath string) ([]byte, bool, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		return []byte("[sandbox_workspace_write]\nwritable_roots = [" + strconv.Quote(sharedMemoryPath) + "]\n"), true, nil
	}
	var document map[string]any
	if _, err := toml.Decode(string(contents), &document); err != nil {
		return nil, false, err
	}
	if _, found := document["default_permissions"]; found {
		return nil, false, fmt.Errorf("%w: default_permissions is active", errUnsupported)
	}
	if _, found := document["permissions"]; found {
		return nil, false, fmt.Errorf("%w: permission profiles are active", errUnsupported)
	}
	if mode, found := document["sandbox_mode"]; found && mode != "workspace-write" {
		return nil, false, fmt.Errorf("%w: sandbox_mode is not workspace-write", errUnsupported)
	}
	if table, found := document["sandbox_workspace_write"].(map[string]any); found {
		if roots, rootsFound := table["writable_roots"]; rootsFound {
			list, ok := roots.([]any)
			if !ok {
				return nil, false, fmt.Errorf("%w: writable_roots is not an array", errUnsupported)
			}
			for _, value := range list {
				if path, pathOK := value.(string); pathOK && filepath.Clean(path) == filepath.Clean(sharedMemoryPath) {
					return contents, false, nil
				}
			}
		}
	}
	updated, err := insertCodexRoot(contents, sharedMemoryPath)
	if err != nil {
		return nil, false, err
	}
	if _, err = toml.Decode(string(updated), &map[string]any{}); err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

func insertCodexRoot(contents []byte, path string) ([]byte, error) {
	text := string(contents)
	headerStart, headerEnd := findTOMLTable(text, "sandbox_workspace_write")
	quoted := strconv.Quote(path)
	if headerStart < 0 {
		separator := ""
		if !strings.HasSuffix(text, "\n") {
			separator = "\n"
		}
		return []byte(text + separator + "\n[sandbox_workspace_write]\nwritable_roots = [" + quoted + "]\n"), nil
	}
	sectionEnd := len(text)
	if next, _ := findNextTOMLTable(text, headerEnd); next >= 0 {
		sectionEnd = next
	}
	section := text[headerEnd:sectionEnd]
	keyStart := findTOMLKey(section, "writable_roots")
	if keyStart < 0 {
		return []byte(text[:headerEnd] + "\nwritable_roots = [" + quoted + "]" + text[headerEnd:]), nil
	}
	equal := strings.Index(section[keyStart:], "=")
	if equal < 0 {
		return nil, errors.New("invalid writable_roots assignment")
	}
	arrayStart := strings.Index(section[keyStart+equal:], "[")
	if arrayStart < 0 {
		return nil, errors.New("invalid writable_roots array")
	}
	arrayStart += headerEnd + keyStart + equal
	arrayEnd, err := matchingArrayEnd(text, arrayStart)
	if err != nil {
		return nil, err
	}
	inside := strings.TrimSpace(text[arrayStart+1 : arrayEnd])
	addition := quoted
	if inside != "" {
		if strings.HasSuffix(inside, ",") {
			addition = " " + addition
		} else {
			addition = ", " + addition
		}
	}
	return []byte(text[:arrayEnd] + addition + text[arrayEnd:]), nil
}

func findTOMLTable(text, name string) (int, int) {
	for offset := 0; offset < len(text); {
		lineEnd := strings.IndexByte(text[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text) - offset
		}
		line := strings.TrimSpace(strings.SplitN(text[offset:offset+lineEnd], "#", 2)[0])
		if line == "["+name+"]" {
			return offset, offset + lineEnd
		}
		offset += lineEnd + 1
	}
	return -1, -1
}

func findNextTOMLTable(text string, from int) (int, int) {
	for offset := from; offset < len(text); {
		lineEnd := strings.IndexByte(text[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text) - offset
		}
		line := strings.TrimSpace(strings.SplitN(text[offset:offset+lineEnd], "#", 2)[0])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return offset, offset + lineEnd
		}
		offset += lineEnd + 1
	}
	return -1, -1
}

func findTOMLKey(section, key string) int {
	for offset := 0; offset < len(section); {
		lineEnd := strings.IndexByte(section[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(section) - offset
		}
		line := strings.TrimSpace(section[offset : offset+lineEnd])
		if strings.HasPrefix(line, key) {
			rest := strings.TrimSpace(strings.TrimPrefix(line, key))
			if strings.HasPrefix(rest, "=") {
				return offset + strings.Index(section[offset:offset+lineEnd], key)
			}
		}
		offset += lineEnd + 1
	}
	return -1
}

func matchingArrayEnd(text string, start int) (int, error) {
	depth, quote, escaped, comment := 0, byte(0), false, false
	for index := start; index < len(text); index++ {
		character := text[index]
		if comment {
			if character == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && character == '\\' {
				escaped = true
				continue
			}
			if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '#':
			comment = true
		case '\'', '"':
			quote = character
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}
	return 0, errors.New("unterminated writable_roots array")
}

func configureClaude(contents []byte, sharedMemoryPath string) ([]byte, bool, error) {
	return configureJSONPath(contents, []string{"permissions", "additionalDirectories"}, sharedMemoryPath, true)
}

func configureOpenCode(contents []byte, sharedMemoryPath string) ([]byte, bool, error) {
	pattern := filepath.ToSlash(filepath.Join(sharedMemoryPath, "**"))
	return configureJSONPath(contents, []string{"permission", "external_directory", pattern}, "allow", false)
}

func configureJSONPath(contents []byte, path []string, value any, appendArray bool) ([]byte, bool, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		contents = []byte("{}\n")
	}
	// Standardize mutates the supplied buffer while removing HuJSON extras.
	// Parse a copy so the later patch retains comments from the original.
	standard, err := hujson.Standardize(bytes.Clone(contents))
	if err != nil {
		return nil, false, err
	}
	var document map[string]any
	if err = json.Unmarshal(standard, &document); err != nil {
		return nil, false, err
	}
	current := any(document)
	missingAt := -1
	for index, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("%w: %s is not an object", errUnsupported, strings.Join(path[:index], "."))
		}
		next, found := object[key]
		if !found {
			missingAt = index
			break
		}
		current = next
	}
	if missingAt < 0 {
		if appendArray {
			array, ok := current.([]any)
			if !ok {
				return nil, false, fmt.Errorf("%w: target is not an array", errUnsupported)
			}
			for _, item := range array {
				if item == value {
					return contents, false, nil
				}
			}
		} else if current == value {
			return contents, false, nil
		}
	}

	patchPath := path
	patchValue := value
	if missingAt >= 0 {
		patchPath = path[:missingAt+1]
		for index := len(path) - 1; index > missingAt; index-- {
			patchValue = map[string]any{path[index]: patchValue}
		}
		if appendArray && missingAt == len(path)-1 {
			patchValue = []any{value}
		}
	} else if appendArray {
		patchPath = append(append([]string{}, path...), "-")
	}
	patch, err := json.Marshal([]map[string]any{{"op": "add", "path": jsonPointer(patchPath), "value": patchValue}})
	if err != nil {
		return nil, false, err
	}
	parsed, err := hujson.Parse(contents)
	if err != nil {
		return nil, false, err
	}
	if err = parsed.Patch(patch); err != nil {
		return nil, false, err
	}
	return parsed.Pack(), true, nil
}

func jsonPointer(parts []string) string {
	escaped := make([]string, len(parts))
	for index, part := range parts {
		escaped[index] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(escaped, "/")
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".happy-memory-config-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(mode); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
