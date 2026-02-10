package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	green  = lipgloss.Color("#22c55e")
	yellow = lipgloss.Color("#eab308")
	red    = lipgloss.Color("#ef4444")
	cyan   = lipgloss.Color("#06b6d4")
	dim    = lipgloss.Color("#6b7280")
	white  = lipgloss.Color("#f9fafb")
	muted  = lipgloss.Color("#9ca3af")

	// Header bar
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white)

	headerDimStyle = lipgloss.NewStyle().
			Foreground(muted)

	// Divider line
	dividerStyle = lipgloss.NewStyle().
			Foreground(dim)

	// Project group header
	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white)

	projectStatsStyle = lipgloss.NewStyle().
				Foreground(muted)

	// Process row
	portStyle = lipgloss.NewStyle().
			Foreground(cyan)

	processNameStyle = lipgloss.NewStyle().
				Foreground(white)

	processStatsStyle = lipgloss.NewStyle().
				Foreground(muted)

	// Selected row
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#1e3a5f"))

	// Health indicators
	healthyStyle = lipgloss.NewStyle().
			Foreground(green)

	warningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	dangerStyle = lipgloss.NewStyle().
			Foreground(red)

	// Detail view sections
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(cyan)

	labelStyle = lipgloss.NewStyle().
			Foreground(muted).
			Width(14)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dim)

	// Confirm dialog
	confirmStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(yellow)
)
