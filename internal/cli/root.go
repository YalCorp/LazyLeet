package cli

import (
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/YalCorp/LazyLeet/internal/catalog"
	"github.com/YalCorp/LazyLeet/internal/config"
	"github.com/YalCorp/LazyLeet/internal/storage"
	"github.com/YalCorp/LazyLeet/internal/tui"
	"github.com/YalCorp/LazyLeet/internal/version"
	"github.com/YalCorp/LazyLeet/internal/workspace"
)

type Runner func(tea.Model) error

type options struct {
	runner Runner
}

type Option func(*options)

func WithRunner(runner Runner) Option {
	return func(opts *options) {
		opts.runner = runner
	}
}

func NewRootCommand(opts ...Option) *cobra.Command {
	cfg := options{runner: runProgram}
	for _, opt := range opts {
		opt(&cfg)
	}

	cmd := &cobra.Command{
		Use:           "lazyleet",
		Short:         "A keyboard-first terminal companion for DSA practice",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cfg.runner)
		},
	}
	cmd.AddCommand(newVersionCommand())
	return cmd
}

func runTUI(runner Runner) error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	if err := config.EnsureAppDir(paths); err != nil {
		return err
	}
	appConfig, err := config.Load(paths)
	if err != nil {
		return err
	}
	store, err := storage.OpenSQLite(appConfig.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()

	c, err := catalog.Load()
	if err != nil {
		return err
	}
	solutions := workspace.New(appConfig.WorkspacePath)
	return runner(tui.NewModel(c, store, tui.WithSolutionStore(solutions), tui.WithStatementStore(solutions)))
}

func runProgram(model tea.Model) error {
	_, err := tea.NewProgram(model).Run()
	return err
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := version.Get()
			out := cmd.OutOrStdout()
			writeVersion(out, info)
			return nil
		},
	}
}

func writeVersion(out io.Writer, info version.Info) {
	fmt.Fprintf(out, "lazyleet %s\ncommit: %s\ndate: %s\ngo: %s\n", info.Version, info.Commit, info.Date, info.Go)
}
