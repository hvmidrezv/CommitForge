// Package cmd defines the CLI commands for CommitForge using Cobra.
package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		writeRootHelp(cmd.OutOrStdout())
	})
}

func writeRootHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, "CommitForge — fake contribution graph generator")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  commitforge [flags]")
	_, _ = fmt.Fprintln(w, "  commitforge panel [flags]")
	_, _ = fmt.Fprintln(w, "  commitforge help")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "How the TUI works (quick summary):")
	_, _ = fmt.Fprintln(w, "  1. Navigate the contribution grid and select days.")
	_, _ = fmt.Fprintln(w, "  2. Assign fixed count or random range to selected days.")
	_, _ = fmt.Fprintln(w, "  3. Choose an action: push, preview, generate locally, etc.")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Flags:")
	_, _ = fmt.Fprintln(w, "  --dir <path>          Target output/working directory")
	_, _ = fmt.Fprintln(w, "  --years <n>           How many years back to render in the grid")
	_, _ = fmt.Fprintln(w, "  --message <text>      Fixed commit message")
	_, _ = fmt.Fprintln(w, "  --message-mode <mode> random | fixed")
	_, _ = fmt.Fprintln(w, "  --remote <url>        Pre-fill remote URL and skip prompt when possible")
	_, _ = fmt.Fprintln(w, "  --no-push             Generate locally only (skip push flow)")
	_, _ = fmt.Fprintln(w, "  --yes                 Auto-confirm prompts where possible")
	_, _ = fmt.Fprintln(w, "  -h, --help            Show this help")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Examples:")
	examples := []string{
		"commitforge panel",
		"commitforge panel --dir .\\output",
		"commitforge panel --years 2",
		"commitforge panel --message \"update\" --message-mode fixed",
		"commitforge panel --message-mode random",
		"commitforge panel --remote git@github.com:user/repo.git",
		"commitforge panel --no-push",
		"commitforge panel --yes",
		"go run main.go",
	}
	for _, ex := range examples {
		_, _ = fmt.Fprintln(w, "  "+ex)
	}
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Tip:")
	_, _ = fmt.Fprintln(w, "  Use '?' inside any TUI screen to open active keybindings help.")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, strings.Repeat("-", 72))
}
