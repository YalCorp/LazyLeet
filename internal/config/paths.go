package config

import (
	"os"
	"path/filepath"
)

const AppDirName = ".lazyleet"

type Paths struct {
	HomeDir string
	AppDir  string
	DBPath  string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	appDir := filepath.Join(home, AppDirName)
	return Paths{
		HomeDir: home,
		AppDir:  appDir,
		DBPath:  filepath.Join(appDir, "db.sqlite"),
	}, nil
}

func EnsureAppDir(paths Paths) error {
	return os.MkdirAll(paths.AppDir, 0o755)
}
