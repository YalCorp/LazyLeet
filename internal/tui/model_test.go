package tui

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/storage"
	"github.com/YalCorp/LazyLeet/internal/workspace"
)

type fakeStore struct {
	statuses            map[string]storage.Status
	preferredLanguageID string
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

type fakeTestCaseStore struct {
	result       workspace.TestRunResult
	err          error
	lastRequest  workspace.TestRunRequest
	requestCount int
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

func (f *fakeStore) PreferredLanguage(_ context.Context) (string, error) {
	return f.preferredLanguageID, nil
}

func (f *fakeStore) SetPreferredLanguage(_ context.Context, languageID string) error {
	f.preferredLanguageID = languageID
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

func (f fakeTestCaseStore) CountTestCases(_ catalog.Problem) (int, string, error) {
	return f.result.Total, "/tests", nil
}

func (f *fakeTestCaseStore) RunTestCases(_ catalog.Problem, _ workspace.Language, _ string, request workspace.TestRunRequest) (workspace.TestRunResult, error) {
	f.lastRequest = request
	f.requestCount++
	return f.result, f.err
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
	if strings.Contains(editor.statusLine, "/") || strings.Contains(editor.statusLine, ".py") {
		t.Fatalf("save status should not include path: %q", editor.statusLine)
	}
}

func TestEditorEnterKeepsPythonIndentAndOpensBlock(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{}))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	editor.editor.SetValue("class Solution:\n    def twoSum(self, nums, target):")
	updated, _ = editor.Update(key("enter"))
	editor = updated.(Model)

	want := "class Solution:\n    def twoSum(self, nums, target):\n        "
	if got := editor.editor.Value(); got != want {
		t.Fatalf("editor value = %q, want %q", got, want)
	}
}

func TestEditorEnterKeepsGoIndentAndOpensBlock(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{}))
	model.language = workspace.Language{ID: "go", Title: "Go", Filename: "solution.go"}
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	editor.editor.SetValue("func main() {")
	updated, _ = editor.Update(key("enter"))
	editor = updated.(Model)

	want := "func main() {\n    "
	if got := editor.editor.Value(); got != want {
		t.Fatalf("editor value = %q, want %q", got, want)
	}
}

func TestEditorTabInsertsIndentInsteadOfLeavingEditor(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{}))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.editor.SetValue("")

	updated, _ = editor.Update(key("tab"))
	editor = updated.(Model)

	if editor.Mode() != ModeEditor {
		t.Fatalf("mode = %s, want %s", editor.Mode(), ModeEditor)
	}
	if got := editor.editor.Value(); got != "    " {
		t.Fatalf("editor value = %q, want four spaces", got)
	}
}

func TestEditorBracketsInsertTextInsteadOfResizing(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{}))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.editor.SetValue("")
	_, _, beforeProblemW, beforeSolutionW, _ := editor.editorLayout(editor.width, editor.height)

	updated, _ = editor.Update(key("["))
	editor = updated.(Model)
	updated, _ = editor.Update(key("]"))
	editor = updated.(Model)
	_, _, afterProblemW, afterSolutionW, _ := editor.editorLayout(editor.width, editor.height)

	if got := editor.editor.Value(); got != "[]" {
		t.Fatalf("editor value = %q, want brackets inserted", got)
	}
	if afterProblemW != beforeProblemW || afterSolutionW != beforeSolutionW {
		t.Fatalf("brackets resized editor panes: problem %d->%d solution %d->%d", beforeProblemW, afterProblemW, beforeSolutionW, afterSolutionW)
	}
}

