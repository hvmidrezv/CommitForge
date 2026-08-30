// Package cmd defines the CLI commands for CommitForge using Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "commitforge",
	Short: "CommitForge — fill your GitHub contribution graph with backdated commits",
	Long: `CommitForge is a TUI application that generates fake Git commits
with backdated timestamps to fill in a GitHub-style contribution graph.

Run 'commitforge panel' to open the interactive TUI.`,
	// Running the root command with no subcommand launches the panel.
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPanel(cmd, args)
	},
}

// Execute runs the root Cobra command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("dir", "d", "output", "Target output/working directory")
	rootCmd.PersistentFlags().Int("years", 1, "How many years back to render in the grid")
	rootCmd.PersistentFlags().String("message", "", "Fixed commit message (overrides message-mode)")
	rootCmd.PersistentFlags().String("message-mode", "random", "Commit message mode: random or fixed")
	rootCmd.PersistentFlags().String("remote", "", "Pre-fill remote URL, skip prompt")
	rootCmd.PersistentFlags().Bool("no-push", false, "Skip push flow, generate locally only")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "Auto-confirm prompts")
}
