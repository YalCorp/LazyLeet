package tui

import "charm.land/lipgloss/v2"

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Background(lipgloss.Color("#2E3440")).
			Padding(0, 1)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4C566A")).
			Padding(0, 1).
			MarginRight(1)
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ECEFF4")).
			Background(lipgloss.Color("#3B4252")).
			Padding(0, 1)
	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#A3BE8C")).
			Padding(0, 1).
			MarginTop(1)
	activeTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#88C0D0")).
				Bold(true)
	inactiveTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D8DEE9"))
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EBCB8B")).
			Bold(true)
	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88C0D0")).
			Underline(true)
	urlHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89929E")).
			Italic(true)
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89929E"))
	easyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A3BE8C"))
	mediumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EBCB8B"))
	hardStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BF616A"))
	solvedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A3BE8C"))
	attemptedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EBCB8B"))
	revisitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B48EAD"))
	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D08770"))
)
