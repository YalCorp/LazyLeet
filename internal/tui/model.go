package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/storage"
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
)

type ProgressStore interface {
	Progress(ctx context.Context, slug string) (storage.Status, error)
	SetProgress(ctx context.Context, slug string, status storage.Status) error
}

type Model struct {
	catalog catalog.Catalog
	store   ProgressStore
	openURL URLOpener

	width  int
	height int

	activePane Pane
	trackIndex int
	problemIdx int

	mode         Mode
	search       textinput.Model
	commandIndex int
	statusLine   string
}

var commands = []string{
	"Go to Track",
	"Search Problem",
	"Mark Solved",
	"Open Official URL",
	"Open Notes",
	"Show Help",
}

func NewModel(c catalog.Catalog, store ProgressStore) Model {
	search := textinput.New()
	search.Prompt = "/ "
	search.SetWidth(36)
	return Model{
		catalog:    c,
		store:      store,
		openURL:    openURLCommand,
		width:      110,
		height:     32,
		activePane: ProblemsPane,
		mode:       ModeBrowse,
		search:     search,
		statusLine: "Ready",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
		"URL: "+urlStyle.Hyperlink(problem.URL).Render(problem.URL),
		urlHintStyle.Render("Use ctrl + click to open in browser"),
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

func (m Model) renderBottom(width int) string {
	left := "j/k move  tab pane  / search  ctrl+p commands  m mark  o open URL  ? help  q quit"
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
		"m               cycle progress status",
		"o               open official URL",
		"ctrl/cmd-click  open URL link in supported terminals",
		"q/esc           quit or close overlay",
	}
	return overlayStyle.Width(min(64, width-4)).Render(strings.Join(lines, "\n"))
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