func TestEditorAcceptsBracketedPasteInSolutionPane(t *testing.T) {
	solutions := &fakeSolutions{
		contents: map[string]string{"two-sum:python": "class Solution:\n"},
	}
	model := newTestModel(t, WithSolutionStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, _ = editor.Update(tea.PasteMsg{Content: "    def twoSum(self, nums, target):\n        return []"})
	editor = updated.(Model)

	got := editor.editor.Value()
	for _, want := range []string{"def twoSum", "return []"} {
		if !strings.Contains(got, want) {
			t.Fatalf("editor value = %q, want pasted content %q", got, want)
		}
	}
}

func TestEditorIgnoresPasteOutsideSolutionPane(t *testing.T) {
	solutions := &fakeSolutions{
		contents: map[string]string{"two-sum:python": "class Solution:\n"},
	}
	model := newTestModel(t, WithSolutionStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	before := editor.editor.Value()

	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(tea.PasteMsg{Content: "pasted"})
	editor = updated.(Model)

	if editor.editor.Value() != before {
		t.Fatalf("editor changed while output pane focused: %q", editor.editor.Value())
	}
}

func TestEditorFormatsGoOnSave(t *testing.T) {
	solutions := &fakeSolutions{}
	model := newTestModel(t, WithSolutionStore(solutions))
	model.language = workspace.Language{ID: "go", Title: "Go", Filename: "solution.go"}
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	editor.editor.SetValue("package main\n\nfunc main(){\nfmt.Println(\"x\")\n}\n")
	updated, _ = editor.Update(keyCtrl('s'))
	editor = updated.(Model)

	got := solutions.contents[editor.editorProblem.Slug+":go"]
	for _, want := range []string{"func main() {", "\tfmt.Println(\"x\")"} {
		if !strings.Contains(got, want) {
			t.Fatalf("saved content = %q, want it to contain %q", got, want)
		}
	}
	if !strings.Contains(editor.statusLine, "Formatted and saved") {
		t.Fatalf("status line = %q, want formatted save", editor.statusLine)
	}
}

func TestEditorFormatsJavaOnSaveWhenSyntaxIsValid(t *testing.T) {
	solutions := &fakeSolutions{}
	model := newTestModel(t, WithSolutionStore(solutions))
	model.language = workspace.Language{ID: "java", Title: "Java", Filename: "Solution.java"}
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	editor.editor.SetValue("class Solution {public boolean validPath(int n,int[][] edges,int source,int destination){if(source==destination){return true;}return false;}}")
	updated, _ = editor.Update(keyCtrl('s'))
	editor = updated.(Model)

	got := solutions.contents[editor.editorProblem.Slug+":java"]
	for _, want := range []string{
		"class Solution {",
		"    public boolean validPath",
		"        if(source==destination){",
		"            return true;",
		"        }",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("saved content = %q, want it to contain %q", got, want)
		}
	}
	if !strings.Contains(editor.statusLine, "Formatted and saved") {
		t.Fatalf("status line = %q, want formatted save", editor.statusLine)
	}
}

func TestEditorSkipsJavaFormatWhenSyntaxIsInvalid(t *testing.T) {
	solutions := &fakeSolutions{}
	model := newTestModel(t, WithSolutionStore(solutions))
	model.language = workspace.Language{ID: "java", Title: "Java", Filename: "Solution.java"}
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	invalid := "class Solution {\n    public boolean validPath(int n, int[][] edges, int source, int destination) {\n        return true\n    }\n}\n"
	editor.editor.SetValue(invalid)
	updated, _ = editor.Update(keyCtrl('s'))
	editor = updated.(Model)

	got := solutions.contents[editor.editorProblem.Slug+":java"]
	if got != invalid {
		t.Fatalf("saved content changed after invalid Java format:\n got %q\nwant %q", got, invalid)
	}
	if editor.editor.Value() != invalid {
		t.Fatalf("editor content changed after invalid Java format:\n got %q\nwant %q", editor.editor.Value(), invalid)
	}
	if !strings.Contains(editor.statusLine, "format skipped") {
		t.Fatalf("status line = %q, want skipped format", editor.statusLine)
	}
}

func TestEditorRendersProblemStatementBesideSolution(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass\n"},
		statements: map[string]string{"two-sum": "Given nums and target, return matching indices."},
	}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.width = 120
	editor.height = 30

	view := editor.renderEditor()
	for _, want := range []string{"PROBLEM", "SOLUTION", "OUTPUT", "Given nums and target", "class Solution:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("editor view missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"solution.py", "/workspace/two-sum", "Python"} {
		if strings.Contains(ansi.Strip(view), unwanted) {
			t.Fatalf("editor view contains unwanted %q:\n%s", unwanted, view)
		}
	}
}

