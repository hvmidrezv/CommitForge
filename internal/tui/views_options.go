// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// menuItem identifies one entry in the post-count-assignment options menu.
type menuItem int

const (
	menuPush menuItem = iota
	menuAddRemote
	menuRemoveRemote
	menuDeselect
	menuClearAll
	menuEditCounts
	menuGenerateLocal
	menuPreviewSummary
	menuSaveExit
	menuBack
	menuCount
)

var menuLabels = [menuCount]string{
	menuPush:           "Push to remote",
	menuAddRemote:      "Add remote / connect",
	menuRemoveRemote:   "Remove remote / disconnect",
	menuDeselect:       "Deselect all",
	menuClearAll:       "Clear all commits (force-push rewrite) [x]",
	menuEditCounts:     "Edit counts",
	menuGenerateLocal:  "Generate locally (no push)",
	menuPreviewSummary: "Preview summary",
	menuSaveExit:       "Save & exit",
	menuBack:           "Back",
}

var menuDescriptions = [menuCount]string{
	menuPush:           "Push generated history to this project's configured remote.",
	menuAddRemote:      "Connect this local project to a remote repository URL.",
	menuRemoveRemote:   "Disconnect remote and keep this project local-only.",
	menuDeselect:       "Remove selected days and optionally regenerate history if already generated.",
	menuClearAll:       "Destructive: rewrite local history to empty baseline and force-push to origin/main.",
	menuEditCounts:     "Re-open count assignment for the current selection.",
	menuGenerateLocal:  "Generate commits locally without pushing to any remote.",
	menuPreviewSummary: "Show a date/count/weekday summary for selected days.",
	menuSaveExit:       "Persist state to disk and quit the application.",
	menuBack:           "Return to the grid while keeping current state.",
}

// renderOptionsScreen renders the post-count-assignment action menu body.
func renderOptionsScreen(m Model) string {
	var sb strings.Builder
	bodyW := m.layoutBodyTextWidth()
	bodyH := m.layoutBodyHeight()
	showDescriptions := bodyH >= 14
	n := m.selection.Count()
	total := totalSelectedCommits(m)
	if total > 0 {
		sb.WriteString(InfoStyle.MaxWidth(bodyW).Render(fmt.Sprintf("%d day(s) selected, %d commit(s) queued", n, total)))
	} else {
		sb.WriteString(InfoStyle.MaxWidth(bodyW).Render(fmt.Sprintf("%d day(s) selected", n)))
	}
	sb.WriteString("\n\n")

	for i := menuItem(0); i < menuCount; i++ {
		prefix := "  "
		lineStyle := MenuItemStyle
		if int(i) == m.menuCursor {
			prefix = "› "
			lineStyle = MenuSelectedStyle
		}
		sb.WriteString(lineStyle.MaxWidth(bodyW).Render(prefix + menuLabels[i]))
		if showDescriptions {
			sb.WriteByte('\n')
			sb.WriteString(MenuItemDescriptionStyle.MaxWidth(bodyW).Render("  " + menuDescriptions[i]))
			sb.WriteString("\n\n")
		} else {
			sb.WriteByte('\n')
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func totalSelectedCommits(m Model) int {
	total := 0
	for _, d := range m.selection.Dates() {
		total += m.dateCounts[dateKeyUTC(d)]
	}
	return total
}

// renderPreviewScreen shows a table of date / weekday / commit-count for every selected day.
func renderPreviewScreen(m Model) string {
	var sb strings.Builder
	bodyW := m.layoutBodyTextWidth()
	dates := m.selection.Dates()
	counts := calendarCountMap(m)
	grandTotal := 0
	for _, d := range dates {
		grandTotal += counts[d.UTC().Format("2006-01-02")]
	}
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("%d day(s), %d total commit(s)", len(dates), grandTotal)))
	sb.WriteString("\n\n")
	sb.WriteString(lipTableHeader("Date", "Weekday", "Commits"))
	sb.WriteByte('\n')
	if len(dates) == 0 {
		sb.WriteString(MutedStyle.Render("(no days selected)"))
		sb.WriteByte('\n')
	} else {
		visibleRows := max(1, m.layoutBodyHeight()-7)
		start := m.previewOffset
		maxStart := max(0, len(dates)-visibleRows)
		if start > maxStart {
			start = maxStart
		}
		if start < 0 {
			start = 0
		}
		end := min(len(dates), start+visibleRows)
		for _, d := range dates[start:end] {
			cnt := counts[d.UTC().Format("2006-01-02")]
			row := fmt.Sprintf("%-12s  %-3s  %d", d.Format("2006-01-02"), d.Format("Mon"), cnt)
			sb.WriteString(InfoStyle.MaxWidth(bodyW).Render(row))
			sb.WriteByte('\n')
		}
		if len(dates) > visibleRows {
			sb.WriteString("\n")
			sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(
				fmt.Sprintf("Showing %d-%d of %d rows (use up/down to scroll)", start+1, end, len(dates))))
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("Total: %d commit(s)", grandTotal)))
	return strings.TrimRight(sb.String(), "\n")
}

