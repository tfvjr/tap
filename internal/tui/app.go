package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tfvjr/tap/internal/model"
	"github.com/tfvjr/tap/internal/store"
)

const refreshInterval = 2 * time.Second

// view is the current screen of the TUI.
type view int

const (
	viewDashboard view = iota
	viewDetail
	viewHelp
)

// listItem represents one navigable row — either a project header or a process.
type listItem struct {
	isProject bool
	project   *model.Project    // set if isProject
	process   *model.DevProcess // set if !isProject
	projName  string            // project name (for processes too)
}

// Model is the top-level bubbletea model for the tap TUI.
type Model struct {
	snapshot    *model.Snapshot
	items       []listItem // flattened navigable list
	cursor      int
	view        view
	width       int
	height      int
	store       *store.Store
	err         error
	confirmKill bool   // true when awaiting kill confirmation
	confirmMsg  string // message shown during confirmation
	confirmPID  *int32
	confirmKind model.ProcessKind
	confirmCID  string // container ID for docker kills
	statusMsg   string // transient status message
}

// snapshotMsg carries a fresh snapshot from the background ticker.
type snapshotMsg struct {
	snap *model.Snapshot
	err  error
}

// tickMsg triggers a refresh.
type tickMsg time.Time

// statusClearMsg clears the status message.
type statusClearMsg struct{}

// NewModel creates the initial TUI model.
func NewModel(s *store.Store) Model {
	return Model{
		store: s,
		view:  viewDashboard,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		queryDBCmd(m.store),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case snapshotMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.snapshot = msg.snap
			m.rebuildItems()
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			queryDBCmd(m.store),
			tickCmd(),
		)

	case statusClearMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Confirmation dialog takes priority.
	if m.confirmKill {
		switch {
		case key.Matches(msg, keys.Yes):
			m.confirmKill = false
			return m, m.executeKill()
		case key.Matches(msg, keys.No), key.Matches(msg, keys.Back):
			m.confirmKill = false
			m.confirmMsg = ""
			return m, nil
		}
		return m, nil
	}

	// Help view.
	if m.view == viewHelp {
		if key.Matches(msg, keys.Back) || key.Matches(msg, keys.Help) || key.Matches(msg, keys.Quit) {
			m.view = viewDashboard
		}
		return m, nil
	}

	// Detail view.
	if m.view == viewDetail {
		switch {
		case key.Matches(msg, keys.Back):
			m.view = viewDashboard
			return m, nil
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case msg.String() == "k":
			return m, m.startKillSelected()
		case key.Matches(msg, keys.Stop):
			return m, m.startStopProject()
		}
		return m, nil
	}

	// Dashboard view.
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, keys.Enter):
		if m.cursor >= 0 && m.cursor < len(m.items) && !m.items[m.cursor].isProject {
			m.view = viewDetail
		}
	case key.Matches(msg, keys.Help):
		m.view = viewHelp
	case key.Matches(msg, keys.Refresh):
		return m, queryDBCmd(m.store)
	case msg.String() == "k":
		return m, m.startKillSelected()
	case key.Matches(msg, keys.Stop):
		return m, m.startStopProject()
	}

	return m, nil
}

func (m *Model) startKillSelected() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if item.isProject || item.process == nil {
		return nil
	}

	proc := item.process
	switch proc.Kind {
	case model.ProcessDocker, model.ProcessPodman:
		if proc.ContainerInfo == nil {
			return nil
		}
		m.confirmKill = true
		m.confirmMsg = fmt.Sprintf("Kill container %s (%s)?", proc.ContainerInfo.ContainerName, proc.Name)
		m.confirmPID = nil
		m.confirmKind = proc.Kind
		m.confirmCID = proc.ContainerInfo.ContainerID
	default:
		if proc.PID == nil {
			return nil
		}
		m.confirmKill = true
		m.confirmMsg = fmt.Sprintf("Kill %s (PID %d)?", proc.Name, *proc.PID)
		m.confirmPID = proc.PID
		m.confirmKind = proc.Kind
	}
	return nil
}

