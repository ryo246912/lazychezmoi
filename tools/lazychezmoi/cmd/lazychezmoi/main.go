package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"lazychezmoi/internal/chezmoi"
	"lazychezmoi/internal/ui"
)

var version = "dev"

func main() {
	var (
		source      string
		destination string
		exclude     []string
		chezmoiBin  string
	)

	root := &cobra.Command{
		Use:   "lazychezmoi",
		Short: "A TUI for chezmoi inspired by lazygit/gitui",
		Long: `lazychezmoi is a terminal UI for chezmoi that provides a
gitui-style review-and-apply workflow for your dotfiles.`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := chezmoi.New(chezmoiBin, source, destination, exclude)
			m := ui.New(client)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil
		},
	}

	flags := root.Flags()
	flags.StringVar(&source, "source", "", "chezmoi source directory (default: auto-detected)")
	flags.StringVar(&destination, "destination", "", "chezmoi destination directory (default: $HOME)")
	flags.StringSliceVar(&exclude, "exclude", nil, "comma-separated list of types to exclude (e.g. templates,scripts)")
	flags.StringVar(&chezmoiBin, "chezmoi-bin", "chezmoi", "path to chezmoi binary")

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
