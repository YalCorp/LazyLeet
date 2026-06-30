package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndLoadPaneDeltas(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		HomeDir:      dir,
		AppDir:       dir,
		DBPath:       filepath.Join(dir, "db.sqlite"),
		WorkspaceDir: filepath.Join(dir, "workspace"),
		PacksDir:     filepath.Join(dir, "packs"),
	}
	if err := EnsureAppDir(paths); err != nil {
		t.Fatal(err)
	}

	want := [3]int{4, -2, -2}
	if err := Save(paths, AppConfig{
		DatabasePath:  filepath.Join(dir, "custom.sqlite"),
		WorkspacePath: filepath.Join(dir, "custom-workspace"),
		PaneDeltas:    want,
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PaneDeltas != want {
		t.Fatalf("PaneDeltas = %#v, want %#v", cfg.PaneDeltas, want)
	}
	if cfg.DatabasePath != filepath.Join(dir, "custom.sqlite") {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}
}

func TestPaneLayoutStorePreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		HomeDir:      dir,
		AppDir:       dir,
		DBPath:       filepath.Join(dir, "db.sqlite"),
		WorkspaceDir: filepath.Join(dir, "workspace"),
		PacksDir:     filepath.Join(dir, "packs"),
	}
	if err := EnsureAppDir(paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`database_path = "/tmp/db.sqlite"
workspace_path = "/tmp/workspace"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := [3]int{8, -4, -4}
	if err := NewPaneLayoutStore(paths).SavePaneDeltas(want); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.PaneDeltas, want) {
		t.Fatalf("PaneDeltas = %#v, want %#v", cfg.PaneDeltas, want)
	}
	if cfg.DatabasePath != "/tmp/db.sqlite" || cfg.WorkspacePath != "/tmp/workspace" {
		t.Fatalf("existing config was not preserved: %#v", cfg)
	}
}
