package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

const systemRefreshInterval = 5 * time.Second

type systemLoadedMsg struct {
	info apiclient.SystemInfo
	err  error
}

type systemTickMsg struct{}

type systemReconnectMsg struct {
	online bool
}

type systemActionDoneMsg struct {
	action string
	err    error
}

type SystemModel struct {
	width         int
	height        int
	active        bool
	role          tuiauth.Role
	loading       bool
	info          apiclient.SystemInfo
	lastErr       error
	confirmAction string
	notice        string
	reconnecting  bool
}

func NewSystemModel() SystemModel {
	return SystemModel{loading: true}
}

func (m *SystemModel) SetRole(role tuiauth.Role) {
	m.role = role
}

func (m *SystemModel) Activate() tea.Cmd {
	m.active = true
	if m.role < tuiauth.RoleAdmin {
		return nil
	}
	m.loading = true
	return tea.Batch(loadSystemCmd(), systemTickCmd(systemRefreshInterval))
}

func (m *SystemModel) Deactivate() { m.active = false }

func (m *SystemModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m SystemModel) Update(msg tea.Msg) (SystemModel, tea.Cmd) {
	if m.role < tuiauth.RoleAdmin {
		return m, nil
	}
	switch msg := msg.(type) {
	case systemLoadedMsg:
		m.loading = false
		m.info = msg.info
		m.lastErr = msg.err
	case systemReconnectMsg:
		if msg.online {
			m.reconnecting = false
			m.notice = "✓ Server is back online. Press any key to reconnect."
			return m, loadSystemCmd()
		}
		if m.reconnecting {
			return m, systemReconnectTickCmd()
		}
	case systemActionDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		switch msg.action {
		case "reboot", "shutdown":
			m.notice = "⟳ " + strings.Title(strings.ReplaceAll(msg.action, "-", " ")) + " initiated. Waiting for reconnect..."
			m.reconnecting = true
			return m, systemReconnectTickCmd()
		default:
			m.notice = "✓ " + strings.ReplaceAll(msg.action, "-", " ") + " complete"
			return m, loadSystemCmd()
		}
	case systemTickMsg:
		if m.active && !m.reconnecting {
			return m, tea.Batch(loadSystemCmd(), systemTickCmd(systemRefreshInterval))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.confirmAction = "reboot"
		case "x":
			m.confirmAction = "shutdown"
		case "d":
			m.confirmAction = "restart-docker"
		case "y":
			if m.confirmAction != "" {
				action := m.confirmAction
				m.confirmAction = ""
				return m, runSystemActionCmd(action)
			}
		case "esc", "n":
			m.confirmAction = ""
		default:
			if m.notice == "✓ Server is back online. Press any key to reconnect." {
				m.notice = ""
				return m, loadSystemCmd()
			}
		}
	}
	return m, nil
}

func (m SystemModel) View() string {
	if m.role < tuiauth.RoleAdmin {
		return mutedText("System view is available to Admin only")
	}
	if m.loading && m.info.Hostname == "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading system…")
	}

	lines := []string{
		"System                              [r] reboot · [x] shutdown · [d] restart docker",
		"",
		fmt.Sprintf("hostname      %s", fallbackText(m.info.Hostname, "unknown")),
		fmt.Sprintf("uptime        %s", fallbackText(m.info.Uptime, "unknown")),
		fmt.Sprintf("kernel        %s", fallbackText(m.info.Kernel, "unknown")),
		fmt.Sprintf("OS            %s", fallbackText(m.info.OS, "unknown")),
		fmt.Sprintf("concave       %s", fallbackText(m.info.Concave, "unknown")),
		fmt.Sprintf("docker        %s", fallbackText(m.info.Docker, "unknown")),
		"",
		"Services",
	}
	for _, service := range m.info.Services {
		lines = append(lines, fmt.Sprintf("%-16s %s %s", service.Name, statusGlyph(service.Status), service.Status))
	}
	if m.confirmAction != "" {
		lines = append(lines, "", fmt.Sprintf("%s the server? All sessions will disconnect. [y/N]", strings.Title(strings.ReplaceAll(m.confirmAction, "-", " "))))
	}
	if m.notice != "" {
		lines = append(lines, "", m.notice)
	}
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m SystemModel) HelpView() string {
	return "System\nr reboot · x shutdown · d restart docker"
}

func loadSystemCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		info, err := apiSystemInfoFn(ctx)
		return systemLoadedMsg{info: info, err: err}
	}
}

func systemTickCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return systemTickMsg{} })
}

func runSystemActionCmd(action string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return systemActionDoneMsg{action: action, err: apiSystemActionFn(ctx, action)}
	}
}

func systemReconnectTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := sharedClient.Health(ctx)
		return systemReconnectMsg{online: err == nil}
	})
}

func statusGlyph(status string) string {
	switch strings.ToLower(status) {
	case "active", "running":
		return successText("●")
	default:
		return warnText("○")
	}
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