func (m *Model) startStopProject() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	projName := item.projName
	if projName == "" || projName == "unattributed" {
		return nil
	}

	m.confirmKill = true
	m.confirmMsg = fmt.Sprintf("Stop all processes for project %q?", projName)
	// Mark as a project-level stop by clearing PID and container ID.
	m.confirmPID = nil
	m.confirmCID = ""
	m.confirmKind = "stop-project"
	return nil
}

func (m *Model) executeKill() tea.Cmd {
	if m.confirmKind == "stop-project" {
		return m.executeStopProject()
	}

	return func() tea.Msg {
		switch m.confirmKind {
		case model.ProcessDocker, model.ProcessPodman:
			rt := "docker"
			if m.confirmKind == model.ProcessPodman {
				rt = "podman"
			}
			out, err := exec.Command(rt, "stop", m.confirmCID).CombinedOutput()
			if err != nil {
				return snapshotMsg{err: fmt.Errorf("stopping container: %s: %w", strings.TrimSpace(string(out)), err)}
			}
		default:
			if m.confirmPID == nil {
				return snapshotMsg{err: fmt.Errorf("no PID to kill")}
			}
			p, err := os.FindProcess(int(*m.confirmPID))
			if err != nil {
				return snapshotMsg{err: err}
			}
			if err := p.Kill(); err != nil {
				return snapshotMsg{err: err}
			}
		}
		// Wait briefly for the collector to pick up the change, then re-query.
		time.Sleep(500 * time.Millisecond)
		snap, err := m.store.LatestSnapshot(true)
		return snapshotMsg{snap: snap, err: err}
	}
}

func (m *Model) executeStopProject() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	projName := m.items[m.cursor].projName

	return func() tea.Msg {
		if m.snapshot == nil {
			return snapshotMsg{err: fmt.Errorf("no snapshot")}
		}
		for _, proj := range m.snapshot.Projects {
			if !strings.EqualFold(proj.Name, projName) {
				continue
			}
			for _, proc := range proj.Processes {
				switch proc.Kind {
				case model.ProcessDocker, model.ProcessPodman:
					if proc.ContainerInfo != nil {
						rt := "docker"
						if proc.Kind == model.ProcessPodman {
							rt = "podman"
						}
						exec.Command(rt, "stop", proc.ContainerInfo.ContainerID).Run()
					}
				default:
					if proc.PID != nil {
						if p, err := os.FindProcess(int(*proc.PID)); err == nil {
							p.Kill()
						}
					}
				}
			}
		}
		// Wait briefly for the collector to pick up the change, then re-query.
		time.Sleep(500 * time.Millisecond)
		snap, err := m.store.LatestSnapshot(true)
		return snapshotMsg{snap: snap, err: err}
	}
}

func (m *Model) moveCursor(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

func (m *Model) rebuildItems() {
	if m.snapshot == nil {
		return
	}

	items := make([]listItem, 0, 32)

	for i := range m.snapshot.Projects {
		proj := &m.snapshot.Projects[i]
		items = append(items, listItem{
			isProject: true,
			project:   proj,
			projName:  proj.Name,
		})
		for j := range proj.Processes {
			items = append(items, listItem{
				process:  &proj.Processes[j],
				projName: proj.Name,
			})
		}
	}

	if len(m.snapshot.Unattributed) > 0 {
		items = append(items, listItem{
			isProject: true,
			projName:  "unattributed",
		})
		for i := range m.snapshot.Unattributed {
			items = append(items, listItem{
				process:  &m.snapshot.Unattributed[i],
				projName: "unattributed",
			})
		}
	}

	m.items = items

	// Keep cursor in bounds.
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) View() string {
	if m.err != nil && m.snapshot == nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}
	if m.snapshot == nil {
		return "Loading..."
	}

	switch m.view {
	case viewDetail:
		return m.renderDetail()
	case viewHelp:
		return m.renderHelp()
	default:
		return m.renderDashboard()
	}
}

func queryDBCmd(s *store.Store) tea.Cmd {
	return func() tea.Msg {
		snap, err := s.LatestSnapshot(true)
		return snapshotMsg{snap: snap, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
