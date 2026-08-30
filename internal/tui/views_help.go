// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"fmt"
	"strings"
)

func renderHelpOverlay(m Model) string {
	w := m.layoutBodyTextWidth()
	var b strings.Builder
	b.WriteString(ScreenTitleStyle.MaxWidth(w).Render("Help"))
	b.WriteString("  ")
	b.WriteString(MutedStyle.MaxWidth(w).Render(screenName(m.screen)))
	b.WriteString("\n\n")

	if m.screen == screenGrid {
		b.WriteString(ScreenTitleStyle.MaxWidth(w).Render("Toolbar guide"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.MaxWidth(w).Render("Navigation: arrows/hjkl move day-to-day. [ and ] (or PgUp/PgDn) shift the visible year window."))
		b.WriteString("\n")
		b.WriteString(MutedStyle.MaxWidth(w).Render("Select: space toggles one day, v starts/confirms a contiguous date range, a selects all visible days, u clears selection."))
		b.WriteString("\n")
		b.WriteString(MutedStyle.MaxWidth(w).Render("Actions: enter continues with selected dates (count assignment or action menu). q quits, ctrl+c force-quits."))
		b.WriteString("\n\n")
	}

	b.WriteString(ScreenTitleStyle.MaxWidth(w).Render("Active keys"))
	b.WriteString("\n")
	for _, kb := range activeBindings(m) {
		line := fmt.Sprintf("%s - %s", kb.label(), kb.help)
		b.WriteString(MutedStyle.MaxWidth(w).Render(line))
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(MutedStyle.MaxWidth(w).Render("Press ? to close help."))
	return HelpPanelStyle.MaxWidth(w).Render(strings.TrimRight(b.String(), "\n"))
}

func screenName(s screen) string {
	switch s {
	case screenProjectSelect:
		return "Project Select"
	case screenProjectCreateName:
		return "Project Create Name"
	case screenProjectRemoteMode:
		return "Project Remote Mode"
	case screenProjectRemoteInput:
		return "Project Remote Input"
	case screenGrid:
		return "Grid"
	case screenCountEntry:
		return "Count Assignment"
	case screenOptions:
		return "Options Menu"
	case screenDeselectConfirm:
		return "Deselect Confirm"
	case screenClearAllConfirm:
		return "Clear All Confirm"
	case screenPreview:
		return "Preview Summary"
	case screenGenerating:
		return "Generating Commits"
	case screenGenerateDone:
		return "Generation Result"
	case screenPushConfirm:
		return "Push Confirm"
	case screenPushGuidance:
		return "Push Guidance"
	case screenPushRepoType:
		return "Push Repository Type"
	case screenPushRemoteInput:
		return "Push Remote Input"
	case screenPushRunning:
		return "Push Running"
	case screenPushDone:
		return "Push Result"
	default:
		return "Screen"
	}
}
