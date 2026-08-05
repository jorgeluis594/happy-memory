package sqlite

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDatabasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		env  map[string]string
		home string
		want string
	}{
		{name: "macOS", goos: "darwin", home: "/Users/test", want: "/Users/test/Library/Application Support/happy-memory/happy-memory.db"},
		{name: "Linux XDG", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "/data"}, home: "/home/test", want: "/data/happy-memory/happy-memory.db"},
		{name: "Linux fallback", goos: "linux", home: "/home/test", want: "/home/test/.local/share/happy-memory/happy-memory.db"},
		{name: "Windows", goos: "windows", env: map[string]string{"LOCALAPPDATA": `C:\Users\test\AppData\Local`}, want: filepath.Join(`C:\Users\test\AppData\Local`, "happy-memory", "happy-memory.db")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			getenv := func(key string) string { return test.env[key] }
			home := func() (string, error) { return test.home, nil }
			got, err := databasePath(test.goos, getenv, home)
			if err != nil {
				t.Fatalf("databasePath() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("databasePath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDataDirectoryRequiresWindowsLocalAppData(t *testing.T) {
	t.Parallel()

	_, err := dataDirectory("windows", func(string) string { return "" }, func() (string, error) {
		return "", errors.New("must not be called")
	})
	if err == nil {
		t.Fatal("dataDirectory() error = nil, want an error")
	}
}
