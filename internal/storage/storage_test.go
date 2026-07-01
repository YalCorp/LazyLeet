package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteProgressLifecycle(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	status, err := store.Progress(ctx, "two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusTodo {
		t.Fatalf("unknown problem status = %q, want %q", status, StatusTodo)
	}

	if err := store.SetProgress(ctx, "two-sum", StatusSolved); err != nil {
		t.Fatal(err)
	}
	status, err = store.Progress(ctx, "two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusSolved {
		t.Fatalf("stored status = %q, want %q", status, StatusSolved)
	}
}

func TestSQLitePreferredLanguageLifecycle(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	languageID, err := store.PreferredLanguage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if languageID != "" {
		t.Fatalf("default language preference = %q, want empty", languageID)
	}

	if err := store.SetPreferredLanguage(ctx, "java"); err != nil {
		t.Fatal(err)
	}
	languageID, err = store.PreferredLanguage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if languageID != "java" {
		t.Fatalf("stored language preference = %q, want java", languageID)
	}
}
