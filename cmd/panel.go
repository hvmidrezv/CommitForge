// Package cmd defines the CLI commands for CommitForge using Cobra.
package cmd

import (
	"fmt"

	"commitforge/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var panelCmd = &cobra.Command{
	Use:   "panel",
	Short: "Open the interactive TUI contribution panel",
	Long:  `Launch the CommitForge TUI panel to navigate the contribution grid and generate commits.`,
	RunE:  runPanel,
}

func init() {
	rootCmd.AddCommand(panelCmd)
}

// runPanel starts the bubbletea TUI, forwarding CLI flags to the model.
func runPanel(cmd *cobra.Command, _ []string) error {
	flags := cmd.Root().PersistentFlags()

	years, err := flags.GetInt("years")
	if err != nil || years < 1 {
		years = 1
	}

	dir, _ := flags.GetString("dir")
	if dir == "" {
		dir = "output"
	}
	message, _ := flags.GetString("message")
	messageMode, _ := flags.GetString("message-mode")
	remote, _ := flags.GetString("remote")
	noPush, _ := flags.GetBool("no-push")
	yes, _ := flags.GetBool("yes")

	cfg := tui.Config{
		Dir:         dir,
		Message:     message,
		MessageMode: messageMode,
		Remote:      remote,
		NoPush:      noPush,
		Yes:         yes,
	}

	p := tea.NewProgram(tui.NewModel(years, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("panel error: %w", err)
	}
	return nil
}
