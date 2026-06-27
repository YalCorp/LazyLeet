package tui

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"
)

type urlOpenedMsg struct {
	url string
	err error
}

type URLOpener func(url string) tea.Cmd

func openURLCommand(url string) tea.Cmd {
	cmd, err := browserCommand(url)
	if err != nil {
		return func() tea.Msg {
			return urlOpenedMsg{url: url, err: err}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return urlOpenedMsg{url: url, err: err}
	})
}

func browserCommand(url string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url), nil
	case "windows":
		return exec.Command("cmd", "/c", "start", "", url), nil
	case "linux":
		return exec.Command("xdg-open", url), nil
	default:
		return nil, fmt.Errorf("opening URLs is not supported on %s", runtime.GOOS)
	}
}