func TestEditorRenderKeepsFooterInsideTerminalHeight(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass\n"},
		statements: map[string]string{"two-sum": strings.Repeat("Given nums and target, return matching indices.\n", 20)},
	}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.width = 110
	editor.height = 24

	view := editor.renderEditor()
	if got := lipgloss.Height(view); got > editor.height {
		t.Fatalf("editor height = %d, want <= %d:\n%s", got, editor.height, view)
	}
	text := ansi.Strip(view)
	if !strings.Contains(text, "^w pane") {
		t.Fatalf("editor footer is missing:\n%s", view)
	}
	for _, unwanted := range []string{"ctrl+w", "ctrl+r", "ctrl+t", "ctrl+s", "ctrl+u/d", "ctrl+y", "Editing /"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("editor footer contains clutter %q:\n%s", unwanted, view)
		}
	}
}

func TestEditorRightColumnHasNoGapBetweenSolutionAndOutput(t *testing.T) {
	solutionH, outputH, gap, total := editorRightHeights(20)
	if gap != 0 {
		t.Fatalf("editor right column gap = %d, want 0", gap)
	}
	if total != solutionH+outputH {
		t.Fatalf("editor right column total = %d, want solution + output %d", total, solutionH+outputH)
	}
}

func TestEditorOutputPanelShowsRunError(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": "Given nums and target"},
	}
	tests := fakeTestCaseStore{err: errors.New("javac failed: exit status 1\nSolution.java:7: error: cannot find symbol")}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions), WithTestCaseStore(&tests))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, cmd := editor.Update(keyCtrl('r'))
	editor = updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+r did not start test command")
	}
	msg := cmd().(testRunFinishedMsg)
	updated, _ = editor.Update(msg)
	editor = updated.(Model)
	view := editor.renderEditor()

	for _, want := range []string{"OUTPUT", "Run error", "javac failed", "cannot find symbol"} {
		if !strings.Contains(view, want) {
			t.Fatalf("editor output missing %q:\n%s", want, view)
		}
	}
}

func TestEditorOutputPanelShowsTLE(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": "Given nums and target"},
	}
	tests := fakeTestCaseStore{result: workspace.TestRunResult{
		Total:     1,
		TimedOut:  true,
		TimeLimit: 10 * time.Second,
	}}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions), WithTestCaseStore(&tests))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, cmd := editor.Update(keyCtrl('r'))
	editor = updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+r did not start test command")
	}
	msg := cmd().(testRunFinishedMsg)
	updated, _ = editor.Update(msg)
	editor = updated.(Model)
	view := strings.Join(editor.editorOutputLines(80), "\n")

	for _, want := range []string{"Time Limit Exceeded", "Time limit: 10s", "Progress before timeout: 0/1 passed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("editor output missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(editor.statusLine, "Run TLE: time limit exceeded after 10s") {
		t.Fatalf("statusLine = %q", editor.statusLine)
	}
}

func TestEditorOutputPanelShowsFailedCaseIO(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": "Given nums and target"},
	}
	tests := fakeTestCaseStore{result: workspace.TestRunResult{
		Passed: 0,
		Total:  1,
		Failures: []workspace.TestFailure{{
			Index:    1,
			Input:    "nums = [2,7,11,15], target = 9",
			Actual:   "[1,0]",
			Expected: "[0,1]",
		}},
		Output: "LAZYLEET_RESULT 0 1",
	}}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions), WithTestCaseStore(&tests))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, cmd := editor.Update(keyCtrl('r'))
	editor = updated.(Model)
	msg := cmd().(testRunFinishedMsg)
	updated, _ = editor.Update(msg)
	editor = updated.(Model)
	view := strings.Join(editor.editorOutputLines(80), "\n")

	for _, want := range []string{"Failed case 1", "Input", "nums = [2,7,11,15], target = 9", "Output", "[1,0]", "Expected", "[0,1]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("editor output missing %q:\n%s", want, view)
		}
	}
}

func TestEditorRunAndSubmitUseSeparateTestScopes(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": "Given nums and target"},
	}
	tests := &fakeTestCaseStore{result: workspace.TestRunResult{Passed: 1, Total: 1}}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions), WithTestCaseStore(tests))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, cmd := editor.Update(keyCtrl('r'))
	editor = updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+r did not start run command")
	}
	_ = cmd()
	if tests.lastRequest.Mode != workspace.TestRunExamples {
		t.Fatalf("run mode = %s, want examples", tests.lastRequest.Mode)
	}

	updated, cmd = editor.Update(keyCtrl('t'))
	if cmd == nil {
		t.Fatal("ctrl+t did not start submit command")
	}
	_ = cmd()
	if tests.lastRequest.Mode != workspace.TestRunAll {
		t.Fatalf("submit mode = %s, want all", tests.lastRequest.Mode)
	}
}

