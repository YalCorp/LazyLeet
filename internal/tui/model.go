package tui

import (
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/storage"
	"github.com/YalCorp/LazyLeet/internal/workspace"
)

type Pane int

const (
	TracksPane Pane = iota
	ProblemsPane
	DetailsPane
)

type EditorPane int

const (
	EditorProblemPane EditorPane = iota
	EditorSolutionPane
	EditorOutputPane
)

type Mode string

const (
	ModeBrowse  Mode = "browse"
	ModeSearch  Mode = "search"
	ModeCommand Mode = "command"
	ModeHelp    Mode = "help"
	ModeEditor  Mode = "editor"
)

type ProgressStore interface {
	Progress(ctx context.Context, slug string) (storage.Status, error)
	SetProgress(ctx context.Context, slug string, status storage.Status) error
}

type SolutionStore interface {
	ReadSolution(problem catalog.Problem, language workspace.Language) (content string, path string, err error)
	SaveSolution(problem catalog.Problem, language workspace.Language, content string) (path string, err error)
}

type StatementStore interface {
	ReadStatement(problem catalog.Problem) (content string, path string, err error)
}

type TestCaseStore interface {
	CountTestCases(problem catalog.Problem) (int, string, error)
	RunTestCases(problem catalog.Problem, language workspace.Language, solution string, request workspace.TestRunRequest) (workspace.TestRunResult, error)
}

type PaneLayoutStore interface {
	SavePaneDeltas(deltas [3]int) error
}

type LanguagePreferenceStore interface {
	PreferredLanguage(ctx context.Context) (string, error)
	SetPreferredLanguage(ctx context.Context, languageID string) error
}

type testRunFinishedMsg struct {
	problem catalog.Problem
	mode    workspace.TestRunMode
	result  workspace.TestRunResult
	err     error
}

type Option func(*Model)

func WithSolutionStore(store SolutionStore) Option {
	return func(m *Model) {
		m.solutions = store
	}
}

func WithStatementStore(store StatementStore) Option {
	return func(m *Model) {
		m.statements = store
	}
}

func WithTestCaseStore(store TestCaseStore) Option {
	return func(m *Model) {
		m.tests = store
	}
}

func WithPaneDeltas(deltas [3]int) Option {
	return func(m *Model) {
		m.paneDeltas = deltas
	}
}

func WithPaneLayoutStore(store PaneLayoutStore) Option {
	return func(m *Model) {
		m.paneLayoutStore = store
	}
}

type Model struct {
	catalog         catalog.Catalog
	store           ProgressStore
	solutions       SolutionStore
	statements      StatementStore
	tests           TestCaseStore
	paneLayoutStore PaneLayoutStore
	openURL         URLOpener

	width  int
	height int

	activePane   Pane
	trackIndex   int
	problemIdx   int
	detailScroll int
	paneDeltas   [3]int

	mode                Mode
	search              textinput.Model
	editor              textarea.Model
	editorPane          EditorPane
	editorPaneDelta     int
	editorProblem       catalog.Problem
	editorProblemScroll int
	editorOutputScroll  int
	language            workspace.Language
	editorPath          string
	lastRunProblem      catalog.Problem
	lastRunMode         workspace.TestRunMode
	lastRunResult       workspace.TestRunResult
	lastRunErr          string
	debugCaseProblem    string
	debugCase           *workspace.TestCase
	hasLastRun          bool
	testRunning         bool
	commandIndex        int
	statusLine          string
}

var commands = []string{
	"Go to Track",
	"Search Problem",
	"Edit Solution",
	"Change Language",
	"Mark Solved",
	"Open Official URL",
	"Open Notes",
	"Show Help",
}

const (
	paneGapWidth        = 6
	footerGapHeight     = 1
	chromeHeight        = 2 + footerGapHeight
	paneResizeStep      = 4
	trackMinWidth       = 20
	problemMinWidth     = 34
	detailMinWidth      = 30
	defaultLayoutWidth  = 96
	defaultLayoutHeight = 12
)

var paneMinWidths = [3]int{trackMinWidth, problemMinWidth, detailMinWidth}

