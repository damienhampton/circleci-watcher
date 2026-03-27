package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/damien/circleci-watch/internal/api"
	"github.com/damien/circleci-watch/internal/git"
	"github.com/damien/circleci-watch/internal/ui"
)

var (
	flagProject  string
	flagBranch   string
	flagToken    string
	flagRefresh  time.Duration
	flagLimit    int
	flagDebug    string
	flagNoNotify bool
)

var rootCmd = &cobra.Command{
	Use:   "circleci-watch",
	Short: "Live-updating CircleCI pipeline monitor",
	Long: `circleci-watch is a terminal UI for monitoring CircleCI pipelines in real time.

It auto-detects your project from the current git remote, polls the CircleCI API,
and displays pipelines, workflows, jobs and inline failure details — updated live.

Auth: set CIRCLECI_TOKEN env var or pass --token.`,
	RunE: run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&flagProject, "project", "", "CircleCI project slug (e.g. gh/myorg/myrepo). Auto-detected from git remote if omitted.")
	rootCmd.Flags().StringVar(&flagBranch, "branch", "", "Filter pipelines to a specific branch")
	rootCmd.Flags().StringVar(&flagToken, "token", "", "CircleCI API token (or set CIRCLECI_TOKEN env var)")
	rootCmd.Flags().DurationVar(&flagRefresh, "refresh", 5*time.Second, "Polling interval (e.g. 5s, 10s)")
	rootCmd.Flags().IntVar(&flagLimit, "limit", 10, "Number of recent pipelines to show")
	rootCmd.Flags().StringVar(&flagDebug, "debug", "", "Write raw API responses to this file for debugging (e.g. /tmp/cw-debug.log)")
	rootCmd.Flags().BoolVar(&flagNoNotify, "no-notify", false, "Disable desktop notifications")
}

func run(cmd *cobra.Command, args []string) error {
	token := flagToken
	if token == "" {
		token = os.Getenv("CIRCLECI_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("CircleCI API token required: set CIRCLECI_TOKEN env var or pass --token")
	}

	projectSlug := flagProject
	if projectSlug == "" {
		var err error
		projectSlug, err = git.DetectProjectSlug()
		if err != nil {
			return fmt.Errorf("could not detect project from git remote: %w\nUse --project to specify the project slug (e.g. gh/myorg/myrepo)", err)
		}
	}

	client := api.NewClient(token, flagDebug)
	model := ui.NewModel(client, projectSlug, flagBranch, flagLimit, flagRefresh, !flagNoNotify)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
