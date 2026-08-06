package agentconfig

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	domain "github.com/jorgeluis594/happy-memory/internal/agentconfig"
)

func testConfigurator(t *testing.T) (*Configurator, string) {
	t.Helper()
	home := t.TempDir()
	values := map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, ".config")}
	return &Configurator{
		homeDir: func() (string, error) { return home, nil },
		lookupEnv: func(key string) (string, bool) {
			value, found := values[key]
			return value, found
		},
	}, home
}

func TestConfigureCodexPreservesCommentsAndIsIdempotent(t *testing.T) {
	configurator, home := testConfigurator(t)
	path := filepath.Join(home, ".codex", "config.toml")
	writeFixture(t, path, "# keep me\nmodel = \"gpt\"\n\n[sandbox_workspace_write]\nnetwork_access = false # keep this too\nwritable_roots = [\"/existing\"]\n")
	result := configurator.Configure(context.Background(), domain.AgentCodex, "/repo/.happy-memory")
	if result.Status != domain.StatusConfigured {
		t.Fatalf("result=%#v", result)
	}
	contents := readFixture(t, path)
	if !strings.Contains(contents, "# keep me") || !strings.Contains(contents, `writable_roots = ["/existing", "/repo/.happy-memory"]`) {
		t.Fatalf("contents=%s", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%v", info.Mode().Perm())
	}
	result = configurator.Configure(context.Background(), domain.AgentCodex, "/repo/.happy-memory")
	if result.Status != domain.StatusAlreadyConfigured {
		t.Fatalf("result=%#v", result)
	}
}

func TestConfigureCodexAppendsAfterTrailingCommaComment(t *testing.T) {
	configurator, home := testConfigurator(t)
	path := filepath.Join(home, ".codex", "config.toml")
	writeFixture(t, path, "[sandbox_workspace_write]\nwritable_roots = [\n  \"/existing\", # keep\n]\n")

	result := configurator.Configure(context.Background(), domain.AgentCodex, "/repo/.happy-memory")
	contents := readFixture(t, path)
	if result.Status != domain.StatusConfigured {
		t.Fatalf("result=%#v contents=%s", result, contents)
	}
	if !strings.Contains(contents, "# keep") || strings.Contains(contents, "# keep\n, ") {
		t.Fatalf("contents=%s", contents)
	}
	var document map[string]any
	if _, err := toml.Decode(contents, &document); err != nil {
		t.Fatalf("invalid TOML: %v\ncontents=%s", err, contents)
	}
}

func TestConfigureClaudeAndOpenCodePreserveJSONComments(t *testing.T) {
	configurator, home := testConfigurator(t)
	claudePath := filepath.Join(home, ".claude", "settings.json")
	writeFixture(t, claudePath, "{\n  \"permissions\": {\"allow\": []}\n}\n")
	result := configurator.Configure(context.Background(), domain.AgentClaudeCode, "/repo/.happy-memory")
	if result.Status != domain.StatusConfigured || !strings.Contains(readFixture(t, claudePath), "additionalDirectories") {
		t.Fatalf("result=%#v contents=%s", result, readFixture(t, claudePath))
	}

	openCodePath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	writeFixture(t, openCodePath, "{\n  // keep me\n  \"permission\": {}\n}\n")
	result = configurator.Configure(context.Background(), domain.AgentOpenCode, "/repo/.happy-memory")
	contents := readFixture(t, openCodePath)
	if result.Status != domain.StatusConfigured || !strings.Contains(contents, "// keep me") || !strings.Contains(contents, "external_directory") {
		t.Fatalf("result=%#v contents=%s", result, contents)
	}
}

func TestConfigureClaudeCreatesAdditionalDirectoriesArray(t *testing.T) {
	configurator, home := testConfigurator(t)
	path := filepath.Join(home, ".claude", "settings.json")

	result := configurator.Configure(context.Background(), domain.AgentClaudeCode, "/repo/.happy-memory")
	if result.Status != domain.StatusConfigured {
		t.Fatalf("result=%#v", result)
	}

	var document struct {
		Permissions struct {
			AdditionalDirectories []string `json:"additionalDirectories"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(readFixture(t, path)), &document); err != nil {
		t.Fatal(err)
	}
	if got := document.Permissions.AdditionalDirectories; len(got) != 1 || got[0] != "/repo/.happy-memory" {
		t.Fatalf("additionalDirectories=%v", got)
	}

	result = configurator.Configure(context.Background(), domain.AgentClaudeCode, "/repo/.happy-memory")
	if result.Status != domain.StatusAlreadyConfigured {
		t.Fatalf("result=%#v", result)
	}
}

func TestConfigureContinuesAsWarningForInvalidOrUnsupportedConfig(t *testing.T) {
	configurator, home := testConfigurator(t)
	path := filepath.Join(home, ".codex", "config.toml")
	writeFixture(t, path, "default_permissions = \":workspace\"\n")
	result := configurator.Configure(context.Background(), domain.AgentCodex, "/repo/.happy-memory")
	if result.Status != domain.StatusWarning || result.Warning == nil || result.Warning.Code != warningUnsupported {
		t.Fatalf("result=%#v", result)
	}
	if got := readFixture(t, path); got != "default_permissions = \":workspace\"\n" {
		t.Fatalf("config changed: %s", got)
	}
	writeFixture(t, path, "sandbox_mode = \"read-only\"\n")
	result = configurator.Configure(context.Background(), domain.AgentCodex, "/repo/.happy-memory")
	if result.Status != domain.StatusWarning || result.Warning == nil || result.Warning.Code != warningUnsupported {
		t.Fatalf("result=%#v", result)
	}
}

func writeFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