func NewModel(c catalog.Catalog, store ProgressStore, opts ...Option) Model {
	search := textinput.New()
	search.Prompt = "/ "
	search.SetWidth(36)
	editor := textarea.New()
	editor.Placeholder = "Write your solution here..."
	editor.Prompt = ""
	editor.ShowLineNumbers = true
	model := Model{
		catalog:    c,
		store:      store,
		openURL:    openURLCommand,
		editor:     editor,
		editorPane: EditorSolutionPane,
		language:   workspace.DefaultLanguage(),
		width:      110,
		height:     32,
		activePane: ProblemsPane,
		mode:       ModeBrowse,
		search:     search,
		statusLine: "Ready",
	}
	for _, opt := range opts {
		opt(&model)
	}
	if languageStore, ok := store.(LanguagePreferenceStore); ok {
		if languageID, err := languageStore.PreferredLanguage(context.Background()); err == nil {
			if language, ok := workspace.LanguageByID(languageID); ok {
				model.language = language
			}
		}
	}
	return model.resizeEditor()
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.resizeEditor(), nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		return m.handlePaste(msg)
	case urlOpenedMsg:
		if msg.err != nil {
			m.statusLine = "Could not open URL: " + msg.err.Error()
		} else {
			m.statusLine = "Opened " + msg.url
		}
		return m, nil
	case testRunFinishedMsg:
		m.testRunning = false
		m.hasLastRun = true
		m.lastRunProblem = msg.problem
		m.lastRunMode = msg.mode
		m.lastRunResult = msg.result
		m.lastRunErr = ""
		if msg.err != nil {
			m.lastRunErr = msg.err.Error()
		}
		m.editorOutputScroll = 0
		m.statusLine = testRunStatus(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.mode != ModeEditor || m.editorPane != EditorSolutionPane {
		return m, nil
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "LazyLeet"
	return view
}

func (m Model) Mode() Mode {
	return m.mode
}

func (m Model) ActivePane() Pane {
	return m.activePane
}

func (m Model) SelectedProblem() catalog.Problem {
	problems := m.visibleProblems()
	if len(problems) == 0 {
		return catalog.Problem{}
	}
	idx := clamp(m.problemIdx, 0, len(problems)-1)
	return problems[idx]
}

func (m Model) SearchQuery() string {
	return m.search.Value()
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.mode {
	case ModeSearch:
		switch key {
		case "esc":
			m.mode = ModeBrowse
			m.statusLine = "Search closed"
			return m, nil
		case "enter":
			m.mode = ModeBrowse
			m.statusLine = "Search applied"
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.problemIdx = 0
		return m, cmd
	case ModeCommand:
		return m.handleCommandKey(key)
	case ModeHelp:
		if key == "esc" || key == "q" || key == "?" {
			m.mode = ModeBrowse
			m.statusLine = "Help closed"
		}
		return m, nil
	case ModeEditor:
		return m.handleEditorKey(msg)
	}

	switch key {
	case "q", "esc":
		return m, tea.Quit
	case "tab":
		m.activePane = (m.activePane + 1) % 3
	case "ctrl+left":
		m = m.resizeActivePane(-paneResizeStep)
	case "ctrl+right":
		m = m.resizeActivePane(paneResizeStep)
	case "ctrl+0":
		m = m.resetPaneWidths()
	case "up", "k":
		m = m.moveOrScrollDetails(-1)
	case "down", "j":
		m = m.moveOrScrollDetails(1)
	case "/":
		m.mode = ModeSearch
		m.search.Focus()
		m.statusLine = "Search problems"
	case "ctrl+p":
		m.mode = ModeCommand
		m.commandIndex = 0
		m.statusLine = "Command palette"
	case "?":
		m.mode = ModeHelp
		m.statusLine = "Help"
	case "enter":
		m.statusLine = fmt.Sprintf("Selected %s", m.SelectedProblem().Title)
	case "e":
		return m.openSelectedEditor()
	case "r":
		return m.runSelectedTests()
	case "l":
		m = m.cycleLanguage()
	case "n":
		m.statusLine = "Notes editor is planned for a later milestone"
	case "m":
		m = m.cycleProgress()
	case "o":
		return m.openSelectedURL()
	}
	return m, nil
}

func (m Model) handleCommandKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.mode = ModeBrowse
		m.statusLine = "Command palette closed"
	case "up", "k":
		m.commandIndex = clamp(m.commandIndex-1, 0, len(commands)-1)
	case "down", "j":
		m.commandIndex = clamp(m.commandIndex+1, 0, len(commands)-1)
	case "enter":
		action := commands[m.commandIndex]
		switch action {
		case "Go to Track":
			m.activePane = TracksPane
			m.mode = ModeBrowse
			m.statusLine = "Tracks focused"
		case "Search Problem":
			m.mode = ModeSearch
			m.search.Focus()
			m.statusLine = "Search problems"
		case "Edit Solution":
			m.mode = ModeBrowse
			return m.openSelectedEditor()
		case "Change Language":
			m = m.cycleLanguage()
			m.mode = ModeBrowse
		case "Mark Solved":
			m = m.setSelectedProgress(storage.StatusSolved)
			m.mode = ModeBrowse
		case "Open Official URL":
			m.mode = ModeBrowse
			return m.openSelectedURL()
		case "Open Notes":
			m.mode = ModeBrowse
			m.statusLine = "Notes editor is planned for a later milestone"
		case "Show Help":
			m.mode = ModeHelp
			m.statusLine = "Help"
		}
	}
	return m, nil
}

func (m Model) handleEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.editor.Blur()
		m.mode = ModeBrowse
		m.statusLine = "Editor closed"
		return m, nil
	case "ctrl+s":
		m = m.saveEditor()
		return m, nil
	case "ctrl+r":
		return m.runEditorTests(workspace.TestRunExamples)
	case "ctrl+t":
		return m.runEditorTests(workspace.TestRunAll)
	case "ctrl+y":
		m = m.useFailedTestCase()
		return m, nil
	case "ctrl+w":
		m = m.toggleEditorPane()
		return m, nil
	case "ctrl+u":
		m = m.scrollEditorPane(-editorScrollStep(m))
		return m, nil
	case "ctrl+d":
		m = m.scrollEditorPane(editorScrollStep(m))
		return m, nil
	case "ctrl+left":
		m = m.resizeEditorPane(-paneResizeStep)
		return m.resizeEditor(), nil
	case "ctrl+right":
		m = m.resizeEditorPane(paneResizeStep)
		return m.resizeEditor(), nil
	case "ctrl+0":
		m.editorPaneDelta = 0
		m.statusLine = "Editor panes reset"
		return m.resizeEditor(), nil
	}

	if m.editorPane != EditorSolutionPane {
		switch key {
		case "up", "k":
			m = m.scrollEditorPane(-1)
		case "down", "j":
			m = m.scrollEditorPane(1)
		}
		return m, nil
	}

	switch key {
	case "enter":
		m.insertEditorNewline()
		return m, nil
	case "tab":
		m.editor.InsertString(editorIndentUnit(m.language))
		return m, nil
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m Model) openSelectedEditor() (tea.Model, tea.Cmd) {
	problem := m.SelectedProblem()
	if problem.Slug == "" {
		m.statusLine = "No problem selected"
		return m, nil
	}
	if m.solutions == nil {
		m.statusLine = "Solution workspace unavailable"
		return m, nil
	}
	content, path, err := m.solutions.ReadSolution(problem, m.language)
	if err != nil {
		m.statusLine = "Editor error: " + err.Error()
		return m, nil
	}
	m.editorProblem = problem
	m.editorProblemScroll = 0
	m.editorOutputScroll = 0
	m.editorPane = EditorSolutionPane
	m.editorPath = path
	m.editor.SetValue(content)
	m.mode = ModeEditor
	m.statusLine = "Editing solution"
	m = m.resizeEditor()
	cmd := m.editor.Focus()
	return m, cmd
}

func (m Model) saveEditor() Model {
	if m.solutions == nil || m.editorProblem.Slug == "" {
		m.statusLine = "No solution file open"
		return m
	}
	content, formatStatus := formatEditorContent(m.editor.Value(), m.language)
	if formatStatus.changed {
		m.editor.SetValue(content)
	}
	path, err := m.solutions.SaveSolution(m.editorProblem, m.language, content)
	if err != nil {
		m.statusLine = "Save error: " + err.Error()
		return m
	}
	m.editorPath = path
	if formatStatus.changed {
		m.statusLine = "Formatted and saved"
	} else if formatStatus.skipped {
		m.statusLine = "Saved (format skipped: " + formatStatus.reason + ")"
	} else {
		m.statusLine = "Saved"
	}
	return m
}

func (m Model) runSelectedTests() (tea.Model, tea.Cmd) {
	problem := m.SelectedProblem()
	if problem.Slug == "" {
		m.statusLine = "No problem selected"
		return m, nil
	}
	if m.tests == nil {
		m.statusLine = "Test runner unavailable"
		return m, nil
	}
	if m.solutions == nil {
		m.statusLine = "Solution workspace unavailable"
		return m, nil
	}
	content, _, err := m.solutions.ReadSolution(problem, m.language)
	if err != nil {
		m.statusLine = "Run error: " + err.Error()
		return m, nil
	}
	m.statusLine = "Running examples for " + problem.Title
	m.testRunning = true
	m.hasLastRun = false
	m.lastRunProblem = problem
	m.lastRunMode = workspace.TestRunExamples
	m.lastRunErr = ""
	m.editorOutputScroll = 0
	return m, runTestsCmd(m.tests, problem, m.language, content, workspace.TestRunRequest{Mode: workspace.TestRunExamples})
}

