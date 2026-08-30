// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme colors used across all TUI screens.
var (
	// AccentColor highlights primary focus/selection UI.
	AccentColor = lipgloss.Color("#8b5cf6")
	// FocusColor marks the active/focused element with high contrast.
	FocusColor = lipgloss.Color("#facc15")
	// TodayColor highlights the "today" marker.
	TodayColor = lipgloss.Color("#fb7185")
	// PrimaryTextColor is the main readable foreground color.
	PrimaryTextColor = lipgloss.Color("#f8fafc")
	// MutedTextColor is for secondary labels and hints.
	MutedTextColor = lipgloss.Color("#94a3b8")
	// BorderColor is the default border/separator color.
	BorderColor = lipgloss.Color("#334155")
	// DangerColor is used for destructive or error states.
	DangerColor = lipgloss.Color("#ef4444")
	// WarningColor is used for warning/confirmation states.
	WarningColor = lipgloss.Color("#f59e0b")
	// PanelBackgroundColor is the base panel background.
	PanelBackgroundColor = lipgloss.Color("#0f172a")
	// InputBackgroundColor is used for input surfaces.
	InputBackgroundColor = lipgloss.Color("#111827")
)

// IntensityLevelColors are the 5 GitHub-style contribution levels (0-4).
var IntensityLevelColors = [5]lipgloss.Color{
	"#161b22",
	"#0e4429",
	"#006d32",
	"#26a641",
	"#39d353",
}

// IntensityCellStyles are background styles for contribution intensity levels.
var IntensityCellStyles [5]lipgloss.Style

func init() {
	for i := range IntensityCellStyles {
		IntensityCellStyles[i] = lipgloss.NewStyle().Background(IntensityLevelColors[i])
	}
}

var (
	AppFrameStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(PrimaryTextColor).
			Background(PanelBackgroundColor)

	HeaderZoneStyle = lipgloss.NewStyle().
			Padding(0, 1)

	BodyZoneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	DangerBodyZoneStyle  = BodyZoneStyle.BorderForeground(DangerColor)
	WarningBodyZoneStyle = BodyZoneStyle.BorderForeground(WarningColor)

	FooterZoneStyle = lipgloss.NewStyle().
			Padding(0, 1)

	AppLogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryTextColor).
			Background(AccentColor).
			Padding(0, 1)

	ScreenTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryTextColor)

	HeaderMetaStyle = lipgloss.NewStyle().
			Foreground(MutedTextColor)

	MutedStyle = lipgloss.NewStyle().Foreground(MutedTextColor)
	ErrorStyle = lipgloss.NewStyle().Foreground(DangerColor).Bold(true)
	InfoStyle  = lipgloss.NewStyle().Foreground(PrimaryTextColor)

	MonthLabelStyle  = lipgloss.NewStyle().Foreground(MutedTextColor)
	WeekdayStyle     = lipgloss.NewStyle().Foreground(MutedTextColor)
	LegendLabelStyle = lipgloss.NewStyle().Foreground(MutedTextColor)

	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(PrimaryTextColor)
	MenuItemDescriptionStyle = lipgloss.NewStyle().
					Foreground(MutedTextColor).
					PaddingLeft(3)
	MenuSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(PrimaryTextColor).
				Background(AccentColor).
				Padding(0, 1)

	InputLabelStyle = lipgloss.NewStyle().Foreground(MutedTextColor).Bold(true)
	InputBoxStyle   = lipgloss.NewStyle().
			Foreground(PrimaryTextColor).
			Background(InputBackgroundColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)
	InputErrorBoxStyle = InputBoxStyle.BorderForeground(DangerColor)
	InlineErrorStyle   = lipgloss.NewStyle().Foreground(DangerColor)

	HelpPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	KeyPillStyle = lipgloss.NewStyle().
			Foreground(PrimaryTextColor).
			Background(lipgloss.Color("#1e293b")).
			Padding(0, 1)
	GroupLabelStyle = lipgloss.NewStyle().
			Foreground(MutedTextColor)

	GridCursorStyle = lipgloss.NewStyle().
			Foreground(FocusColor).
			Bold(true)
	GridSelectedStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)
	GridRangeAnchorStyle = lipgloss.NewStyle().
				Foreground(WarningColor).
				Bold(true)
	GridRangePreviewStyle = lipgloss.NewStyle().
				Foreground(PrimaryTextColor)
	GridTodayStyle = lipgloss.NewStyle().
			Foreground(TodayColor).
			Bold(true)

	CodeBlockStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 1)
	LogPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)
)
