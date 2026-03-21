package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	workspacepkg "github.com/Gradient-Linux/concave-tui/internal/workspace"
)

const workspaceRefreshInterval = 5 * time.Second

type workspaceLoadedMsg struct {
	token      int
	used       uint64
	total      uint64
	usages     []workspacepkg.Usage
	root       string
	lastBackup time.Time
	loadErr    error
}

type workspaceTickMsg struct {
	token int
}

type workspaceOpMsg struct {
	kind    string
	detail  string
	success bool
	err     error
}

type workspaceNoteExpiredMsg struct{}

type WorkspaceModel struct {
	width          int
	height         int
	active         bool
	confirmClean   bool
	loading        bool
	loadToken      int
	root           string
	used           uint64
	total          uint64
	usages         []workspacepkg.Usage
	lastBackup     time.Time
	lastErr        error
	busyMessage    string
	completionNote string
}

func NewWorkspaceModel() WorkspaceModel { return WorkspaceModel{loading: true} }

func (m *WorkspaceModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.loadToken++
	token := m.loadToken
	return tea.Batch(loadWorkspaceCmd(token), workspaceTickCmd(token))
}

func (m *WorkspaceModel) Deactivate() {
	m.active = false
	m.loadToken++
}

func (m *WorkspaceModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m WorkspaceModel) Update(msg tea.Msg) (WorkspaceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			m.loadToken++
			token := m.loadToken
			return m, tea.Batch(loadWorkspaceCmd(token), workspaceTickCmd(token))
		case "b":
			if m.busyMessage == "" {
				m.busyMessage = "Creating backup…"
				return m, runWorkspaceBackupCmd()
			}
		case "x":
			if m.busyMessage == "" {
				m.confirmClean = true
			}
		case "esc", "n":
			m.confirmClean = false
		case "y":
			if m.confirmClean && m.busyMessage == "" {
				m.confirmClean = false
				m.busyMessage = "Cleaning outputs…"
				return m, runWorkspaceCleanCmd()
			}
		}
	case workspaceLoadedMsg:
		if msg.token != m.loadToken {
			return m, nil
		}
		m.loading = false
		m.root = msg.root
		m.used = msg.used
		m.total = msg.total
		m.usages = msg.usages
		m.lastBackup = msg.lastBackup
		m.lastErr = msg.loadErr
	case workspaceTickMsg:
		if !m.active || msg.token != m.loadToken {
			return m, nil
		}
		return m, tea.Batch(loadWorkspaceCmd(msg.token), workspaceTickCmd(msg.token))
	case workspaceOpMsg:
		m.busyMessage = ""
		if msg.err != nil {
			m.lastErr = msg.err
			break
		}
		m.completionNote = msg.detail
		m.loading = true
		m.loadToken++
		token := m.loadToken
		return m, tea.Batch(loadWorkspaceCmd(token), workspaceNoteExpiryCmd())
	case workspaceNoteExpiredMsg:
		m.completionNote = ""
	}
	return m, nil
}

func (m WorkspaceModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading workspace…")
	}
	if m.lastErr != nil {
		return errorText(m.lastErr.Error())
	}

	lines := []string{
		m.root,
		"",
		fmt.Sprintf("Total    %s  %s free / %s (%.0f%%)", m.totalBar(), humanBytes(m.total-m.used), humanBytes(m.total), m.usedRatio()*100),
		"",
	}
	maxUsage := int64(1)
	for _, usage := range m.usages {
		if usage.Bytes > maxUsage {
			maxUsage = usage.Bytes
		}
	}
	for _, usage := range m.usages {
		lines = append(lines, fmt.Sprintf("%-10s %-8s %s", usage.Name, usage.Human(), m.directoryBar(usage.Bytes, maxUsage)))
	}
	lines = append(lines, "")
	if m.busyMessage != "" {
		lines = append(lines, warnText(m.busyMessage))
	} else {
		lines = append(lines, fmt.Sprintf("[b] backup notebooks + models      Last backup: %s", relativeBackupTime(m.lastBackup)))
	}
	if m.confirmClean {
		lines = append(lines, fmt.Sprintf("Clean outputs? %s will be freed. y confirm · esc cancel", m.outputsUsage().Human()))
	} else {
		lines = append(lines, fmt.Sprintf("[x] clean outputs                  %s will be freed", m.outputsUsage().Human()))
	}
	if m.completionNote != "" {
		lines = append(lines, successText(m.completionNote))
	}
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) HelpView() string {
	return "Workspace\nb backup · x clean outputs · r refresh · j/k move in other views"
}

func (m WorkspaceModel) usedRatio() float64 {
	if m.total == 0 {
		return 0
	}
	return float64(m.used) / float64(m.total)
}

func (m WorkspaceModel) totalBar() string {
	width := max(20, min(36, m.width-34))
	ratio := m.usedRatio()
	freeRatio := 1 - ratio
	styleThreshold := false
	if freeRatio < 0.20 {
		styleThreshold = true
	}
	return gradientBar(width, ratio, styleThreshold)
}

func (m WorkspaceModel) directoryBar(value, maxValue int64) string {
	if maxValue <= 0 {
		return ""
	}
	width := max(20, min(40, m.width-26))
	ratio := float64(value) / float64(maxValue)
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMid)).Render(strings.Repeat("█", filled)) +
		mutedText(strings.Repeat("░", width-filled))
}

func (m WorkspaceModel) outputsUsage() workspacepkg.Usage {
	for _, usage := range m.usages {
		if usage.Name == "outputs" {
			return usage
		}
	}
	return workspacepkg.Usage{Name: "outputs"}
}

func loadWorkspaceCmd(token int) tea.Cmd {
	return func() tea.Msg {
		if err := workspaceEnsureFn(); err != nil {
			return workspaceLoadedMsg{token: token, loadErr: err}
		}
		usages, err := workspaceStatusFn()
		if err != nil {
			return workspaceLoadedMsg{token: token, loadErr: err}
		}
		root := workspaceRootFn()
		var stat syscall.Statfs_t
		if err := syscall.Statfs(root, &stat); err != nil {
			return workspaceLoadedMsg{token: token, loadErr: err}
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		return workspaceLoadedMsg{
			token:      token,
			root:       root,
			total:      total,
			used:       used,
			usages:     usages,
			lastBackup: latestBackupTime(filepath.Join(root, "backups")),
		}
	}
}

func workspaceTickCmd(token int) tea.Cmd {
	return tea.Tick(workspaceRefreshInterval, func(time.Time) tea.Msg {
		return workspaceTickMsg{token: token}
	})
}

func workspaceNoteExpiryCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return workspaceNoteExpiredMsg{}
	})
}

func runWorkspaceBackupCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := workspaceBackupFn()
		if err != nil {
			return workspaceOpMsg{kind: "backup", err: err}
		}
		return workspaceOpMsg{kind: "backup", success: true, detail: "backup created at " + path}
	}
}

func runWorkspaceCleanCmd() tea.Cmd {
	return func() tea.Msg {
		if err := workspaceCleanFn(); err != nil {
			return workspaceOpMsg{kind: "clean", err: err}
		}
		return workspaceOpMsg{kind: "clean", success: true, detail: "outputs cleaned"}
	}
}

func latestBackupTime(dir string) time.Time {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}
	}
	latest := time.Time{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func relativeBackupTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	delta := time.Since(t).Round(time.Hour)
	if delta < time.Minute {
		return "just now"
	}
	return delta.String() + " ago"
}
