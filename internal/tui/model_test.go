package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/storage"
	"github.com/YalCorp/LazyLeet/internal/workspace"
)

type fakeStore struct {
	statuses map[string]storage.Status
}

type fakeSolutions struct {
	contents   map[string]string
	statements map[string]string
	paths      map[string]string
	readErr    error
	saveErr    error
}

func (f *fakeStore) Progress(_ context.Context, slug string) (storage.Status, error) {
	if f.statuses == nil {
		f.statuses = map[string]storage.Status{}
	}
	if status, ok := f.statuses[slug]; ok {
		return status, nil
	}
	return storage.StatusTodo, nil
}

func (f *fakeStore) SetProgress(_ context.Context, slug string, status storage.Status) error {
	if f.statuses == nil {
		f.statuses = map[string]storage.Status{}
	}
	f.statuses[slug] = status
	return nil
}

func (f *fakeSolutions) ReadSolution(problem catalog.Problem, language workspace.Language) (string, string, error) {
	if f.readErr != nil {
		return "", "", f.readErr
	}
	if f.contents == nil {
		f.contents = map[string]string{}
	}
	if f.paths == nil {
		f.paths = map[string]string{}
	}
	path := f.paths[problem.Slug]
	if path == "" {
		path = filepath.Join("/workspace", problem.Slug, language.Filename)
	}
	return f.contents[problem.Slug+":"+language.ID], path, nil
}

func (f *fakeSolutions) SaveSolution(problem catalog.Problem, language workspace.Language, content string) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	if f.contents == nil {
		f.contents = map[string]string{}
	}
	if f.paths == nil {
		f.paths = map[string]string{}
	}
	path := f.paths[problem.Slug]
	if path == "" {
		path = filepath.Join("/workspace", problem.Slug, language.Filename)
	}
	f.contents[problem.Slug+":"+language.ID] = content
	return path, nil
}

func (f *fakeSolutions) ReadStatement(problem catalog.Problem) (string, string, error) {
	if f.readErr != nil {
		return "", "", f.readErr
	}
	if f.statements == nil {
		f.statements = map[string]string{}
	}
	path := filepath.Join("/workspace", problem.Slug, "statement.md")
	return f.statements[problem.Slug], path, nil
}

func TestNavigationChangesSelection(t *testing.T) {
	model := newTestModel(t)
	before := model.SelectedProblem().Slug
	updated, _ := model.Update(key("j"))
	after := updated.(Model).SelectedProblem().Slug
	if before == after {
		t.Fatalf("selection did not change after navigation: %s", before)
	}
}

func TestSearchAndCommandModes(t *testing.T) {
	model := newTestModel(t)
	updated, _ := model.Update(key("/"))
	if updated.(Model).Mode() != ModeSearch {
		t.Fatalf("/ mode = %s, want %s", updated.(Model).Mode(), ModeSearch)
	}
	updated, _ = model.Update(keyCtrl('p'))
	if updated.(Model).Mode() != ModeCommand {
		t.Fatalf("ctrl+p mode = %s, want %s", updated.(Model).Mode(), ModeCommand)
	}
}

