package cli

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand(WithRunner(func(tea.Model) error {
		t.Fatal("runner should not be called for version")
		return nil
	}))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("lazyleet dev")) {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestRootCommandConstructsModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	called := false
	cmd := NewRootCommand(WithRunner(func(model tea.Model) error {
		called = true
		if model == nil {
			t.Fatal("model is nil")
		}
		return nil
	}))
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runner was not called")
	}
}