func renderDeselectConfirmScreen(m Model) string {
	msg := fmt.Sprintf(
		"This will remove %d already-generated commit(s) for the selected day(s). Continue?",
		m.deselectPendingGeneratedTotal)
	return InfoStyle.Render("Dangerous action") + "\n\n" +
		MutedStyle.MaxWidth(m.layoutBodyTextWidth()).Render(msg) + "\n\n" +
		MutedStyle.MaxWidth(m.layoutBodyTextWidth()).Render("Press y to confirm, or n/esc to cancel.")
}

func renderClearAllConfirmScreen(m Model) string {
	bodyW := m.layoutBodyTextWidth()
	msg := "This will rewrite history and force-push origin/main, removing all generated commits on remote and local history."
	confirm := fmt.Sprintf("Type %q or %q, then press Enter to confirm.", "yes", m.projectName)
	inputStyle := InputBoxStyle
	if strings.TrimSpace(m.clearAllConfirmInput) != "" && !m.isValidClearAllConfirmation() {
		inputStyle = InputErrorBoxStyle
	}
	inputW := max(12, bodyW-2)
	out := InfoStyle.Render("Destructive action") + "\n\n" +
		MutedStyle.MaxWidth(bodyW).Render(msg) + "\n\n" +
		MutedStyle.MaxWidth(bodyW).Render(confirm) + "\n" +
		inputStyle.Width(inputW).MaxWidth(inputW).Render(clampInputTail(m.clearAllConfirmInput, inputW-1)+"█")
	if strings.TrimSpace(m.clearAllConfirmInput) != "" && !m.isValidClearAllConfirmation() {
		out += "\n" + InlineErrorStyle.MaxWidth(bodyW).Render("✗ Confirmation text does not match.")
	}
	return out
}

// renderGeneratingScreen shows an animated spinner while commits are created.
func renderGeneratingScreen(m Model) string {
	spinner := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	dir := m.generateDir
	if dir == "" {
		dir = m.activeProjectDir()
	}
	var sb strings.Builder
	action := "Regenerating commit history"
	if m.clearAllInFlight {
		action = "Clearing all commits and force-pushing"
	}
	sb.WriteString(InfoStyle.MaxWidth(m.layoutBodyTextWidth()).Render(
		fmt.Sprintf("%s  %s in %s", spinner, action, dir)))
	sb.WriteString("\n\n")
	sb.WriteString(MutedStyle.MaxWidth(m.layoutBodyTextWidth()).Render("Please wait while commits are being created..."))
	return sb.String()
}

// renderGenerateDoneScreen shows the generation result (success or error).
func renderGenerateDoneScreen(m Model) string {
	if m.generateErr != nil {
		return ErrorStyle.MaxWidth(m.layoutBodyTextWidth()).Render("Generation failed") + "\n\n" +
			InlineErrorStyle.MaxWidth(m.layoutBodyTextWidth()).Render(m.generateErr.Error())
	}
	return InfoStyle.MaxWidth(m.layoutBodyTextWidth()).Render("Generation completed successfully.\n\n" + m.generateMsg)
}

func lipTableHeader(col1, col2, col3 string) string {
	head := fmt.Sprintf("%-12s  %-9s  %s", col1, col2, col3)
	return lipgloss.NewStyle().Foreground(MutedTextColor).Bold(true).Render(head)
}

// calendarCountMap builds a "2006-01-02" -> Count lookup from the model map.
func calendarCountMap(m Model) map[string]int {
	out := make(map[string]int, len(m.dateCounts))
	for k, v := range m.dateCounts {
		out[k] = v
	}
	return out
}