func TestEditorUseFailedSubmitCaseRunsCustomCase(t *testing.T) {
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": "Given nums and target"},
	}
	failedCase := workspace.TestCase{
		Input:    map[string]json.RawMessage{"nums": json.RawMessage(`[2,7,11,15]`), "target": json.RawMessage(`9`)},
		Expected: json.RawMessage(`[0,1]`),
		Comment:  "hidden",
	}
	tests := &fakeTestCaseStore{result: workspace.TestRunResult{
		Passed: 0,
		Total:  1,
		Failures: []workspace.TestFailure{{
			Index:    1,
			Input:    "nums = [2,7,11,15], target = 9",
			Actual:   "[1,0]",
			Expected: "[0,1]",
			Case:     failedCase,
		}},
	}}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions), WithTestCaseStore(tests))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)

	updated, cmd := editor.Update(keyCtrl('t'))
	editor = updated.(Model)
	msg := cmd().(testRunFinishedMsg)
	updated, _ = editor.Update(msg)
	editor = updated.(Model)

	updated, _ = editor.Update(keyCtrl('y'))
	editor = updated.(Model)
	updated, cmd = editor.Update(keyCtrl('r'))
	if cmd == nil {
		t.Fatal("ctrl+r did not run selected testcase")
	}
	_ = cmd()
	if tests.lastRequest.Mode != workspace.TestRunCustom {
		t.Fatalf("run mode = %s, want custom", tests.lastRequest.Mode)
	}
	if len(tests.lastRequest.Cases) != 1 || tests.lastRequest.Cases[0].Comment != "hidden" {
		t.Fatalf("custom cases = %#v, want failed hidden case", tests.lastRequest.Cases)
	}
}

func TestEditorOutputHidesRunnerProtocolLines(t *testing.T) {
	model := newTestModel(t)
	model.mode = ModeEditor
	model.hasLastRun = true
	model.lastRunMode = workspace.TestRunAll
	model.lastRunResult = workspace.TestRunResult{
		Passed: 0,
		Total:  1,
		Failures: []workspace.TestFailure{{
			Index:    1,
			Input:    "nums = [2,7,11,15], target = 9",
			Actual:   "[1,0]",
			Expected: "[0,1]",
		}},
		Output: "LAZYLEET_FAIL\t1\thidden\tnums = [2,7,11,15]\t[0,1]\t[1,0]\nuser log",
	}

	view := strings.Join(model.editorOutputLines(60), "\n")
	if strings.Contains(view, "LAZYLEET_FAIL") {
		t.Fatalf("output panel leaked protocol line:\n%s", view)
	}
	if !strings.Contains(view, "user log") {
		t.Fatalf("output panel missing user log:\n%s", view)
	}
}

func TestEditorProblemPaneScrollsIndependently(t *testing.T) {
	statement := strings.Join([]string{
		"first line",
		"second line",
		"third line",
		"fourth line",
		"fifth line",
		"sixth line",
		"seventh line",
		"eighth line",
		"ninth line",
		"tenth line",
		"eleventh line",
		"twelfth line",
		"thirteenth line",
		"fourteenth line",
		"fifteenth line",
		"sixteenth line",
		"seventeenth line",
		"eighteenth line",
		"nineteenth line",
		"twentieth line",
	}, "\n")
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": statement},
	}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.width = 100
	editor.height = 12

	before := editor.renderEditor()
	if !strings.Contains(before, "first line") {
		t.Fatalf("editor view missing first line before scroll:\n%s", before)
	}

	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(keyCtrl('d'))
	editor = updated.(Model)
	after := editor.renderEditor()
	if editor.editorProblemScroll == 0 {
		t.Fatal("ctrl+d did not scroll the editor problem pane")
	}
	if strings.Contains(after, "first line") {
		t.Fatalf("editor view still shows first line after scroll:\n%s", after)
	}
	if !strings.Contains(after, "seventh line") {
		t.Fatalf("editor view missing later statement line after scroll:\n%s", after)
	}
	if editor.editor.Value() != "class Solution:\n    pass" {
		t.Fatalf("editor content changed while scrolling problem pane: %q", editor.editor.Value())
	}
}

