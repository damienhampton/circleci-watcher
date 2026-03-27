package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("82")
	colorRed    = lipgloss.Color("196")
	colorYellow = lipgloss.Color("220")
	colorGrey   = lipgloss.Color("240")
	colorWhite  = lipgloss.Color("255")
	colorBlue   = lipgloss.Color("39")
	colorOrange = lipgloss.Color("208")
	colorBg     = lipgloss.Color("235")
	colorBgBar  = lipgloss.Color("236")

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("63")).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 2)

	styleStatusBar = lipgloss.NewStyle().
			Background(colorBgBar).
			Foreground(colorGrey).
			Padding(0, 2)

	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen)
	styleFailed  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleRunning = lipgloss.NewStyle().Foreground(colorYellow)
	styleGrey    = lipgloss.NewStyle().Foreground(colorGrey)
	styleWhite   = lipgloss.NewStyle().Foreground(colorWhite)
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleBlue    = lipgloss.NewStyle().Foreground(colorBlue)
	styleOrange  = lipgloss.NewStyle().Foreground(colorOrange)

	styleFailureBlock = lipgloss.NewStyle().
				Foreground(colorRed).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(colorRed).
				PaddingLeft(1)

	stylePipelineRow = lipgloss.NewStyle().
				Background(lipgloss.Color("234")).
				Padding(0, 1)
)

// Status icons
const (
	iconSuccess = "✔"
	iconFailed  = "✖"
	iconRunning = "●"
	iconPending = "○"
	iconOnHold  = "⏸"
	iconArrow   = "▶"
)

func statusIcon(status string) string {
	switch status {
	case "success":
		return styleSuccess.Render(iconSuccess)
	case "failed", "error", "infrastructure_fail", "timedout":
		return styleFailed.Render(iconFailed)
	case "running", "failing":
		return styleRunning.Render(iconRunning)
	case "on_hold", "blocked":
		return styleGrey.Render(iconOnHold)
	default:
		return styleGrey.Render(iconPending)
	}
}

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "success":
		return styleSuccess
	case "failed", "error", "infrastructure_fail", "timedout":
		return styleFailed
	case "running", "failing":
		return styleRunning
	case "on_hold", "blocked":
		return styleGrey
	default:
		return styleGrey
	}
}
