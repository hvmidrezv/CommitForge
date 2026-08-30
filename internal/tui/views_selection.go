// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"fmt"
	"strings"
)

// renderCountEntryScreen renders the commit-count assignment body.
func renderCountEntryScreen(m Model) string {
	var sb strings.Builder
	bodyW := m.layoutBodyTextWidth()
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("%d day(s) selected", m.selection.Count())))
	sb.WriteString("\n\n")
	sb.WriteString(InputLabelStyle.MaxWidth(bodyW).Render("Count or range (e.g. 5 or 1-8)"))
	sb.WriteString("\n")

	inputStyle := InputBoxStyle
	if m.countErr != "" {
		inputStyle = InputErrorBoxStyle
	}
	inputW := max(12, bodyW-2)
	sb.WriteString(inputStyle.Width(inputW).MaxWidth(inputW).Render(clampInputTail(m.countInput, inputW-1) + "█"))
	sb.WriteString("\n")

	if m.countErr != "" {
		sb.WriteString(InlineErrorStyle.MaxWidth(bodyW).Render("✗ " + m.countErr))
	} else if m.previewCounts != nil {
		sb.WriteString(MutedStyle.MaxWidth(bodyW).Render("Preview updates the grid intensity live."))
	} else {
		sb.WriteString(MutedStyle.MaxWidth(bodyW).Render("Enter a fixed count or a min-max range."))
	}
	sb.WriteString("\n\n")

	vs := viewState{
		cursor:        m.cursor,
		sel:           m.selection,
		previewCounts: m.previewCounts,
		compact:       m.useCompactGrid(),
	}
	sb.WriteString(RenderGrid(m.calendar, vs))
	return strings.TrimRight(sb.String(), "\n")
}

func clampInputTail(in string, maxRunes int) string {
	if maxRunes < 1 {
		return ""
	}
	rs := []rune(in)
	if len(rs) <= maxRunes {
		return in
	}
	return string(rs[len(rs)-maxRunes:])
}