func TestEditorFocusedProblemPaneScrollsSmoothlyWithJK(t *testing.T) {
	statement := strings.Join([]string{
		"first line",
		"second line",
		"third line",
		"fourth line",
		"fifth line",
		"sixth line",
		"seventh line",
		"eighth line",
		"ninth line",
		"tenth line",
		"eleventh line",
		"twelfth line",
		"thirteenth line",
		"fourteenth line",
		"fifteenth line",
		"sixteenth line",
		"seventeenth line",
		"eighteenth line",
		"nineteenth line",
		"twentieth line",
	}, "\n")
	solutions := &fakeSolutions{
		contents:   map[string]string{"two-sum:python": "class Solution:\n    pass"},
		statements: map[string]string{"two-sum": statement},
	}
	model := newTestModel(t, WithSolutionStore(solutions), WithStatementStore(solutions))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.width = 100
	editor.height = 12

	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	if editor.editorPane != EditorProblemPane {
		t.Fatalf("editor pane = %d, want problem pane", editor.editorPane)
	}

	updated, _ = editor.Update(key("j"))
	editor = updated.(Model)
	if editor.editorProblemScroll != 1 {
		t.Fatalf("problem scroll = %d, want 1", editor.editorProblemScroll)
	}
	if editor.editor.Value() != "class Solution:\n    pass" {
		t.Fatalf("editor content changed while problem pane focused: %q", editor.editor.Value())
	}

	updated, _ = editor.Update(key("k"))
	editor = updated.(Model)
	if editor.editorProblemScroll != 0 {
		t.Fatalf("problem scroll = %d, want 0", editor.editorProblemScroll)
	}
}