func (m Model) runEditorTests(mode workspace.TestRunMode) (tea.Model, tea.Cmd) {
	if m.editorProblem.Slug == "" {
		m.statusLine = "No solution file open"
		return m, nil
	}
	if m.tests == nil {
		m.statusLine = "Test runner unavailable"
		return m, nil
	}
	request := workspace.TestRunRequest{Mode: mode}
	label := "examples"
	if mode == workspace.TestRunAll {
		label = "submit"
	} else if m.debugCase != nil && m.debugCaseProblem == m.editorProblem.Slug {
		request = workspace.TestRunRequest{Mode: workspace.TestRunCustom, Cases: []workspace.TestCase{*m.debugCase}}
		label = "selected testcase"
	}
	m.statusLine = "Running " + label + " for " + m.editorProblem.Title
	m.testRunning = true
	m.hasLastRun = false
	m.lastRunProblem = m.editorProblem
	m.lastRunMode = request.Mode
	m.lastRunErr = ""
	m.editorOutputScroll = 0
	return m, runTestsCmd(m.tests, m.editorProblem, m.language, m.editor.Value(), request)
}

func runTestsCmd(store TestCaseStore, problem catalog.Problem, language workspace.Language, solution string, request workspace.TestRunRequest) tea.Cmd {
	return func() tea.Msg {
		result, err := store.RunTestCases(problem, language, solution, request)
		return testRunFinishedMsg{problem: problem, mode: request.Mode, result: result, err: err}
	}
}

func testRunStatus(msg testRunFinishedMsg) string {
	label := "Run"
	if msg.mode == workspace.TestRunAll {
		label = "Submit"
	}
	if msg.err != nil {
		return label + " error: " + msg.err.Error()
	}
	if msg.result.TimedOut {
		return fmt.Sprintf("%s TLE: time limit exceeded after %s", label, msg.result.TimeLimit)
	}
	if msg.result.Total == 0 {
		return "No tests ran for " + msg.problem.Title
	}
	if msg.result.Passed == msg.result.Total {
		return fmt.Sprintf("%s passed: %d/%d", label, msg.result.Passed, msg.result.Total)
	}
	status := fmt.Sprintf("%s failed: %d/%d", label, msg.result.Passed, msg.result.Total)
	if len(msg.result.Failures) > 0 {
		failure := msg.result.Failures[0]
		status += fmt.Sprintf(" case %d expected %s got %s", failure.Index, failure.Expected, failure.Actual)
	}
	return status
}

func (m Model) useFailedTestCase() Model {
	if len(m.lastRunResult.Failures) == 0 {
		m.statusLine = "No failed testcase to use"
		return m
	}
	failure := m.lastRunResult.Failures[0]
	if len(failure.Case.Input) == 0 {
		m.statusLine = "Failed testcase data unavailable"
		return m
	}
	tc := failure.Case
	m.debugCase = &tc
	m.debugCaseProblem = m.lastRunProblem.Slug
	m.editorOutputScroll = 0
	m.statusLine = fmt.Sprintf("Using failed case %d for Run", failure.Index)
	return m
}

func (m *Model) insertEditorNewline() {
	line := currentEditorLine(m.editor)
	beforeCursor := line
	if col := m.editor.Column(); col >= 0 && col < len([]rune(line)) {
		beforeCursor = string([]rune(line)[:col])
	}
	indent := nextEditorIndent(beforeCursor, m.language)
	m.editor.InsertString("\n" + indent)
}

func currentEditorLine(editor textarea.Model) string {
	lines := strings.Split(editor.Value(), "\n")
	line := editor.Line()
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

func nextEditorIndent(line string, language workspace.Language) string {
	indent := leadingWhitespace(line)
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return indent
	}
	if opensCodeBlock(trimmed, language) {
		return indent + editorIndentUnit(language)
	}
	return indent
}

