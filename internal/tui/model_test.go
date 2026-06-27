package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/storage"
)

type fakeStore struct {
	statuses map[string]storage.Status
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

func newTestModel(t *testing.T) Model {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(c, &fakeStore{})
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
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func keyCtrl(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}
