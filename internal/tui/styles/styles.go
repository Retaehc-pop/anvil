package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Palette
	Green  = lipgloss.Color("#22c55e")
	Red    = lipgloss.Color("#ef4444")
	Yellow = lipgloss.Color("#eab308")
	Blue   = lipgloss.Color("#3b82f6")
	Gray   = lipgloss.Color("#6b7280")
	White  = lipgloss.Color("#f9fafb")
	Dim    = lipgloss.Color("#9ca3af")

	// Pane borders
	ActiveBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Blue)

	InactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Gray)

	// Status icons (task list)
	IconPending  = lipgloss.NewStyle().Foreground(Gray).Render("·")
	IconOK       = lipgloss.NewStyle().Foreground(Green).Render("✓")
	IconPartial  = lipgloss.NewStyle().Foreground(Yellow).Render("~")
	IconFailed   = lipgloss.NewStyle().Foreground(Red).Render("✗")
	IconSkipped  = lipgloss.NewStyle().Foreground(Blue).Render("⊘")
	IconRunning  = lipgloss.NewStyle().Foreground(Yellow).Render("▶")

	// Status bar
	StatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#1f2937")).
			Foreground(White).
			Padding(0, 1)

	OkCount      = lipgloss.NewStyle().Foreground(Green).Bold(true)
	ChangedCount = lipgloss.NewStyle().Foreground(Yellow).Bold(true)
	FailCount    = lipgloss.NewStyle().Foreground(Red).Bold(true)
	SkipCount    = lipgloss.NewStyle().Foreground(Blue)

	// Title bar
	TitleBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(lipgloss.Color("#111827")).
			Padding(0, 1)

	// Help key hints
	KeyHint = lipgloss.NewStyle().Foreground(Dim)

	// Selected list item
	Selected = lipgloss.NewStyle().Foreground(Blue).Bold(true)
)
