package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

type Model struct {
	catalog    catalog.Catalog
	store      ProgressStore
	solutions  SolutionStore
	statements StatementStore
	openURL    URLOpener

	width  int
	height int

	activePane Pane
	trackIndex int
	problemIdx int

	mode          Mode
	search        textinput.Model
	editor        textarea.Model
	editorProblem catalog.Problem
	language      workspace.Language
	editorPath    string
	commandIndex  int
	statusLine    string
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
	case "up", "k":
		m = m.move(-1)
	case "down", "j":
		m = m.move(1)
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
	switch msg.String() {
	case "esc":
		m.editor.Blur()
		m.mode = ModeBrowse
		m.statusLine = "Editor closed"
		return m, nil
	case "ctrl+s":
		m = m.saveEditor()
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
	path, err := m.solutions.SaveSolution(m.editorProblem, m.language, m.editor.Value())
	if err != nil {
		m.statusLine = "Save error: " + err.Error()
		return m
	}
	m.editorPath = path
	m.statusLine = "Saved " + path
	return m
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
	case ProblemsPane, DetailsPane:
		problems := m.visibleProblems()
		m.problemIdx = clamp(m.problemIdx+delta, 0, len(problems)-1)
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
	width := max(m.width, 96)
	height := max(m.height, 24)
	trackW := max(24, width/5)
	problemW := max(42, width*2/5)
	detailW := max(30, width-trackW-problemW-6)
	bodyH := max(12, height-5)

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
	return lipgloss.JoinVertical(lipgloss.Left, header, body, bar)
}

func (m Model) renderTracks(width, height int) string {
	lines := []string{panelTitle("tracks", m.activePane == TracksPane)}
	for i, track := range m.catalog.Tracks {
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
	return panelStyle.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderProblems(width, height int) string {
	lines := []string{panelTitle("problems", m.activePane == ProblemsPane)}
	problems := m.visibleProblems()
	limit := max(1, height-3)
	start := scrollStart(m.problemIdx, limit, len(problems))
	for i := start; i < len(problems) && len(lines) <= limit; i++ {
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
	return panelStyle.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDetails(width, height int) string {
	problem := m.SelectedProblem()
	lines := []string{panelTitle("details", m.activePane == DetailsPane)}
	if problem.Slug == "" {
		lines = append(lines, mutedStyle.Render("Select a problem to view details."))
		return panelStyle.Width(width).Height(height).Render(strings.Join(lines, "\n"))
	}
	lines = append(lines,
		titleStyle.Render(problem.Title),
		fmt.Sprintf("ID: %d", problem.ID),
		fmt.Sprintf("Difficulty: %s", difficulty(problem.Difficulty)),
		fmt.Sprintf("Status: %s", statusBadge(m.selectedStatus(problem))),
		fmt.Sprintf("Language: %s", m.language.Title),
		"URL: "+urlStyle.Hyperlink(problem.URL).Render(problem.URL),
		urlHintStyle.Render("Use ctrl + click to open in browser"),
		"",
	)
	lines = append(lines, m.renderStatementPreview(problem, width, height, len(lines))...)
	lines = append(lines,
		"",
		"Patterns:",
	)
	patterns := append([]string(nil), problem.Patterns...)
	sort.Strings(patterns)
	for _, pattern := range patterns {
		lines = append(lines, "  "+pattern)
	}
	lines = append(lines, "", "Tracks:")
	for _, track := range problem.Tracks {
		lines = append(lines, "  "+track)
	}
	return panelStyle.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) renderStatementPreview(problem catalog.Problem, width, height, used int) []string {
	if m.statements == nil {
		return []string{"Statement:", "  " + mutedStyle.Render("Local statement workspace unavailable.")}
	}
	content, path, err := m.statements.ReadStatement(problem)
	if err != nil {
		return []string{"Statement:", "  " + mutedStyle.Render("Could not read statement: "+err.Error())}
	}
	available := max(3, height-used-11)
	out := []string{"Statement:", "  " + mutedStyle.Render(path)}
	for _, line := range previewLines(content, max(12, width-6), available) {
		out = append(out, "  "+line)
	}
	return out
}

func (m Model) renderBottom(width int) string {
	left := "j/k move  tab pane  / search  ctrl+p commands  e edit  l lang  m mark  o open URL  ? help  q quit"
	if m.mode == ModeSearch {
		left = m.search.View()
	}
	return footerStyle.Width(width).Render(left + "  |  " + m.statusLine)
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
		"tab             cycle panes",
		"/               search problems",
		"ctrl+p          command palette",
		"enter           select current item",
		"e               edit local solution",
		"l               cycle language",
		"ctrl+s          save while editing",
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

	title := "LazyLeet  " + mutedStyle.Render("editor")
	if m.editorProblem.Title != "" {
		title += "  " + m.editorProblem.Title + "  " + mutedStyle.Render(m.language.Title)
	}
	header := headerStyle.Width(width).Render(title)
	path := mutedStyle.Width(width).Render(m.editorPath)
	body := editorStyle.Width(max(20, width-4)).Height(max(8, height-6)).Render(m.editor.View())
	bar := footerStyle.Width(width).Render("ctrl+s save  esc browser  ctrl+c quit  |  " + m.statusLine)
	return lipgloss.JoinVertical(lipgloss.Left, header, path, body, bar)
}

func (m Model) resizeEditor() Model {
	m.editor.SetWidth(max(20, m.width-6))
	m.editor.SetHeight(max(6, m.height-8))
	return m
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
	raw := strings.Split(content, "\n")
	lines := make([]string, 0, min(len(raw), limit))
	for _, line := range raw {
		line = strings.TrimRight(line, " \t")
		if line == "" && len(lines) == 0 {
			continue
		}
		lines = append(lines, truncateLine(line, width))
		if len(lines) == limit {
			break
		}
	}
	if len(raw) > limit {
		lines = append(lines, mutedStyle.Render("..."))
	}
	if len(lines) == 0 {
		return []string{mutedStyle.Render("No local statement content yet.")}
	}
	return lines
}

func truncateLine(line string, width int) string {
	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	if width <= 3 {
		return string(runes[:max(0, width)])
	}
	return string(runes[:width-3]) + "..."
}
