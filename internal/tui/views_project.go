package tui

import (
	"fmt"
	"strings"
)

func renderProjectSelectScreen(m Model) string {
	bodyW := m.layoutBodyTextWidth()
	if len(m.projectNames) == 0 {
		return MutedStyle.MaxWidth(bodyW).Render("No projects found.")
	}
	var sb strings.Builder
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render("Select a project in ./output, or create a new one."))
	sb.WriteString("\n\n")
	for i, name := range m.projectNames {
		prefix := "  "
		style := MenuItemStyle
		if i == m.projectCursor {
			prefix = "› "
			style = MenuSelectedStyle
		}
		sb.WriteString(style.MaxWidth(bodyW).Render(prefix + name))
		sb.WriteByte('\n')
	}
	if m.projectInputErr != "" {
		sb.WriteString("\n")
		sb.WriteString(InlineErrorStyle.MaxWidth(bodyW).Render("✗ " + m.projectInputErr))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderProjectCreateNameScreen(m Model) string {
	bodyW := m.layoutBodyTextWidth()
	var sb strings.Builder
	sb.WriteString(InputLabelStyle.MaxWidth(bodyW).Render("Project name"))
	sb.WriteString("\n")
	inputStyle := InputBoxStyle
	if m.projectInputErr != "" {
		inputStyle = InputErrorBoxStyle
	}
	inputW := max(12, bodyW-2)
	sb.WriteString(inputStyle.Width(inputW).MaxWidth(inputW).Render(clampInputTail(m.projectNameInput, inputW-1) + "█"))
	sb.WriteString("\n")
	if m.projectInputErr != "" {
		sb.WriteString(InlineErrorStyle.MaxWidth(bodyW).Render("✗ " + m.projectInputErr))
		sb.WriteString("\n")
	}
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render("Use letters, numbers, spaces, dash or underscore."))
	return sb.String()
}

func renderProjectRemoteModeScreen(m Model) string {
	bodyW := m.layoutBodyTextWidth()
	rows := []struct {
		label string
		desc  string
	}{
		{label: "Local only", desc: "Create project without any remote connection."},
		{label: "Add remote now", desc: "Save a remote URL now and push history later."},
	}
	var sb strings.Builder
	sb.WriteString(MutedStyle.MaxWidth(bodyW).Render(fmt.Sprintf("Project: %s", m.projectName)))
	sb.WriteString("\n\n")
	for i, r := range rows {
		prefix := "  "
		style := MenuItemStyle
		if i == m.projectRemoteChoice {
			prefix = "› "
			style = MenuSelectedStyle
		}
		sb.WriteString(style.MaxWidth(bodyW).Render(prefix + r.label))
		sb.WriteByte('\n')
		sb.WriteString(MenuItemDescriptionStyle.MaxWidth(bodyW).Render("  " + r.desc))
		sb.WriteString("\n\n")
	}
	if m.projectInputErr != "" {
		sb.WriteString(InlineErrorStyle.MaxWidth(bodyW).Render("✗ " + m.projectInputErr))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderProjectRemoteInputScreen(m Model) string {
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
