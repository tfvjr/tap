package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tfvjr/tap/internal/model"
)

func (m Model) renderDashboard() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(m.renderDivider())
	b.WriteString("\n")

	// Process list
	for i, item := range m.items {
		selected := i == m.cursor
		if item.isProject {
			b.WriteString(m.renderProjectRow(item, selected))
		} else {
			b.WriteString(m.renderProcessRow(item, selected))
		}
		b.WriteString("\n")
	}

	if len(m.items) == 0 {
		b.WriteString("\n  No dev processes found.\n")
	}

	// Confirmation dialog or status bar
	b.WriteString("\n")
	b.WriteString(m.renderDivider())
	b.WriteString("\n")
	if m.confirmKill {
		b.WriteString(m.renderConfirm())
	} else {
		b.WriteString(m.renderStatusBar())
	}

	return b.String()
}

func (m Model) renderHeader() string {
	title := headerStyle.Render(" tap")

	memUsedGB := float64(m.snapshot.SystemMemoryUsed) / (1024 * 1024 * 1024)
	memTotalGB := float64(m.snapshot.SystemMemoryTotal) / (1024 * 1024 * 1024)

	stats := headerDimStyle.Render(fmt.Sprintf(
		"CPU: %.0f%%  MEM: %.1f/%.1f GB",
		m.snapshot.SystemCPU,
		memUsedGB,
		memTotalGB,
	))

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(stats)
	if gap < 1 {
		gap = 1
	}

	return title + strings.Repeat(" ", gap) + stats
}

func (m Model) renderDivider() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	return dividerStyle.Render(strings.Repeat("─", w))
}

func (m Model) renderProjectRow(item listItem, selected bool) string {
	name := item.projName
	if item.project != nil {
		name = item.project.Name
	}

	arrow := " ▼ "
	line := arrow + projectStyle.Render(name)

	if item.project != nil {
		n := len(item.project.Processes)
		unit := "services"
		if n == 1 {
			unit = "service"
		}

		memMB := float64(item.project.TotalMemory) / (1024 * 1024)
		stats := projectStatsStyle.Render(fmt.Sprintf(
			"  %d %s    CPU %.1f%%    %.0f MB",
			n, unit,
			item.project.TotalCPU,
			memMB,
		))
		line += stats
	}

	if selected {
		line = selectedStyle.Render(padRight(line, m.width))
	}

	return line
}

func (m Model) renderProcessRow(item listItem, selected bool) string {
	proc := item.process
	if proc == nil {
		return ""
	}

	// Health indicator
	indicator := healthyStyle.Render("●")
	if proc.Health != nil && !proc.Health.IsHealthy() {
		indicator = warningStyle.Render("⚠")
	}

	// Port
	portStr := "      "
	if len(proc.Ports) > 0 {
		portStr = portStyle.Render(fmt.Sprintf(":%-5d", proc.Ports[0].Port))
	}

	// Name (with container info if Docker)
	name := proc.Name
	if proc.ContainerInfo != nil && proc.ContainerInfo.ContainerName != "" {
		name = fmt.Sprintf("%s (docker: %s)", proc.Name, proc.ContainerInfo.ContainerName)
	}
	nameStr := processNameStyle.Render(name)

	// Health warnings inline
	warnings := ""
	if proc.Health != nil {
		if proc.Health.HasFlag(model.HealthStale) {
			warnings += warningStyle.Render(" stale")
		}
		if proc.Health.HasFlag(model.HealthOrphan) {
			warnings += warningStyle.Render(" orphan")
		}
		if proc.Health.HasFlag(model.HealthHighMem) {
			warnings += warningStyle.Render(" hi-mem")
		}
		if proc.Health.HasFlag(model.HealthHighCPU) {
			warnings += warningStyle.Render(" hi-cpu")
		}
	}

	// Stats
	memMB := proc.MemoryMB()
	uptime := formatUptime(proc.Uptime())
	stats := processStatsStyle.Render(fmt.Sprintf(
		"CPU %5.1f%%  %7.0f MB  %s",
		proc.CPUPercent,
		memMB,
		uptime,
	))

	line := fmt.Sprintf("   %s %s  %s%s    %s", indicator, portStr, nameStr, warnings, stats)

	if selected {
		line = selectedStyle.Render(padRight(line, m.width))
	}

	return line
}

func (m Model) renderConfirm() string {
	return confirmStyle.Render(fmt.Sprintf(" %s  [y]es  [n]o", m.confirmMsg))
}

func (m Model) renderStatusBar() string {
	if m.statusMsg != "" {
		return statusBarStyle.Render(" " + m.statusMsg)
	}
	return statusBarStyle.Render(" ↑↓ navigate  enter expand  k kill  s stop project  r refresh  ? help  q quit")
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
