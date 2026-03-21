package model

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const workspaceRefreshInterval = 5 * time.Second

type workspaceLoadedMsg struct {
	token   int
	used    uint64
	total   uint64
	usages  []string
	root    string
	loadErr error
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
	usages         []string
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
		return m, loadWorkspaceCmd(token)
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
		fmt.Sprintf("%s   %s free of %s", m.root, humanBytes(m.total-m.used), humanBytes(m.total)),
		m.usageBar(),
		"",
	}
	lines = append(lines, m.usages...)
	lines = append(lines, "")
	if m.busyMessage != "" {
		lines = append(lines, warnText(m.busyMessage))
	} else {
		lines = append(lines, "b  backup notebooks + models")
	}
	if m.confirmClean {
		lines = append(lines, "Clean outputs? y confirm · esc cancel")
	} else {
		lines = append(lines, "x  clean outputs")
	}
	if m.completionNote != "" {
		lines = append(lines, successText(m.completionNote))
	}
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) HelpView() string {
	return "Workspace\nb backup · x clean outputs · r refresh"
}

func (m WorkspaceModel) usageBar() string {
	if m.total == 0 {
		return ""
	}
	ratio := float64(m.used) / float64(m.total)
	filled := int(ratio * 32)
	if filled > 32 {
		filled = 32
	}
	color := ColorMid
	if ratio >= 0.9 {
		color = ColorError
	} else if ratio >= 0.8 {
		color = ColorWarn
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 32-filled)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(bar) + fmt.Sprintf("   %.0f%%", ratio*100)
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

		lines := make([]string, 0, len(usages))
		for _, usage := range usages {
			lines = append(lines, fmt.Sprintf("%-10s %s", usage.Name, usage.Human()))
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		return workspaceLoadedMsg{
			token:  token,
			root:   root,
			total:  total,
			used:   used,
			usages: lines,
		}
	}
}

func workspaceTickCmd(token int) tea.Cmd {
	return tea.Tick(workspaceRefreshInterval, func(time.Time) tea.Msg {
		return workspaceTickMsg{token: token}
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
