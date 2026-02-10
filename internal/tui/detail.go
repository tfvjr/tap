package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tap-dev/tap/internal/model"
)

func (m Model) renderDetail() string {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return "No process selected."
	}
	item := m.items[m.cursor]
	if item.process == nil {
		return "No process selected."
	}

	proc := item.process
	var b strings.Builder

	// Header line
	healthBadge := healthyStyle.Render("● HEALTHY")
	if proc.Health != nil && !proc.Health.IsHealthy() {
		badges := []string{}
		for _, f := range proc.Health.Flags {
			switch f {
			case model.HealthStale:
				badges = append(badges, "STALE")
			case model.HealthOrphan:
				badges = append(badges, "ORPHAN")
			case model.HealthHighMem:
				badges = append(badges, "HIGH MEM")
			case model.HealthHighCPU:
				badges = append(badges, "HIGH CPU")
			}
		}
		healthBadge = warningStyle.Render("⚠ " + strings.Join(badges, " | "))
	}

	portStr := ""
	if len(proc.Ports) > 0 {
		portStr = fmt.Sprintf("Port %d — ", proc.Ports[0].Port)
	}
	title := headerStyle.Render(fmt.Sprintf(" %s%s", portStr, proc.Name))

	gap := m.width - len(portStr) - len(proc.Name) - 3 - len(healthBadge) + 10
	if gap < 2 {
		gap = 2
	}
	b.WriteString(title + strings.Repeat(" ", gap) + healthBadge)
	b.WriteString("\n")
	b.WriteString(m.renderDivider())
	b.WriteString("\n\n")

	// Process section
	b.WriteString(sectionHeaderStyle.Render(" Process"))
	b.WriteString("\n")

	pid := "-"
	if proc.PID != nil {
		pid = fmt.Sprintf("%d", *proc.PID)
	}
	b.WriteString(detailLine("PID", pid))
	b.WriteString(detailLine("Name", proc.Name))
	if proc.Command != "" {
		b.WriteString(detailLine("Command", proc.Command))
	}
	b.WriteString(detailLine("Uptime", formatDetailUptime(proc.Uptime())))
	b.WriteString(detailLine("CPU", fmt.Sprintf("%.1f%%", proc.CPUPercent)))
	b.WriteString(detailLine("Memory", fmt.Sprintf("%.1f MB (RSS)", proc.MemoryMB())))
	b.WriteString(detailLine("Kind", string(proc.Kind)))
	b.WriteString("\n")

	// Container section (if Docker/Podman)
	if proc.ContainerInfo != nil {
		b.WriteString(sectionHeaderStyle.Render(" Container"))
		b.WriteString("\n")
		b.WriteString(detailLine("ID", truncateID(proc.ContainerInfo.ContainerID)))
		b.WriteString(detailLine("Name", proc.ContainerInfo.ContainerName))
		b.WriteString(detailLine("Image", proc.ContainerInfo.Image))
		b.WriteString(detailLine("Status", proc.ContainerInfo.Status))
		if len(proc.ContainerInfo.PortMappings) > 0 {
			ports := make([]string, len(proc.ContainerInfo.PortMappings))
			for i, pm := range proc.ContainerInfo.PortMappings {
				ports[i] = fmt.Sprintf("%d→%d", pm.HostPort, pm.ContainerPort)
			}
			b.WriteString(detailLine("Ports", strings.Join(ports, ", ")))
		}
		b.WriteString("\n")
	}

	// Parent chain section (only for native processes)
	if proc.Kind == model.ProcessNative && proc.Health != nil && len(proc.Health.ParentChain) > 0 {
		b.WriteString(sectionHeaderStyle.Render(" Parent Chain"))
		b.WriteString("\n")
		for i, parent := range proc.Health.ParentChain {
			indent := strings.Repeat("   ", i+1)
			status := healthyStyle.Render("✓ alive")
			if !parent.Alive {
				status = dangerStyle.Render("✗ DEAD")
			}
			b.WriteString(fmt.Sprintf("   %s└─ %s (PID %d)  %s\n",
				indent, parent.Name, parent.PID, status))
		}
		b.WriteString("\n")
	}

	// Project section
	if proc.Project != "" {
		b.WriteString(sectionHeaderStyle.Render(" Project"))
		b.WriteString("\n")
		b.WriteString(detailLine("Name", proc.Project))
		if proc.ProjectPath != "" {
			b.WriteString(detailLine("Path", proc.ProjectPath))
		}

		// Show sibling processes in the same project
		siblings := m.findSiblings(proc)
		if len(siblings) > 0 {
			b.WriteString("   " + labelStyle.Render("Other services:") + "\n")
			for _, sib := range siblings {
				portStr := "      "
				if len(sib.Ports) > 0 {
					portStr = fmt.Sprintf(":%-5d", sib.Ports[0].Port)
				}
				b.WriteString(fmt.Sprintf("     %s %s  %s    %.0f MB  %s\n",
					healthyStyle.Render("●"),
					portStyle.Render(portStr),
					sib.Name,
					sib.MemoryMB(),
					formatUptime(sib.Uptime()),
				))
			}
		}
		b.WriteString("\n")
	}

	// Diagnostics section
	if proc.Health != nil && !proc.Health.IsHealthy() {
		b.WriteString(sectionHeaderStyle.Render(" Diagnostics"))
		b.WriteString("\n")
		for _, f := range proc.Health.Flags {
			switch f {
			case model.HealthStale:
				b.WriteString("   " + warningStyle.Render("⚠") + " Process has been running for " + formatDetailUptime(proc.Uptime()) + "\n")
			case model.HealthOrphan:
				parentName := "unknown"
				if len(proc.Health.ParentChain) > 0 {
					parentName = fmt.Sprintf("%s (PID %d)", proc.Health.ParentChain[0].Name, proc.Health.ParentChain[0].PID)
				}
				b.WriteString("   " + warningStyle.Render("⚠") + " Parent process (" + parentName + ") is dead — this may be orphaned\n")
			case model.HealthHighMem:
				b.WriteString("   " + warningStyle.Render("⚠") + fmt.Sprintf(" Memory usage is %.0f MB (> 1 GB threshold)\n", proc.MemoryMB()))
			case model.HealthHighCPU:
				b.WriteString("   " + warningStyle.Render("⚠") + fmt.Sprintf(" CPU usage is %.1f%% (> 50%% threshold)\n", proc.CPUPercent))
			}
		}
		// Port info
		for _, pb := range proc.Ports {
			b.WriteString("   " + healthyStyle.Render("●") + fmt.Sprintf(" Listening on %s:%d (%s)\n", pb.Address, pb.Port, pb.Protocol))
		}
		b.WriteString("\n")
	} else if len(proc.Ports) > 0 {
		// Show port info even for healthy processes
		b.WriteString(sectionHeaderStyle.Render(" Network"))
		b.WriteString("\n")
		for _, pb := range proc.Ports {
			b.WriteString("   " + healthyStyle.Render("●") + fmt.Sprintf(" Listening on %s:%d (%s)\n", pb.Address, pb.Port, pb.Protocol))
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(m.renderDivider())
	b.WriteString("\n")
	if m.confirmKill {
		b.WriteString(m.renderConfirm())
	} else {
		b.WriteString(statusBarStyle.Render(" k kill  s stop project  esc back  q quit"))
	}

	return b.String()
}

func (m Model) findSiblings(proc *model.DevProcess) []model.DevProcess {
	if m.snapshot == nil || proc.Project == "" {
		return nil
	}
	var siblings []model.DevProcess
	for _, proj := range m.snapshot.Projects {
		if !strings.EqualFold(proj.Name, proc.Project) {
			continue
		}
		for _, p := range proj.Processes {
			// Skip self.
			if proc.PID != nil && p.PID != nil && *proc.PID == *p.PID {
				continue
			}
			// Also skip if same container.
			if proc.ContainerInfo != nil && p.ContainerInfo != nil &&
				proc.ContainerInfo.ContainerID == p.ContainerInfo.ContainerID {
				continue
			}
			siblings = append(siblings, p)
		}
	}
	return siblings
}

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(" tap — keyboard shortcuts"))
	b.WriteString("\n")
	b.WriteString(m.renderDivider())
	b.WriteString("\n\n")

	helpItems := []struct{ key, desc string }{
		{"↑/↓ or j/k", "Navigate up/down"},
		{"Enter", "Expand process detail (diagnostic card)"},
		{"Esc", "Back to dashboard"},
		{"k", "Kill selected process (with confirmation)"},
		{"s", "Stop all processes for selected project"},
		{"r", "Force refresh"},
		{"?", "Toggle this help"},
		{"q / Ctrl+C", "Quit"},
	}

	for _, h := range helpItems {
		key := lipgloss.NewStyle().Foreground(cyan).Width(16).Render(h.key)
		b.WriteString("   " + key + "  " + h.desc + "\n")
	}

	b.WriteString("\n")
	b.WriteString(sectionHeaderStyle.Render(" Health indicators"))
	b.WriteString("\n")
	b.WriteString("   " + healthyStyle.Render("●") + "  Healthy — no issues detected\n")
	b.WriteString("   " + warningStyle.Render("⚠") + "  Warning — stale (>24h), orphan, high memory (>1GB), or high CPU (>50%)\n")

	b.WriteString("\n")
	b.WriteString(m.renderDivider())
	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render(" Press ? or Esc to return"))

	return b.String()
}

func detailLine(label, value string) string {
	return "   " + labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatDetailUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", totalSeconds%60)
	}
}
