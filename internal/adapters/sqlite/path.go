package sqlite

import (
	"fmt"
	"path/filepath"
)

type getenvFunc func(string) string
type userHomeDirFunc func() (string, error)

func databasePath(goos string, getenv getenvFunc, userHomeDir userHomeDirFunc) (string, error) {
	baseDirectory, err := dataDirectory(goos, getenv, userHomeDir)
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDirectory, applicationDirectory, databaseFilename), nil
}

func dataDirectory(goos string, getenv getenvFunc, userHomeDir userHomeDirFunc) (string, error) {
	switch goos {
	case "windows":
		if localAppData := getenv("LOCALAPPDATA"); localAppData != "" {
			return localAppData, nil
		}

		return "", fmt.Errorf("LOCALAPPDATA is not set")
	case "darwin":
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}

		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if xdgDataHome := getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return xdgDataHome, nil
		}

		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}

		return filepath.Join(home, ".local", "share"), nil
	}
}
