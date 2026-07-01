package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestDoctorCommandReportsRuntimeAndLocalSetup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	appDir := filepath.Join(home, ".lazyleet")
	for _, dir := range []string{
		appDir,
		filepath.Join(appDir, "workspace"),
		filepath.Join(appDir, "packs"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	oldLookPath := doctorLookPath
	oldOutput := doctorOutput
	t.Cleanup(func() {
		doctorLookPath = oldLookPath
		doctorOutput = oldOutput
	})
	doctorLookPath = func(file string) (string, error) {
		if file == "python3" {
			return "", fmt.Errorf("%s missing", file)
		}
		return "/usr/bin/" + file, nil
	}
	doctorOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "javac":
			return []byte("javac 25.0.3\n"), nil
		case "java":
			return []byte("openjdk version \"25.0.3\"\n"), nil
		case "go":
			return []byte("go version go1.25 linux/amd64\n"), nil
		default:
			return nil, fmt.Errorf("unexpected command %s", name)
		}
	}

	cmd := NewRootCommand(WithRunner(func(tea.Model) error {
		t.Fatal("runner should not be called for doctor")
		return nil
	}))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"LazyLeet doctor",
		"Runtimes",
		"Java",
		"javac 25.0.3",
		"Python",
		"missing",
		"Local data",
		"Workspace",
		"Data packs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
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
