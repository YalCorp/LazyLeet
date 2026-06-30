package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type AppConfig struct {
	DatabasePath  string `toml:"database_path"`
	WorkspacePath string `toml:"workspace_path"`
	PaneDeltas    [3]int `toml:"pane_deltas"`
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

func Save(paths Paths, cfg AppConfig) error {
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = paths.DBPath
	}
	if cfg.WorkspacePath == "" {
		cfg.WorkspacePath = paths.WorkspaceDir
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.AppDir, "config.toml"), buf.Bytes(), 0o644)
}

type PaneLayoutStore struct {
	paths Paths
}

func NewPaneLayoutStore(paths Paths) PaneLayoutStore {
	return PaneLayoutStore{paths: paths}
}

func (s PaneLayoutStore) SavePaneDeltas(deltas [3]int) error {
	cfg, err := Load(s.paths)
	if err != nil {
		return err
	}
	cfg.PaneDeltas = deltas
	return Save(s.paths, cfg)
}
