package tui

import (
	"context"
	"fmt"
	"go/format"
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
}

type PaneLayoutStore interface {
	SavePaneDeltas(deltas [3]int) error
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
	language            workspace.Language
	editorPath          string
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
	case urlOpenedMsg:
		if msg.err != nil {
			m.statusLine = "Could not open URL: " + msg.err.Error()
		} else {
			m.statusLine = "Opened " + msg.url
		}
		return m, nil
	}
	return m, nil
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
	case "[":
		m = m.resizeActivePane(-paneResizeStep)
	case "]":
		m = m.resizeActivePane(paneResizeStep)
	case "0":
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
	case "ctrl+w":
		m = m.toggleEditorPane()
		return m, nil
	case "ctrl+u":
		m = m.scrollEditorProblem(-editorProblemScrollStep(m))
		return m, nil
	case "ctrl+d":
		m = m.scrollEditorProblem(editorProblemScrollStep(m))
		return m, nil
	case "[":
		m = m.resizeEditorPane(-paneResizeStep)
		return m.resizeEditor(), nil
	case "]":
		m = m.resizeEditorPane(paneResizeStep)
		return m.resizeEditor(), nil
	case "0":
		m.editorPaneDelta = 0
		m.statusLine = "Editor panes reset"
		return m.resizeEditor(), nil
	}

	if m.editorPane == EditorProblemPane {
		switch key {
		case "up", "k":
			m = m.scrollEditorProblem(-1)
		case "down", "j":
			m = m.scrollEditorProblem(1)
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
	m.editorPane = EditorSolutionPane
	m.editorPath = path
	m.editor.SetValue(content)
	m.mode = ModeEditor
	m.statusLine = "Editing " + path
	m = m.resizeEditor()
	cmd := m.editor.Focus()
	return m, cmd
}

func (m Model) saveEditor() Model {
	if m.solutions == nil || m.editorProblem.Slug == "" {
		m.statusLine = "No solution file open"
		return m
	}
	content, formatted := formatEditorContent(m.editor.Value(), m.language)
	if formatted {
		m.editor.SetValue(content)
	}
	path, err := m.solutions.SaveSolution(m.editorProblem, m.language, content)
	if err != nil {
		m.statusLine = "Save error: " + err.Error()
		return m
	}
	m.editorPath = path
	if formatted {
		m.statusLine = "Formatted and saved " + path
	} else {
		m.statusLine = "Saved " + path
	}
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

func formatEditorContent(content string, language workspace.Language) (string, bool) {
	cleaned := trimTrailingLineWhitespace(content)
	switch language.ID {
	case "go":
		formatted, err := format.Source([]byte(cleaned))
		if err == nil {
			cleaned = string(formatted)
		}
	}
	return cleaned, cleaned != content
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

func (m Model) toggleEditorPane() Model {
	if m.editorPane == EditorProblemPane {
		m.editorPane = EditorSolutionPane
		m.statusLine = "Solution focused"
		return m
	}
	m.editorPane = EditorProblemPane
	m.statusLine = "Problem focused"
	return m
}

func (m Model) resizeEditorPane(delta int) Model {
	if m.editorPane == EditorSolutionPane {
		delta = -delta
	}
	m.editorPaneDelta += delta
	m.editorPaneDelta = clamp(m.editorPaneDelta, -40, 40)
	if m.editorPane == EditorProblemPane {
		m.statusLine = "Problem pane resized"
	} else {
		m.statusLine = "Solution pane resized"
	}
	return m
}

func editorProblemScrollStep(m Model) int {
	_, _, _, _, bodyH := m.editorLayout(max(m.width, 80), max(m.height, 20))
	return max(1, panelBodyLimit(bodyH)/2)
}

func (m Model) cycleLanguage() Model {
	m.language = workspace.NextLanguage(m.language)
	m.statusLine = "Language: " + m.language.Title
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
	m.statusLine = fmt.Sprintf("Pane widths: tracks %d, problems %d, details %d", widths[0], widths[1], widths[2])
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
		return renderPanel(width, height, strings.Join(append(panelLines(width, title), mutedStyle.Render("Select a problem to view details.")), "\n"))
	}
	lines := m.detailLines(width)
	bodyLimit := panelBodyLimit(height)
	maxScroll := max(0, len(lines)-bodyLimit)
	scroll := clamp(m.detailScroll, 0, maxScroll)
	visible := append(panelLines(width, title), lines[scroll:min(len(lines), scroll+bodyLimit)]...)
	if maxScroll > 0 {
		visible[0] = titleLine(title, mutedStyle.Render(fmt.Sprintf("%d/%d", scroll+1, maxScroll+1)), width)
	}
	return renderPanel(width, height, strings.Join(visible, "\n"))
}

func renderPanel(width, height int, content string) string {
	return panelStyle.Width(width).Height(height).MaxHeight(height).Render(content)
}

func renderTightPanel(width, height int, content string) string {
	return panelStyle.MarginRight(0).Width(width).Height(height).MaxHeight(height).Render(content)
}

func titleLine(left, right string, width int) string {
	available := max(1, width-4)
	padding := max(1, available-ansi.StringWidth(left)-ansi.StringWidth(right))
	return left + strings.Repeat(" ", padding) + right
}

func panelLines(width int, title string) []string {
	return []string{title, panelSeparator(width)}
}

func panelSeparator(width int) string {
	return mutedStyle.Render(strings.Repeat("─", max(1, width-4)))
}

func panelBodyLimit(height int) int {
	return max(1, height-4)
}

func (m Model) detailLines(width int) []string {
	problem := m.SelectedProblem()
	if problem.Slug == "" {
		return nil
	}
	lines := []string{
		titleStyle.Render(problem.Title),
		fmt.Sprintf("ID: %d", problem.ID),
		fmt.Sprintf("Difficulty: %s", difficulty(problem.Difficulty)),
		fmt.Sprintf("Status: %s", statusBadge(m.selectedStatus(problem))),
		"URL: " + urlStyle.Hyperlink(problem.URL).Render(truncateLine(problem.URL, max(8, width-7))),
		"",
	}
	lines = append(lines, m.renderStatementPreview(problem, width)...)
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
	for _, line := range previewLines(content, max(12, width-6), 0) {
		out = append(out, line)
	}
	return out
}

func (m Model) renderBottom(width int) string {
	left := renderFooterCommands([]footerCommand{
		{Key: "j/k", Label: "scroll"},
		{Key: "tab", Label: "pane"},
		{Key: "[ ]", Label: "resize"},
		{Key: "/", Label: "search"},
		{Key: "e", Label: "edit"},
		{Key: "m", Label: "mark"},
		{Key: "?", Label: "help"},
		{Key: "q", Label: "quit"},
	})
	if m.activePane == DetailsPane {
		left = renderFooterCommands([]footerCommand{
			{Key: "j/k", Label: "scroll"},
			{Key: "tab", Label: "pane"},
			{Key: "[ ]", Label: "resize"},
			{Key: "/", Label: "search"},
			{Key: "e", Label: "edit"},
			{Key: "?", Label: "help"},
			{Key: "q", Label: "quit"},
		})
	}
	if m.mode == ModeSearch {
		left = m.search.View()
	}
	status := footerSeparatorStyle.Render(" | ") + footerLabelStyle.Render(m.statusLine)
	return footerStyle.Width(width).MaxHeight(1).Render(ansi.Truncate(left+status, max(1, width-2), ""))
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
		"[, ]            shrink or widen active pane",
		"0               reset pane widths",
		"/               search problems",
		"ctrl+p          command palette",
		"enter           select current item",
		"e               edit local solution",
		"ctrl+w          switch editor pane",
		"j/k             scroll focused problem pane",
		"ctrl+u/d        page focused problem pane",
		"tab             insert indentation while editing",
		"enter           auto-indent while editing",
		"l               cycle language",
		"ctrl+s          format and save while editing",
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

	title := "LazyLeet  " + mutedStyle.Render("editor")
	if m.editorProblem.Title != "" {
		title += "  " + m.editorProblem.Title + "  " + mutedStyle.Render(m.language.Title)
	}
	header := headerStyle.Width(width).Render(title)
	path := mutedStyle.Width(width).Render(m.editorPath)
	problem := m.renderEditorProblem(problemW, bodyH)
	solution := m.renderEditorSolution(solutionW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, problem, solution)
	footer := "ctrl+w pane  j/k problem scroll  [ ] resize  0 reset  ctrl+s save  tab indent  enter auto-indent  esc close  |  " + m.statusLine
	bar := footerStyle.Width(width).MaxHeight(1).Render(ansi.Truncate(footer, max(1, width-2), ""))
	return lipgloss.JoinVertical(lipgloss.Left, header, path, body, "", bar)
}

func (m Model) resizeEditor() Model {
	_, _, _, solutionW, bodyH := m.editorLayout(max(m.width, 80), max(m.height, 20))
	m.editor.SetWidth(max(20, solutionW-4))
	m.editor.SetHeight(panelBodyLimit(bodyH))
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
	bodyH = max(8, height-4)
	return available, gap, problemW, solutionW, bodyH
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
	return strings.Split(ansi.Wordwrap(line, width, " "), "\n")
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
