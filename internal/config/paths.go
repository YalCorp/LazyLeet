package config

import (
	"os"
	"path/filepath"
)

const AppDirName = ".lazyleet"

type Paths struct {
	HomeDir      string
	AppDir       string
	DBPath       string
	WorkspaceDir string
	PacksDir     string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	appDir := filepath.Join(home, AppDirName)
	return Paths{
		HomeDir:      home,
		AppDir:       appDir,
		DBPath:       filepath.Join(appDir, "db.sqlite"),
		WorkspaceDir: filepath.Join(appDir, "workspace"),
		PacksDir:     filepath.Join(appDir, "packs"),
	}, nil
}

func EnsureAppDir(paths Paths) error {
	if err := os.MkdirAll(paths.AppDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.WorkspaceDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(paths.PacksDir, 0o755)
}
