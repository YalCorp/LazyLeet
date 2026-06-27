package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type AppConfig struct {
	DatabasePath  string `toml:"database_path"`
	WorkspacePath string `toml:"workspace_path"`
}

func Load(paths Paths) (AppConfig, error) {
	cfg := AppConfig{DatabasePath: paths.DBPath, WorkspacePath: paths.WorkspaceDir}
	path := filepath.Join(paths.AppDir, "config.toml")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return AppConfig{}, err
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = paths.DBPath
	}
	if cfg.WorkspacePath == "" {
		cfg.WorkspacePath = paths.WorkspaceDir
	}
	return cfg, nil
}