func leadingWhitespace(line string) string {
	var b strings.Builder
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func opensCodeBlock(trimmed string, language workspace.Language) bool {
	switch language.ID {
	case "python":
		return strings.HasSuffix(trimmed, ":")
	default:
		return strings.HasSuffix(trimmed, "{") ||
			strings.HasSuffix(trimmed, "(") ||
			strings.HasSuffix(trimmed, "[")
	}
}

func editorIndentUnit(language workspace.Language) string {
	return "    "
}

type editorFormatStatus struct {
	changed bool
	skipped bool
	reason  string
}

func formatEditorContent(content string, language workspace.Language) (string, editorFormatStatus) {
	switch language.ID {
	case "go":
		cleaned := trimTrailingLineWhitespace(content)
		formatted, err := format.Source([]byte(cleaned))
		if err == nil {
			cleaned = string(formatted)
		}
		return cleaned, editorFormatStatus{changed: cleaned != content}
	case "java":
		if !javaCompiles(content) {
			return content, editorFormatStatus{skipped: true, reason: "syntax error"}
		}
		formatted := formatJavaContent(content)
		return formatted, editorFormatStatus{changed: formatted != content}
	default:
		cleaned := trimTrailingLineWhitespace(content)
		return cleaned, editorFormatStatus{changed: cleaned != content}
	}
}

func javaCompiles(content string) bool {
	dir, err := os.MkdirTemp("", "lazyleet-format-java-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)

	source := "import java.util.*;\nimport java.io.*;\nimport java.math.*;\n\n" + content
	if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte(source), 0o644); err != nil {
		return false
	}
	cmd := exec.Command("javac", "-proc:none", "Solution.java")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func formatJavaContent(content string) string {
	tokens := javaFormatTokens(trimTrailingLineWhitespace(content))
	lines := make([]string, 0, len(tokens))
	indent := 0
	blank := false
	for _, token := range tokens {
		text := strings.TrimSpace(token.text)
		if text == "" {
			if !blank && len(lines) > 0 {
				lines = append(lines, "")
				blank = true
			}
			continue
		}
		if token.dedentBefore {
			indent = max(0, indent-1)
		}
		lines = append(lines, strings.Repeat(editorIndentUnit(workspace.Language{ID: "java"}), indent)+text)
		blank = false
		if token.indentAfter {
			indent++
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

type javaFormatToken struct {
	text         string
	indentAfter  bool
	dedentBefore bool
}

func javaFormatTokens(content string) []javaFormatToken {
	var tokens []javaFormatToken
	var b strings.Builder
	inLineComment := false
	inBlockComment := false
	inString := false
	inChar := false
	escaped := false
	parenDepth := 0
	runes := []rune(content)

	flush := func(indentAfter, dedentBefore bool) {
		text := strings.TrimSpace(b.String())
		if text != "" {
			tokens = append(tokens, javaFormatToken{text: text, indentAfter: indentAfter, dedentBefore: dedentBefore})
		}
		b.Reset()
	}

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				flush(false, false)
				inLineComment = false
				continue
			}
			b.WriteRune(ch)
			continue
		}
		if inBlockComment {
			b.WriteRune(ch)
			if ch == '*' && next == '/' {
				i++
				b.WriteRune(next)
				inBlockComment = false
			}
			continue
		}
		if inString || inChar {
			b.WriteRune(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if inString && ch == '"' {
				inString = false
			}
			if inChar && ch == '\'' {
				inChar = false
			}
			continue
		}

		switch {
		case ch == '/' && next == '/':
			b.WriteRune(ch)
			i++
			b.WriteRune(next)
			inLineComment = true
		case ch == '/' && next == '*':
			b.WriteRune(ch)
			i++
			b.WriteRune(next)
			inBlockComment = true
		case ch == '"':
			b.WriteRune(ch)
			inString = true
		case ch == '\'':
			b.WriteRune(ch)
			inChar = true
		case ch == '(':
			parenDepth++
			b.WriteRune(ch)
		case ch == ')':
			parenDepth = max(0, parenDepth-1)
			b.WriteRune(ch)
		case ch == '{':
			b.WriteRune(ch)
			flush(true, false)
		case ch == '}':
			flush(false, false)
			b.WriteRune(ch)
			if next == ';' {
				i++
				b.WriteRune(next)
			}
			flush(false, true)
		case ch == ';' && parenDepth == 0:
			b.WriteRune(ch)
			flush(false, false)
		case ch == '\n':
			flush(false, false)
		default:
			b.WriteRune(ch)
		}
	}
	flush(false, false)
	return tokens
}

func trimTrailingLineWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func (m Model) scrollEditorProblem(delta int) Model {
	_, _, problemW, _, bodyH := m.editorLayout(max(m.width, 80), max(m.height, 20))
	maxScroll := m.editorProblemMaxScroll(problemW, bodyH)
	m.editorProblemScroll = clamp(m.editorProblemScroll+delta, 0, maxScroll)
	return m
}

func (m Model) scrollEditorOutput(delta int) Model {
	_, _, _, outputH := editorRightHeights(m.editorBodyHeight(max(m.height, 20)))
	_, _, _, solutionW, _ := m.editorLayout(max(m.width, 80), max(m.height, 20))
	maxScroll := m.editorOutputMaxScroll(solutionW, outputH)
	m.editorOutputScroll = clamp(m.editorOutputScroll+delta, 0, maxScroll)
	return m
}

func (m Model) scrollEditorPane(delta int) Model {
	switch m.editorPane {
	case EditorProblemPane:
		return m.scrollEditorProblem(delta)
	case EditorOutputPane:
		return m.scrollEditorOutput(delta)
	default:
		return m
	}
}

func (m Model) toggleEditorPane() Model {
	switch m.editorPane {
	case EditorSolutionPane:
		m.editorPane = EditorOutputPane
		m.statusLine = "Output focused"
	case EditorOutputPane:
		m.editorPane = EditorProblemPane
		m.statusLine = "Problem focused"
	default:
		m.editorPane = EditorSolutionPane
		m.statusLine = "Solution focused"
	}
	return m
}

func (m Model) resizeEditorPane(delta int) Model {
	if m.editorPane != EditorProblemPane {
		delta = -delta
	}
	m.editorPaneDelta += delta
	m.editorPaneDelta = clamp(m.editorPaneDelta, -40, 40)
	if m.editorPane == EditorProblemPane {
		m.statusLine = "Problem pane resized"
	} else {
		m.statusLine = "Editor column resized"
	}
	return m
}

func editorScrollStep(m Model) int {
	bodyH := m.editorBodyHeight(max(m.height, 20))
	return max(1, panelBodyLimit(bodyH)/2)
}

func (m Model) cycleLanguage() Model {
	m.language = workspace.NextLanguage(m.language)
	m.statusLine = "Language changed"
	if languageStore, ok := m.store.(LanguagePreferenceStore); ok {
		if err := languageStore.SetPreferredLanguage(context.Background(), m.language.ID); err != nil {
			m.statusLine += " (could not save preference: " + err.Error() + ")"
		}
	}
	return m
}

func (m Model) openSelectedURL() (tea.Model, tea.Cmd) {
	problem := m.SelectedProblem()
	if problem.Slug == "" {
		m.statusLine = "No problem selected"
		return m, nil
	}
	if m.openURL == nil {
		m.statusLine = "Official URL: " + problem.URL
		return m, nil
	}
	m.statusLine = "Opening " + problem.URL
	return m, m.openURL(problem.URL)
}

func (m Model) move(delta int) Model {
	switch m.activePane {
	case TracksPane:
		m.trackIndex = clamp(m.trackIndex+delta, 0, len(m.catalog.Tracks)-1)
		m.problemIdx = 0
		m.detailScroll = 0
	case ProblemsPane, DetailsPane:
		problems := m.visibleProblems()
		m.problemIdx = clamp(m.problemIdx+delta, 0, len(problems)-1)
		m.detailScroll = 0
	}
	return m
}

func (m Model) moveOrScrollDetails(delta int) Model {
	if m.activePane == DetailsPane {
		m.detailScroll = clamp(m.detailScroll+delta, 0, m.maxDetailScroll())
		return m
	}
	return m.move(delta)
}

func (m Model) maxDetailScroll() int {
	_, _, _, _, detailW, bodyH := m.layout()
	return max(0, len(m.detailLines(detailW))-panelBodyLimit(bodyH))
}

func (m Model) resizeActivePane(delta int) Model {
	if delta == 0 {
		return m
	}
	width, _, trackW, problemW, detailW, _ := m.layout()
	available := max(totalPaneMinWidth(), width-paneGapWidth)
	widths := [3]int{trackW, problemW, detailW}
	active := int(m.activePane)

	if delta > 0 {
		remaining := delta
		for _, donor := range resizeDonors(m.activePane) {
			if remaining == 0 {
				break
			}
			capacity := max(0, widths[donor]-paneMinWidths[donor])
			take := min(remaining, capacity)
			widths[donor] -= take
			widths[active] += take
			remaining -= take
		}
	} else {
		shrink := min(-delta, max(0, widths[active]-paneMinWidths[active]))
		widths[active] -= shrink
		for _, receiver := range resizeReceivers(m.activePane) {
			if shrink == 0 {
				break
			}
			widths[receiver] += shrink
			shrink = 0
		}
	}

	widths = normalizePaneWidths(widths, available)
	defaults := defaultPaneWidths(available)
	for i := range m.paneDeltas {
		m.paneDeltas[i] = widths[i] - defaults[i]
	}
	m.statusLine = "Panes resized"
	m = m.savePaneDeltas()
	return m
}

func (m Model) resetPaneWidths() Model {
	m.paneDeltas = [3]int{}
	m.statusLine = "Pane widths reset"
	m = m.savePaneDeltas()
	return m
}

func (m Model) savePaneDeltas() Model {
	if m.paneLayoutStore == nil {
		return m
	}
	if err := m.paneLayoutStore.SavePaneDeltas(m.paneDeltas); err != nil {
		m.statusLine += " (could not save layout: " + err.Error() + ")"
	}
	return m
}

func (m Model) cycleProgress() Model {
	problem := m.SelectedProblem()
	if problem.Slug == "" || m.store == nil {
		m.statusLine = "No problem selected"
		return m
	}
	current, err := m.store.Progress(context.Background(), problem.Slug)
	if err != nil {
		m.statusLine = "Progress error: " + err.Error()
		return m
	}
	return m.setSelectedProgress(storage.NextStatus(current))
}

func (m Model) setSelectedProgress(status storage.Status) Model {
	problem := m.SelectedProblem()
	if problem.Slug == "" || m.store == nil {
		m.statusLine = "No problem selected"
		return m
	}
	if err := m.store.SetProgress(context.Background(), problem.Slug, status); err != nil {
		m.statusLine = "Progress error: " + err.Error()
		return m
	}
	m.statusLine = fmt.Sprintf("%s marked %s", problem.Title, status)
	return m
}

func (m Model) activeTrack() catalog.Track {
	if len(m.catalog.Tracks) == 0 {
		return catalog.Track{}
	}
	idx := clamp(m.trackIndex, 0, len(m.catalog.Tracks)-1)
	return m.catalog.Tracks[idx]
}

func (m Model) visibleProblems() []catalog.Problem {
	track := m.activeTrack()
	problems := m.catalog.TrackProblems(track)
	query := strings.TrimSpace(strings.ToLower(m.search.Value()))
	if query == "" {
		return problems
	}
	filtered := problems[:0]
	for _, problem := range problems {
		haystack := strings.ToLower(problem.Title + " " + problem.Slug + " " + strings.Join(problem.Tags, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, problem)
		}
	}
	return filtered
}

func (m Model) trackProgress(track catalog.Track) (solved, attempted, total int) {
	for _, slug := range track.Problems {
		total++
		if m.store == nil {
			continue
		}
		status, err := m.store.Progress(context.Background(), slug)
		if err != nil {
			continue
		}
		switch status {
		case storage.StatusSolved:
			solved++
		case storage.StatusAttempted, storage.StatusRevisiting:
			attempted++
		}
	}
	return solved, attempted, total
}

func (m Model) selectedStatus(problem catalog.Problem) storage.Status {
	if problem.Slug == "" || m.store == nil {
		return storage.StatusTodo
	}
	status, err := m.store.Progress(context.Background(), problem.Slug)
	if err != nil {
		return storage.StatusTodo
	}
	return status
}

func (m Model) render() string {
	if m.mode == ModeEditor {
		return m.renderEditor()
	}
	width, _, trackW, problemW, detailW, bodyH := m.layout()

	header := headerStyle.Width(width).Render("LazyLeet  " + mutedStyle.Render("local-first DSA practice"))
	tracks := m.renderTracks(trackW, bodyH)
	problems := m.renderProblems(problemW, bodyH)
	details := m.renderDetails(detailW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, tracks, problems, details)

	bar := m.renderBottom(width)
	if m.mode == ModeCommand {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderCommandPalette(width))
	}
	if m.mode == ModeHelp {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderHelp(width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, "", bar)
}

func (m Model) layout() (width, height, trackW, problemW, detailW, bodyH int) {
	width = max(m.width, defaultLayoutWidth)
	height = max(m.height, defaultLayoutHeight)
	available := max(totalPaneMinWidth(), width-paneGapWidth)
	widths := applyPaneDeltas(defaultPaneWidths(available), m.paneDeltas, available)
	trackW = widths[0]
	problemW = widths[1]
	detailW = widths[2]
	bodyH = max(6, height-chromeHeight)
	return width, height, trackW, problemW, detailW, bodyH
}

func defaultPaneWidths(available int) [3]int {
	available = max(totalPaneMinWidth(), available)
	extra := available - totalPaneMinWidth()
	trackW := trackMinWidth + extra/5
	problemW := problemMinWidth + (extra*2)/5
	detailW := available - trackW - problemW
	return [3]int{trackW, problemW, detailW}
}

func applyPaneDeltas(defaults, deltas [3]int, available int) [3]int {
	widths := [3]int{
		defaults[0] + deltas[0],
		defaults[1] + deltas[1],
		defaults[2] + deltas[2],
	}
	return normalizePaneWidths(widths, available)
}

func normalizePaneWidths(widths [3]int, available int) [3]int {
	available = max(totalPaneMinWidth(), available)
	for i, minWidth := range paneMinWidths {
		if widths[i] < minWidth {
			deficit := minWidth - widths[i]
			widths[i] = minWidth
			widths = takePaneWidth(widths, deficit, i)
		}
	}

	for sum := sumPaneWidths(widths); sum > available; sum = sumPaneWidths(widths) {
		removed := takePaneWidth(widths, sum-available, -1)
		if removed == widths {
			break
		}
		widths = removed
	}
	for sum := sumPaneWidths(widths); sum < available; sum = sumPaneWidths(widths) {
		widths[2] += available - sum
	}
	return widths
}

func takePaneWidth(widths [3]int, amount, exclude int) [3]int {
	for amount > 0 {
		donor := -1
		capacity := 0
		for i, width := range widths {
			if i == exclude {
				continue
			}
			if c := width - paneMinWidths[i]; c > capacity {
				donor = i
				capacity = c
			}
		}
		if donor == -1 || capacity == 0 {
			return widths
		}
		take := min(amount, capacity)
		widths[donor] -= take
		amount -= take
	}
	return widths
}

func resizeDonors(pane Pane) []int {
	switch pane {
	case TracksPane:
		return []int{int(ProblemsPane), int(DetailsPane)}
	case ProblemsPane:
		return []int{int(DetailsPane), int(TracksPane)}
	default:
		return []int{int(ProblemsPane), int(TracksPane)}
	}
}

func resizeReceivers(pane Pane) []int {
	switch pane {
	case TracksPane:
		return []int{int(ProblemsPane)}
	case ProblemsPane:
		return []int{int(DetailsPane)}
	default:
		return []int{int(ProblemsPane)}
	}
}

func sumPaneWidths(widths [3]int) int {
	return widths[0] + widths[1] + widths[2]
}

func totalPaneMinWidth() int {
	return trackMinWidth + problemMinWidth + detailMinWidth
}

func (m Model) renderTracks(width, height int) string {
	lines := panelLines(width, panelTitle("tracks", m.activePane == TracksPane))
	limit := panelBodyLimit(height)
	start := scrollStart(m.trackIndex, limit, len(m.catalog.Tracks))
	for i := start; i < len(m.catalog.Tracks) && len(lines) < limit+2; i++ {
		track := m.catalog.Tracks[i]
		solved, attempted, total := m.trackProgress(track)
		prefix := " "
		if i == m.trackIndex {
			prefix = ">"
		}
		line := fmt.Sprintf("%s %-15s %3d/%-3d", prefix, track.Title, solved, total)
		if attempted > 0 {
			line += fmt.Sprintf(" +%d", attempted)
		}
		lines = append(lines, line)
	}
	return renderPanel(width, height, strings.Join(lines, "\n"))
}

func (m Model) renderProblems(width, height int) string {
	lines := panelLines(width, panelTitle("problems", m.activePane == ProblemsPane))
	problems := m.visibleProblems()
	limit := panelBodyLimit(height)
	start := scrollStart(m.problemIdx, limit, len(problems))
	for i := start; i < len(problems) && len(lines) < limit+2; i++ {
		problem := problems[i]
		status := m.selectedStatus(problem)
		prefix := " "
		if i == m.problemIdx {
			prefix = ">"
		}
		line := fmt.Sprintf("%s %-9s %-6s %s", prefix, statusBadge(status), difficulty(problem.Difficulty), problem.Title)
		lines = append(lines, line)
	}
	if len(problems) == 0 {
		lines = append(lines, mutedStyle.Render("No problems match the current search."))
	}
	return renderPanel(width, height, strings.Join(lines, "\n"))
}

func (m Model) renderDetails(width, height int) string {
	problem := m.SelectedProblem()
	title := panelTitle("details", m.activePane == DetailsPane)
	if problem.Slug == "" {
		return renderTightPanel(width, height, strings.Join(append(panelLines(width, title), mutedStyle.Render("Select a problem to view details.")), "\n"))
	}
	lines := m.detailLines(width)
	bodyLimit := panelBodyLimit(height)
	maxScroll := max(0, len(lines)-bodyLimit)
	scroll := clamp(m.detailScroll, 0, maxScroll)
	visible := append(panelLines(width, title), lines[scroll:min(len(lines), scroll+bodyLimit)]...)
	if maxScroll > 0 {
		visible[0] = titleLine(title, mutedStyle.Render(fmt.Sprintf("%d/%d", scroll+1, maxScroll+1)), width)
	}
	return renderTightPanel(width, height, strings.Join(visible, "\n"))
}

func renderPanel(width, height int, content string) string {
	return panelStyle.Width(width).Height(height).MaxHeight(height).Render(fitPanelContent(content, width, height))
}

func renderTightPanel(width, height int, content string) string {
	return panelStyle.MarginRight(0).Width(width).Height(height).MaxHeight(height).Render(fitPanelContent(content, width, height))
}

func fitPanelContent(content string, width, height int) string {
	innerWidth := panelInnerWidth(width)
	innerHeight := max(1, height-2)
	lines := strings.Split(content, "\n")
	fitted := make([]string, 0, min(len(lines), innerHeight))
	for _, line := range lines {
		if len(fitted) == innerHeight {
			break
		}
		fitted = append(fitted, truncateLine(line, innerWidth))
	}
	return strings.Join(fitted, "\n")
}

func panelInnerWidth(width int) int {
	return max(1, width-4)
}

func titleLine(left, right string, width int) string {
	available := panelInnerWidth(width)
	padding := max(1, available-ansi.StringWidth(left)-ansi.StringWidth(right))
	return left + strings.Repeat(" ", padding) + right
}

func panelLines(width int, title string) []string {
	return []string{title, panelSeparator(width)}
}

func panelSeparator(width int) string {
	return mutedStyle.Render(strings.Repeat("─", panelInnerWidth(width)))
}

func panelBodyLimit(height int) int {
	return max(1, height-4)
}

func (m Model) detailLines(width int) []string {
	problem := m.SelectedProblem()
	if problem.Slug == "" {
		return nil
	}
	innerWidth := panelInnerWidth(width)
	linkPrefix := "External link: "
	urlText := truncateLine(problem.URL, max(1, innerWidth-ansi.StringWidth(linkPrefix)))
	lines := []string{
		titleStyle.Render(problem.Title),
		fmt.Sprintf("Difficulty: %s", difficulty(problem.Difficulty)),
		fmt.Sprintf("Status: %s", statusBadge(m.selectedStatus(problem))),
		"",
	}
	lines = append(lines, m.renderStatementPreview(problem, width)...)
	lines = append(lines, "", linkPrefix+urlStyle.Hyperlink(problem.URL).Render(urlText))
	return lines
}

func (m Model) renderTestCount(problem catalog.Problem) string {
	if m.tests == nil {
		return "Test cases: unavailable"
	}
	count, path, err := m.tests.CountTestCases(problem)
	if err != nil {
		return "Test cases: " + mutedStyle.Render("unavailable")
	}
	return fmt.Sprintf("Test cases: %d (%s)", count, path)
}

func (m Model) renderStatementPreview(problem catalog.Problem, width int) []string {
	if m.statements == nil {
		return []string{titleStyle.Render("Statement:"), mutedStyle.Render("Local statement workspace unavailable.")}
	}
	content, _, err := m.statements.ReadStatement(problem)
	if err != nil {
		return []string{titleStyle.Render("Statement:"), mutedStyle.Render("Could not read statement: " + err.Error())}
	}
	out := []string{titleStyle.Render("Statement:")}
	for _, line := range previewLines(content, max(12, panelInnerWidth(width)), 0) {
		out = append(out, line)
	}
	return out
}

func (m Model) renderBottom(width int) string {
	left := renderFooterCommands([]footerCommand{
		{Key: "j/k", Label: "scroll"},
		{Key: "tab", Label: "pane"},
		{Key: "^←/^→", Label: "resize"},
		{Key: "/", Label: "search"},
		{Key: "e", Label: "edit"},
		{Key: "r", Label: "run"},
		{Key: "m", Label: "mark"},
		{Key: "?", Label: "help"},
		{Key: "q", Label: "quit"},
	})
	if m.activePane == DetailsPane {
		left = renderFooterCommands([]footerCommand{
			{Key: "j/k", Label: "scroll"},
			{Key: "tab", Label: "pane"},
			{Key: "^←/^→", Label: "resize"},
			{Key: "/", Label: "search"},
			{Key: "e", Label: "edit"},
			{Key: "r", Label: "run"},
			{Key: "?", Label: "help"},
			{Key: "q", Label: "quit"},
		})
	}
	if m.mode == ModeSearch {
		left = m.search.View()
	}
	status := footerSeparatorStyle.Render(" | ") + currentLanguageStyle.Render(m.language.Title) + footerSeparatorStyle.Render(" | ") + footerLabelStyle.Render(m.statusLine)
	return footerStyle.Width(width).MaxHeight(1).Render(ansi.Truncate(left+status, max(1, width), ""))
}

type footerCommand struct {
	Key   string
	Label string
}

func renderFooterCommands(commands []footerCommand) string {
	parts := make([]string, 0, len(commands))
	for _, command := range commands {
		parts = append(parts, footerKeyStyle.Render(command.Key)+" "+footerLabelStyle.Render(command.Label))
	}
	return strings.Join(parts, footerSeparatorStyle.Render("  "))
}

func (m Model) renderCommandPalette(width int) string {
	lines := []string{"commands"}
	for i, command := range commands {
		prefix := " "
		if i == m.commandIndex {
			prefix = ">"
		}
		lines = append(lines, prefix+" "+command)
	}
	return overlayStyle.Width(min(48, width-4)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderHelp(width int) string {
	lines := []string{
		"help",
		"j/k, up/down    move selection",
		"details pane    j/k scroll statement",
		"tab             cycle panes",
		"ctrl+left/right shrink or widen active pane",
		"ctrl+0          reset pane widths",
		"/               search problems",
		"ctrl+p          command palette",
		"enter           select current item",
		"e               edit local solution",
		"r               run saved solution tests",
		"^w              switch editor pane",
		"^r              run examples or selected testcase",
		"^t              submit against all local tests",
		"^y              use failed submit testcase",
		"j/k             scroll focused editor pane",
		"^u/^d           page focused editor pane",
		"tab             insert indentation while editing",
		"enter           auto-indent while editing",
		"l               cycle language",
		"^s              format and save while editing",
		"m               cycle progress status",
		"o               open official URL",
		"ctrl/cmd-click  open URL link in supported terminals",
		"q/esc           quit or close overlay",
	}
	return overlayStyle.Width(min(64, width-4)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderEditor() string {
	width := max(m.width, 80)
	height := max(m.height, 20)
	m = m.resizeEditor()
	_, _, problemW, solutionW, bodyH := m.editorLayout(width, height)
	solutionH, outputH, rightGap, _ := editorRightHeights(bodyH)

	header := headerStyle.Width(width).Render("LazyLeet  " + mutedStyle.Render("editor"))
	problem := m.renderEditorProblem(problemW, bodyH)
	solution := m.renderEditorSolution(solutionW, solutionH)
	output := m.renderEditorOutput(solutionW, outputH)
	rightParts := []string{solution}
	for i := 0; i < rightGap; i++ {
		rightParts = append(rightParts, strings.Repeat(" ", max(1, solutionW)))
	}
	rightParts = append(rightParts, output)
	right := lipgloss.JoinVertical(lipgloss.Left, rightParts...)
	body := lipgloss.JoinHorizontal(lipgloss.Top, problem, right)
	bar := m.renderEditorBottom(width)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, "", bar)
}

func (m Model) renderEditorBottom(width int) string {
	left := renderFooterCommands([]footerCommand{
		{Key: "^w", Label: "pane"},
		{Key: "j/k", Label: "scroll"},
		{Key: "^←/^→", Label: "resize"},
		{Key: "^r", Label: "run"},
		{Key: "^t", Label: "submit"},
		{Key: "^s", Label: "save"},
		{Key: "esc", Label: "close"},
	})
	status := footerSeparatorStyle.Render(" | ") + footerLabelStyle.Render(m.statusLine)
	return footerStyle.Width(width).MaxHeight(1).Render(ansi.Truncate(left+status, max(1, width), ""))
}

func (m Model) resizeEditor() Model {
	_, _, _, solutionW, bodyH := m.editorLayout(max(m.width, 80), max(m.height, 20))
	solutionH, _, _, _ := editorRightHeights(bodyH)
	m.editor.SetWidth(max(20, solutionW-4))
	m.editor.SetHeight(panelBodyLimit(solutionH))
	return m
}

func (m Model) editorLayout(width, height int) (available, gap, problemW, solutionW, bodyH int) {
	gap = 1
	available = max(74, width-gap)
	problemW = max(32, (available*42)/100+m.editorPaneDelta)
	solutionW = available - problemW
	if solutionW < 40 {
		solutionW = 40
		problemW = max(32, available-solutionW)
	}
	if problemW < 32 {
		problemW = 32
		solutionW = available - problemW
	}
	bodyH = max(8, height-3)
	return available, gap, problemW, solutionW, bodyH
}

func (m Model) editorBodyHeight(height int) int {
	_, _, _, _, bodyH := m.editorLayout(max(m.width, 80), height)
	return bodyH
}

func editorRightHeights(bodyH int) (solutionH, outputH, gap, total int) {
	gap = 0
	outputH = clamp((bodyH*40)/100, 7, max(7, bodyH-7))
	solutionH = max(6, bodyH-outputH-gap)
	total = solutionH + gap + outputH
	if total > bodyH {
		outputH = max(4, bodyH-solutionH-gap)
		total = solutionH + gap + outputH
	}
	return solutionH, outputH, gap, total
}

func (m Model) renderEditorProblem(width, height int) string {
	title := panelTitle("problem", m.editorPane == EditorProblemPane)
	bodyLimit := panelBodyLimit(height)
	maxScroll := m.editorProblemMaxScroll(width, height)
	scroll := clamp(m.editorProblemScroll, 0, maxScroll)
	if maxScroll > 0 {
		title = titleLine(title, mutedStyle.Render(fmt.Sprintf("%d/%d", scroll+1, maxScroll+1)), width)
	}
	lines := panelLines(width, title)
	if m.editorProblem.Slug == "" {
		lines = append(lines, mutedStyle.Render("No problem selected."))
		return renderPanel(width, height, strings.Join(lines, "\n"))
	}
	lines = append(lines, titleStyle.Render(m.editorProblem.Title), "")
	statementLimit := max(1, bodyLimit-2)
	statementLines := m.editorStatementLines(width)
	lines = append(lines, statementLines[scroll:min(len(statementLines), scroll+statementLimit)]...)
	return renderPanel(width, height, strings.Join(lines, "\n"))
}

func (m Model) editorProblemMaxScroll(width, height int) int {
	bodyLimit := panelBodyLimit(height)
	statementLimit := max(1, bodyLimit-2)
	return max(0, len(m.editorStatementLines(width))-statementLimit)
}

func (m Model) editorStatementLines(width int) []string {
	if m.statements == nil {
		return []string{mutedStyle.Render("Local statement workspace unavailable.")}
	}
	content, _, err := m.statements.ReadStatement(m.editorProblem)
	if err != nil {
		return []string{mutedStyle.Render("Could not read statement: " + err.Error())}
	}
	return previewLines(content, max(12, width-6), 0)
}

func (m Model) renderEditorSolution(width, height int) string {
	lines := panelLines(width, panelTitle("solution", m.editorPane == EditorSolutionPane))
	lines = append(lines, m.editor.View())
	return renderTightPanel(width, height, strings.Join(lines, "\n"))
}

func (m Model) renderEditorOutput(width, height int) string {
	title := panelTitle("output", m.editorPane == EditorOutputPane)
	bodyLines := m.editorOutputLines(width)
	bodyLimit := panelBodyLimit(height)
	maxScroll := max(0, len(bodyLines)-bodyLimit)
	scroll := clamp(m.editorOutputScroll, 0, maxScroll)
	if maxScroll > 0 {
		title = titleLine(title, mutedStyle.Render(fmt.Sprintf("%d/%d", scroll+1, maxScroll+1)), width)
	}
	lines := append(panelLines(width, title), bodyLines[scroll:min(len(bodyLines), scroll+bodyLimit)]...)
	return renderTightPanel(width, height, strings.Join(lines, "\n"))
}

func (m Model) editorOutputMaxScroll(width, height int) int {
	return max(0, len(m.editorOutputLines(width))-panelBodyLimit(height))
}

func (m Model) editorOutputLines(width int) []string {
	wrapWidth := max(12, width-6)
	if m.testRunning {
		if m.lastRunMode == workspace.TestRunAll {
			return []string{mutedStyle.Render("Submitting against all local test cases...")}
		}
		return []string{mutedStyle.Render("Running examples...")}
	}
	if !m.hasLastRun {
		return []string{mutedStyle.Render("Run examples with ^r. Submit all local tests with ^t.")}
	}
	if m.lastRunErr != "" {
		return append([]string{hardStyle.Render("Run error")}, previewLines(m.lastRunErr, wrapWidth, 0)...)
	}
	result := m.lastRunResult
	lines := []string{runResultHeading(m.lastRunMode)}
	if m.debugCase != nil && m.debugCaseProblem == m.editorProblem.Slug && m.lastRunMode == workspace.TestRunCustom {
		lines = append(lines, mutedStyle.Render("Selected testcase"))
	}
	if result.TimedOut {
		lines = append(lines, hardStyle.Render("Time Limit Exceeded"))
		if result.TimeLimit > 0 {
			lines = append(lines, "Time limit: "+result.TimeLimit.String())
		}
		if result.Passed > 0 || result.Total > 0 {
			lines = append(lines, fmt.Sprintf("Progress before timeout: %d/%d passed", result.Passed, result.Total))
		}
	} else if result.Total > 0 && result.Passed == result.Total {
		lines = append(lines, fmt.Sprintf("Result: %d/%d passed", result.Passed, result.Total), easyStyle.Render("Accepted"))
	} else if len(result.Failures) > 0 {
		failure := result.Failures[0]
		lines = append(lines,
			hardStyle.Render(fmt.Sprintf("Failed case %d", failure.Index)),
			titleStyle.Render("Input"),
		)
		lines = append(lines, previewLines(failure.Input, wrapWidth, 0)...)
		lines = append(lines,
			titleStyle.Render("Output"),
		)
		lines = append(lines, previewLines(failure.Actual, wrapWidth, 0)...)
		lines = append(lines,
			titleStyle.Render("Expected"),
		)
		lines = append(lines, previewLines(failure.Expected, wrapWidth, 0)...)
		if m.lastRunMode == workspace.TestRunAll && len(failure.Case.Input) > 0 {
			lines = append(lines, "", mutedStyle.Render("^y use this testcase for Run"))
		}
	}
	if output := visibleRunnerOutput(result.Output); output != "" {
		lines = append(lines, "", titleStyle.Render("Runner output"))
		lines = append(lines, previewLines(output, wrapWidth, 0)...)
	}
	return lines
}

func runResultHeading(mode workspace.TestRunMode) string {
	switch mode {
	case workspace.TestRunAll:
		return titleStyle.Render("Submit")
	case workspace.TestRunCustom:
		return titleStyle.Render("Run")
	default:
		return titleStyle.Render("Run")
	}
}

func visibleRunnerOutput(output string) string {
	lines := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "LAZYLEET_RESULT ") || strings.HasPrefix(line, "LAZYLEET_FAIL\t") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func panelTitle(title string, active bool) string {
	if active {
		return activeTitleStyle.Render(strings.ToUpper(title))
	}
	return inactiveTitleStyle.Render(strings.ToUpper(title))
}

func statusBadge(status storage.Status) string {
	switch status {
	case storage.StatusSolved:
		return solvedStyle.Render("solved")
	case storage.StatusAttempted:
		return attemptedStyle.Render("attempt")
	case storage.StatusRevisiting:
		return revisitStyle.Render("revisit")
	case storage.StatusSkipped:
		return skippedStyle.Render("skip")
	default:
		return mutedStyle.Render("todo")
	}
}

func difficulty(d catalog.Difficulty) string {
	switch d {
	case catalog.Easy:
		return easyStyle.Render(string(d))
	case catalog.Medium:
		return mediumStyle.Render(string(d))
	case catalog.Hard:
		return hardStyle.Render(string(d))
	default:
		return string(d)
	}
}

func scrollStart(index, limit, total int) int {
	if total <= limit || index < limit {
		return 0
	}
	start := index - limit + 1
	if start+limit > total {
		return max(0, total-limit)
	}
	return start
}

func clamp(v, low, high int) int {
	if high < low {
		return low
	}
	return min(max(v, low), high)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func previewLines(content string, width, limit int) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	raw := strings.Split(content, "\n")
	lines := make([]string, 0, max(0, limit))
	for _, rawLine := range raw {
		line := strings.TrimSpace(rawLine)
		if line == "" && len(lines) == 0 {
			continue
		}
		wrapped := wrapLine(line, width)
		for _, wrappedLine := range wrapped {
			lines = append(lines, wrappedLine)
			if limit > 0 && len(lines) == limit {
				break
			}
		}
		if limit > 0 && len(lines) == limit {
			break
		}
	}
	if limit > 0 && len(lines) == limit && len(raw) > 0 {
		lines = append(lines, mutedStyle.Render("..."))
	}
	if len(lines) == 0 {
		return []string{mutedStyle.Render("No local statement content yet.")}
	}
	return lines
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	wrapped := strings.Split(ansi.Wordwrap(line, width, " "), "\n")
	out := make([]string, 0, len(wrapped))
	for _, part := range wrapped {
		out = append(out, hardWrapPlainLine(part, width)...)
	}
	return out
}

func hardWrapPlainLine(line string, width int) []string {
	if width <= 0 || ansi.StringWidth(line) <= width || strings.Contains(line, "\x1b[") {
		return []string{line}
	}
	runes := []rune(line)
	lines := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func truncateLine(line string, width int) string {
	if ansi.StringWidth(line) <= width {
		return line
	}
	if width <= 3 {
		return ansi.Truncate(line, max(0, width), "")
	}
	return ansi.Truncate(line, width, "...")
}
