package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

type fakePaneLayoutStore struct {
	deltas [3]int
	calls  int
	err    error
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

func (f *fakePaneLayoutStore) SavePaneDeltas(deltas [3]int) error {
	f.calls++
	f.deltas = deltas
	return f.err
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
	for _, want := range []string{"Statement:", "Given an array of integers"} {
		if !strings.Contains(view, want) {
			t.Fatalf("details missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"statement.md", "Patterns:", "Language:"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("details contains unwanted %q:\n%s", unwanted, view)
		}
	}
	if strings.Contains(ansi.Strip(view), "  Given an array") {
		t.Fatalf("statement line is indented in details:\n%s", view)
	}
}

func TestDetailsPaneScrollsStatement(t *testing.T) {
	statement := strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
	}, "\n")
	solutions := &fakeSolutions{statements: map[string]string{"two-sum": statement}}
	model := newTestModel(t, WithStatementStore(solutions))
	model.activePane = DetailsPane
	model.height = 12

	view := model.renderDetails(80, 8)
	if !strings.Contains(view, "Two Sum") {
		t.Fatalf("initial details missing problem metadata:\n%s", view)
	}

	updated := tea.Model(model)
	for i := 0; i < 6; i++ {
		updated, _ = updated.(Model).Update(key("j"))
	}
	scrolled := updated.(Model)
	view = scrolled.renderDetails(80, 8)
	if !strings.Contains(view, "line 1") {
		t.Fatalf("scrolled details missing first statement line:\n%s", view)
	}
}

func TestDetailsScrollIndicatorAlignsRight(t *testing.T) {
	statement := strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
	}, "\n")
	solutions := &fakeSolutions{statements: map[string]string{"two-sum": statement}}
	model := newTestModel(t, WithStatementStore(solutions))
	model.activePane = DetailsPane

	view := ansi.Strip(model.renderDetails(50, 4))
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("details view too short:\n%s", view)
	}
	header := lines[1]
	if !strings.Contains(header, "DETAILS") || !strings.Contains(header, "1/") {
		t.Fatalf("header missing title or indicator:\n%s", header)
	}
	if strings.Index(header, "1/")-strings.Index(header, "DETAILS") < 20 {
		t.Fatalf("scroll indicator is not right aligned:\n%s", header)
	}
}

func TestProblemNavigationResetsDetailsScroll(t *testing.T) {
	model := newTestModel(t)
	model.detailScroll = 4
	model.activePane = ProblemsPane

	updated, _ := model.Update(key("j"))
	if updated.(Model).detailScroll != 0 {
		t.Fatalf("detailScroll = %d, want reset to 0", updated.(Model).detailScroll)
	}
}

func TestLayoutPaneWidthsFitTerminal(t *testing.T) {
	model := newTestModel(t)
	model.width = 96
	model.height = 24

	width, height, trackW, problemW, detailW, bodyH := model.layout()
	if got := trackW + problemW + detailW + paneGapWidth; got != width {
		t.Fatalf("pane widths + gap = %d, want %d", got, width)
	}
	if bodyH != height-chromeHeight {
		t.Fatalf("bodyH = %d, want %d", bodyH, height-chromeHeight)
	}
	if trackW < trackMinWidth || problemW < problemMinWidth || detailW < detailMinWidth {
		t.Fatalf("pane widths below minimums: tracks=%d problems=%d details=%d", trackW, problemW, detailW)
	}
}

func TestRenderLeavesSpacerBetweenPanesAndFooter(t *testing.T) {
	model := newTestModel(t)
	model.width = 120
	model.height = 24

	view := model.render()
	hasSpacer := false
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.TrimSpace(line) == "" {
			hasSpacer = true
			break
		}
	}
	if !hasSpacer {
		t.Fatalf("rendered view does not include spacer line above footer:\n%s", view)
	}
}

func TestPanesRenderEqualHeightWithOverflowingTracks(t *testing.T) {
	model := newTestModel(t)
	for i := 0; i < 30; i++ {
		model.catalog.Tracks = append(model.catalog.Tracks, catalog.Track{
			Slug:     "extra-track",
			Title:    "Extra Track",
			Problems: []string{"two-sum"},
		})
	}

	height := 8
	tracks := model.renderTracks(24, height)
	problems := model.renderProblems(42, height)
	details := model.renderDetails(50, height)

	trackHeight := lipgloss.Height(tracks)
	if trackHeight != lipgloss.Height(problems) || trackHeight != lipgloss.Height(details) {
		t.Fatalf("pane heights differ: tracks=%d problems=%d details=%d", trackHeight, lipgloss.Height(problems), lipgloss.Height(details))
	}
}

func TestPanesRenderTitleSeparators(t *testing.T) {
	model := newTestModel(t)

	for name, view := range map[string]string{
		"tracks":   model.renderTracks(24, 8),
		"problems": model.renderProblems(42, 8),
		"details":  model.renderDetails(50, 8),
	} {
		text := ansi.Strip(view)
		if !strings.Contains(text, "──") {
			t.Fatalf("%s pane missing title separator:\n%s", name, view)
		}
	}
}

func TestResizeActivePaneChangesWidths(t *testing.T) {
	model := newTestModel(t)
	model.width = 120
	model.activePane = DetailsPane
	_, _, _, beforeProblemW, beforeDetailW, _ := model.layout()

	updated, _ := model.Update(key("]"))
	resized := updated.(Model)
	_, _, _, afterProblemW, afterDetailW, _ := resized.layout()

	if afterDetailW <= beforeDetailW {
		t.Fatalf("details width = %d, want greater than %d", afterDetailW, beforeDetailW)
	}
	if afterProblemW >= beforeProblemW {
		t.Fatalf("problem width = %d, want less than %d", afterProblemW, beforeProblemW)
	}
}

func TestResizeActivePanePersistsPaneDeltas(t *testing.T) {
	layoutStore := &fakePaneLayoutStore{}
	model := newTestModel(t, WithPaneLayoutStore(layoutStore))
	model.width = 120
	model.activePane = DetailsPane

	updated, _ := model.Update(key("]"))
	resized := updated.(Model)
	if layoutStore.calls != 1 {
		t.Fatalf("SavePaneDeltas calls = %d, want 1", layoutStore.calls)
	}
	if layoutStore.deltas != resized.paneDeltas {
		t.Fatalf("saved deltas = %#v, want %#v", layoutStore.deltas, resized.paneDeltas)
	}
}

func TestResetPaneWidths(t *testing.T) {
	model := newTestModel(t)
	model.width = 120
	model.activePane = DetailsPane

	updated, _ := model.Update(key("]"))
	resized := updated.(Model)
	if resized.paneDeltas == ([3]int{}) {
		t.Fatal("resize did not store pane deltas")
	}

	updated, _ = resized.Update(key("0"))
	reset := updated.(Model)
	if reset.paneDeltas != ([3]int{}) {
		t.Fatalf("paneDeltas = %#v, want reset", reset.paneDeltas)
	}
}

func TestFooterHighlightsCommandKeys(t *testing.T) {
	model := newTestModel(t)

	footer := model.renderBottom(120)
	if !strings.Contains(footer, "\x1b[") {
		t.Fatalf("footer does not contain styled command keys:\n%s", footer)
	}
	text := ansi.Strip(footer)
	for _, want := range []string{"j/k", "scroll", "tab", "pane"} {
		if !strings.Contains(text, want) {
			t.Fatalf("footer missing %q:\n%s", want, footer)
		}
	}
	for _, unwanted := range []string{"move selection", "scroll details"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("footer contains stale label %q:\n%s", unwanted, footer)
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
