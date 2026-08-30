// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"fmt"
	"strings"

	"commitforge/internal/gitops"
)

func renderPushConfirmScreen(m Model) string {
	w := m.layoutBodyTextWidth()
	return InfoStyle.MaxWidth(w).Render("Push to remote repository?") + "\n\n" +
		MutedStyle.MaxWidth(w).Render("This may publish generated history to your remote branch.")
}

func renderPushGuidanceScreen(m Model) string {
	remote := strings.TrimSpace(m.pushRemote)
	if remote == "" {
		remote = strings.TrimSpace(m.cfg.Remote)
	}
	block := gitops.SetupGuidance(remote)
	w := m.layoutBodyTextWidth()
	return CodeBlockStyle.MaxWidth(w).Render(block)
}

func renderPushRepoTypeScreen(m Model) string {
	type row struct {
		label string
		desc  string
	}
	rows := []row{
		{
			label: "Blank repository",
			desc:  "Init locally (if needed), set origin, rename branch to main, then push -u origin main.",
		},
		{
			label: "Existing repository",
			desc:  "Push current local history while handling upstream tracking automatically.",
		},
	}
	var sb strings.Builder
	for i, r := range rows {
		selected := (i == 0 && m.pushRepoType == gitops.PushModeBlankRepo) ||
			(i == 1 && m.pushRepoType == gitops.PushModeExistingRepo)
		prefix := "  "
		lineStyle := MenuItemStyle
		if selected {
			prefix = "› "
			lineStyle = MenuSelectedStyle
		}
		sb.WriteString(lineStyle.MaxWidth(m.layoutBodyTextWidth()).Render(prefix + r.label))
		sb.WriteByte('\n')
		sb.WriteString(MenuItemDescriptionStyle.MaxWidth(m.layoutBodyTextWidth()).Render("  " + r.desc))
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderPushRemoteInputScreen(m Model) string {
	bodyW := m.layoutBodyTextWidth()
	var sb strings.Builder
	sb.WriteString(InputLabelStyle.MaxWidth(bodyW).Render("Remote URL (SSH or HTTPS)"))
	sb.WriteString("\n")
	inputStyle := InputBoxStyle
	if m.pushInputErr != "" {
		inputStyle = InputErrorBoxStyle
	}
	inputW := max(12, bodyW-2)
	sb.WriteString(inputStyle.Width(inputW).MaxWidth(inputW).Render(clampInputTail(m.pushRemoteInput, inputW-1) + "█"))
	sb.WriteString("\n")
	if m.pushInputErr != "" {
		sb.WriteString(InlineErrorStyle.MaxWidth(bodyW).Render("✗ " + m.pushInputErr))
		sb.WriteString("\n")
	}
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render("Example: git@github.com:<your-username>/<your-repo>.git"))
	return sb.String()
}

func renderPushRunningScreen(m Model) string {
	paneHeight := max(4, m.layoutBodyHeight()-6)
	start := m.pushLogOffset
	if start < 0 {
		start = 0
	}
	maxStart := max(0, len(m.pushLogs)-paneHeight)
	if start > maxStart {
		start = maxStart
	}
	end := min(len(m.pushLogs), start+paneHeight)
	lines := m.pushLogs[start:end]
	if len(lines) == 0 {
		lines = []string{"(waiting for git output...)"}
	}

	var sb strings.Builder
	bodyW := m.layoutBodyTextWidth()
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("Mode: %s", pushModeLabel(m.pushRepoType))))
	sb.WriteByte('\n')
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("Remote: %s", strings.TrimSpace(m.pushRemote))))
	sb.WriteString("\n\n")
	logW := max(16, bodyW-2)
	sb.WriteString(LogPaneStyle.MaxWidth(logW).Render(strings.Join(lines, "\n")))
	if len(m.pushLogs) > paneHeight {
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("Showing %d-%d of %d log lines", start+1, end, len(m.pushLogs))))
	}
	return sb.String()
}

func renderPushDoneScreen(m Model) string {
	if m.pushErr != nil {
		return ErrorStyle.Render("Push failed") + "\n\n" + m.pushDoneText
	}
	return InfoStyle.Render("Push completed.\n\n" + m.pushDoneText)
}

func pushModeLabel(mode gitops.PushMode) string {
	if mode == gitops.PushModeBlankRepo {
		return "blank repository"
	}
	return "existing repository"
}