func TestMarkCyclesProgress(t *testing.T) {
	store := &fakeStore{}
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	model := NewModel(c, store)
	problem := model.SelectedProblem()

	updated, _ := model.Update(key("m"))
	status, err := store.Progress(context.Background(), problem.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if status != storage.StatusAttempted {
		t.Fatalf("status = %s, want %s", status, storage.StatusAttempted)
	}
	if updated.(Model).SelectedProblem().Slug != problem.Slug {
		t.Fatal("marking progress changed the selected problem")
	}
}

func TestEditSolutionSavesContent(t *testing.T) {
	solutions := &fakeSolutions{contents: map[string]string{"two-sum:python": "class Solution:\n    pass\n"}}
	model := newTestModel(t, WithSolutionStore(solutions))
	problem := model.SelectedProblem()

	if model.language.ID != "python" {
		t.Fatalf("default language = %s, want python", model.language.ID)
	}
	updated, cmd := model.Update(key("e"))
	editor := updated.(Model)
	if editor.Mode() != ModeEditor {
		t.Fatalf("e mode = %s, want %s", editor.Mode(), ModeEditor)
	}
	if editor.editorProblem.Slug != problem.Slug {
		t.Fatalf("editor problem = %q, want %q", editor.editorProblem.Slug, problem.Slug)
	}
	if cmd == nil {
		t.Fatal("editor focus command is nil")
	}
	if !strings.HasSuffix(editor.editorPath, "solution.py") {
		t.Fatalf("editor path = %q, want Python solution file", editor.editorPath)
	}

	editor.editor.SetValue("class Solution:\n    pass\n")
	updated, _ = editor.Update(keyCtrl('s'))
	editor = updated.(Model)
	if got := solutions.contents[problem.Slug+":python"]; got != "class Solution:\n    pass\n" {
		t.Fatalf("saved content = %q", got)
	}
	if editor.statusLine == "" {
		t.Fatal("save did not update status line")
	}
}

func TestEditSolutionHandlesStoreError(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{readErr: errors.New("boom")}))
	updated, _ := model.Update(key("e"))
	if updated.(Model).Mode() != ModeBrowse {
		t.Fatalf("mode = %s, want %s", updated.(Model).Mode(), ModeBrowse)
	}
	if updated.(Model).statusLine != "Editor error: boom" {
		t.Fatalf("status line = %q", updated.(Model).statusLine)
	}
}

func TestLanguageCyclesAndControlsSolutionFile(t *testing.T) {
	solutions := &fakeSolutions{}
	model := newTestModel(t, WithSolutionStore(solutions))

	updated, _ := model.Update(key("l"))
	model = updated.(Model)
	if model.language.ID != "go" {
		t.Fatalf("language after cycle = %s, want go", model.language.ID)
	}

	updated, _ = model.Update(key("e"))
	editor := updated.(Model)
	if !strings.HasSuffix(editor.editorPath, "solution.go") {
		t.Fatalf("editor path = %q, want Go solution file", editor.editorPath)
	}
}

func TestDetailsRenderLocalStatementPreview(t *testing.T) {
	solutions := &fakeSolutions{statements: map[string]string{
		"two-sum": "# Two Sum\n\nGiven an array of integers, return indices of two numbers.",
	}}
	model := newTestModel(t, WithStatementStore(solutions))

	view := model.renderDetails(80, 28)
	for _, want := range []string{"Statement:", "statement.md", "Given an array of integers"} {
		if !strings.Contains(view, want) {
			t.Fatalf("details missing %q:\n%s", want, view)
		}
	}
}

func TestOpenURLReturnsCommand(t *testing.T) {
	model := newTestModel(t)
	problem := model.SelectedProblem()
	var opened string
	model.openURL = func(url string) tea.Cmd {
		return func() tea.Msg {
			opened = url
			return urlOpenedMsg{url: url}
		}
	}

	updated, cmd := model.Update(key("o"))
	if cmd == nil {
		t.Fatal("open URL command is nil")
	}
	if updated.(Model).statusLine == "" {
		t.Fatal("status line was not updated")
	}
	msg := cmd()
	if got := opened; got != problem.URL {
		t.Fatalf("opened URL = %q, want %q", got, problem.URL)
	}
	updated, _ = updated.(Model).Update(msg)
	if updated.(Model).statusLine != "Opened "+problem.URL {
		t.Fatalf("status line = %q", updated.(Model).statusLine)
	}
}

func newTestModel(t *testing.T, opts ...Option) Model {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(c, &fakeStore{}, opts...)
}

func key(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
	}
	switch s {
	case "j":
		return tea.KeyPressMsg{Text: "j", Code: 'j'}
	case "k":
		return tea.KeyPressMsg{Text: "k", Code: 'k'}
	case "/":
		return tea.KeyPressMsg{Text: "/", Code: '/'}
	case "m":
		return tea.KeyPressMsg{Text: "m", Code: 'm'}
	case "e":
		return tea.KeyPressMsg{Text: "e", Code: 'e'}
	case "l":
		return tea.KeyPressMsg{Text: "l", Code: 'l'}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func keyCtrl(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}