func TestEditorPaneResizeUsesStandardKeys(t *testing.T) {
	model := newTestModel(t, WithSolutionStore(&fakeSolutions{}))
	updated, _ := model.Update(key("e"))
	editor := updated.(Model)
	editor.width = 120
	editor.height = 30
	_, _, beforeProblemW, beforeSolutionW, _ := editor.editorLayout(editor.width, editor.height)

	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(keyCtrl('w'))
	editor = updated.(Model)
	updated, _ = editor.Update(key("ctrl+right"))
	editor = updated.(Model)
	_, _, afterProblemW, afterSolutionW, _ := editor.editorLayout(editor.width, editor.height)
	if afterProblemW <= beforeProblemW || afterSolutionW >= beforeSolutionW {
		t.Fatalf("resize widths = problem %d->%d solution %d->%d", beforeProblemW, afterProblemW, beforeSolutionW, afterSolutionW)
	}

	updated, _ = editor.Update(keyCtrl('0'))
	editor = updated.(Model)
	_, _, resetProblemW, resetSolutionW, _ := editor.editorLayout(editor.width, editor.height)
	if resetProblemW != beforeProblemW || resetSolutionW != beforeSolutionW {
		t.Fatalf("reset widths = problem %d solution %d, want %d %d", resetProblemW, resetSolutionW, beforeProblemW, beforeSolutionW)
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
	store := &fakeStore{}
	solutions := &fakeSolutions{}
	model := newTestModelWithStore(t, store, WithSolutionStore(solutions))

	updated, _ := model.Update(key("l"))
	model = updated.(Model)
	if model.language.ID != "go" {
		t.Fatalf("language after cycle = %s, want go", model.language.ID)
	}
	if store.preferredLanguageID != "go" {
		t.Fatalf("stored language = %q, want go", store.preferredLanguageID)
	}

	updated, _ = model.Update(key("e"))
	editor := updated.(Model)
	if !strings.HasSuffix(editor.editorPath, "solution.go") {
		t.Fatalf("editor path = %q, want Go solution file", editor.editorPath)
	}
}

func TestModelLoadsPreferredLanguage(t *testing.T) {
	store := &fakeStore{preferredLanguageID: "java"}
	model := newTestModelWithStore(t, store)
	if model.language.ID != "java" {
		t.Fatalf("language = %s, want java", model.language.ID)
	}
}

func TestDetailsRenderLocalStatementPreview(t *testing.T) {
	solutions := &fakeSolutions{statements: map[string]string{
		"two-sum": "# Two Sum\n\nGiven an array of integers, return indices of two numbers.",
	}}
	model := newTestModel(t, WithStatementStore(solutions))

	view := model.renderDetails(80, 28)
	text := ansi.Strip(view)
	for _, want := range []string{"Difficulty:", "Status:", "Statement:", "Given an array of integers", "External link:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("details missing %q:\n%s", want, view)
		}
	}
	if strings.Index(text, "External link:") < strings.Index(text, "Given an array of integers") {
		t.Fatalf("external link should render after statement:\n%s", view)
	}
	for _, unwanted := range []string{"ID:", "URL:", "statement.md", "Patterns:", "Language:"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("details contains unwanted %q:\n%s", unwanted, view)
		}
	}
	if strings.Contains(text, "  Given an array") {
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
	for i := 0; i < 5; i++ {
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

func TestDetailsPaneDoesNotRenderRightMargin(t *testing.T) {
	model := newTestModel(t)
	details := model.renderDetails(50, 8)
	for _, line := range strings.Split(ansi.Strip(details), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Fatalf("details pane line has trailing margin: %q\n%s", line, details)
		}
	}
}

func TestPanesConstrainLongContentToBounds(t *testing.T) {
	solutions := &fakeSolutions{statements: map[string]string{
		"find-if-path-exists-in-graph": strings.Repeat("verylongstatementtoken", 12),
	}}
	model := newTestModel(t, WithStatementStore(solutions))
	problem := model.SelectedProblem()
	problem.Title = strings.Repeat("Very Long Problem Title ", 8)
	problem.URL = "https://leetcode.com/problems/" + strings.Repeat("very-long-url-segment-", 12)
	model.catalog.Problems[problem.Slug] = problem

	problemsWidth, detailsWidth, height := 34, 38, 10
	problems := model.renderProblems(problemsWidth, height)
	details := model.renderDetails(detailsWidth, height)

	if got := lipgloss.Height(problems); got != height {
		t.Fatalf("problems height = %d, want %d:\n%s", got, height, problems)
	}
	if got := lipgloss.Height(details); got != height {
		t.Fatalf("details height = %d, want %d:\n%s", got, height, details)
	}
	for _, line := range strings.Split(ansi.Strip(problems), "\n") {
		if got := ansi.StringWidth(line); got > problemsWidth+1 {
			t.Fatalf("problems line width = %d, want <= %d: %q\n%s", got, problemsWidth+1, line, problems)
		}
	}
	for _, line := range strings.Split(ansi.Strip(details), "\n") {
		if got := ansi.StringWidth(line); got > detailsWidth {
			t.Fatalf("details line width = %d, want <= %d: %q\n%s", got, detailsWidth, line, details)
		}
	}
}

func TestResizeActivePaneChangesWidths(t *testing.T) {
	model := newTestModel(t)
	model.width = 120
	model.activePane = DetailsPane
	_, _, _, beforeProblemW, beforeDetailW, _ := model.layout()

	updated, _ := model.Update(key("ctrl+right"))
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

	updated, _ := model.Update(key("ctrl+right"))
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

	updated, _ := model.Update(key("ctrl+right"))
	resized := updated.(Model)
	if resized.paneDeltas == ([3]int{}) {
		t.Fatal("resize did not store pane deltas")
	}

	updated, _ = resized.Update(keyCtrl('0'))
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
	for _, want := range []string{"Python", "Ready"} {
		if !strings.Contains(text, want) {
			t.Fatalf("footer missing %q:\n%s", want, footer)
		}
	}
	for _, unwanted := range []string{"Language:", "Using Python", "move selection", "scroll details"} {
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
	return newTestModelWithStore(t, &fakeStore{}, opts...)
}

func newTestModelWithStore(t *testing.T, store *fakeStore, opts ...Option) Model {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(c, store, opts...)
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+left":
		return tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl}
	case "ctrl+right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl}
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
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	}
	if len(s) == 1 {
		return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func keyCtrl(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}
