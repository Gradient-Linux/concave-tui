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

const (
	labEnvsRefreshInterval = 10 * time.Second
	labEnvsCountdownTick   = 1 * time.Second
)

type labEnvsLoadedMsg struct {
	token    int
	response apiclient.LabEnvsResponse
	err      error
}

type labEnvsActionMsg struct {
	token  int
	action string
	envID  string
	err    error
}

type labEnvsTickMsg struct {
	token int
}

type labEnvsCountdownMsg struct {
	token int
}

type LabEnvsModel struct {
	width    int
	height   int
	active   bool
	role     tuiauth.Role
	loading  bool
	token    int
	selected int
	response apiclient.LabEnvsResponse
	now      time.Time
	busy     bool
	lastErr  error
	lastInfo string
}

// NewLabEnvsModel returns a fresh lab envs model.
func NewLabEnvsModel() LabEnvsModel {
	return LabEnvsModel{loading: true, now: time.Now()}
}

// SetRole applies the current session role.
func (m *LabEnvsModel) SetRole(role tuiauth.Role) {
	m.role = role
}

// Activate triggers a fresh load and starts the refresh + countdown timers.
func (m *LabEnvsModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.token++
	token := m.token
	return tea.Batch(
		loadLabEnvsCmd(token),
		labEnvsTickCmd(token, labEnvsRefreshInterval),
		labEnvsCountdownCmd(token),
	)
}

// Deactivate stops refreshes.
func (m *LabEnvsModel) Deactivate() {
	m.active = false
	m.token++
}

// SetSize updates the layout bounds.
func (m *LabEnvsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update processes tea messages for the lab envs view.
func (m LabEnvsModel) Update(msg tea.Msg) (LabEnvsModel, tea.Cmd) {
	if m.role < tuiauth.RoleViewer {
		return m, nil
	}
	switch msg := msg.(type) {
	case labEnvsLoadedMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.loading = false
		m.response = msg.response
		m.lastErr = msg.err
		if m.selected >= len(m.response.Envs) && len(m.response.Envs) > 0 {
			m.selected = len(m.response.Envs) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
	case labEnvsActionMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.busy = false
		if msg.err != nil {
			m.lastErr = msg.err
		} else {
			m.lastErr = nil
			m.lastInfo = fmt.Sprintf("%s %s", msg.action, msg.envID)
		}
		m.token++
		return m, loadLabEnvsCmd(m.token)
	case labEnvsTickMsg:
		if m.active && msg.token == m.token {
			return m, tea.Batch(loadLabEnvsCmd(msg.token), labEnvsTickCmd(msg.token, labEnvsRefreshInterval))
		}
	case labEnvsCountdownMsg:
		if m.active && msg.token == m.token {
			m.now = time.Now()
			return m, labEnvsCountdownCmd(msg.token)
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m LabEnvsModel) handleKey(msg tea.KeyMsg) (LabEnvsModel, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.token++
		m.loading = true
		return m, loadLabEnvsCmd(m.token)
	case "j", "down":
		if m.selected < len(m.response.Envs)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "e":
		if m.role < tuiauth.RoleDeveloper {
			return m, nil
		}
		env := m.selectedEnv()
		if env == nil || m.busy {
			return m, nil
		}
		m.busy = true
		m.token++
		return m, extendLabEnvCmd(m.token, env.ID, 3600)
	case "a":
		if m.role < tuiauth.RoleOperator {
			return m, nil
		}
		env := m.selectedEnv()
		if env == nil || m.busy {
			return m, nil
		}
		m.busy = true
		m.token++
		return m, archiveLabEnvCmd(m.token, env.ID)
	}
	return m, nil
}

// View renders the lab envs page.
func (m LabEnvsModel) View() string {
	if m.role < tuiauth.RoleViewer {
		return mutedText("Lab envs are available to Viewer and above")
	}
	if m.loading && len(m.response.Envs) == 0 && m.lastErr == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading lab envs…")
	}

	var lines []string
	lines = append(lines, m.renderHeader(), "")
	lines = append(lines, m.renderTable()...)
	if sel := m.selectedEnv(); sel != nil {
		lines = append(lines, "", "Selected env")
		lines = append(lines, m.renderDetails(*sel)...)
	} else if len(m.response.Envs) == 0 {
		lines = append(lines, mutedText("No lab envs yet. Launch one via `concave lab envs launch`."))
	}
	if m.lastInfo != "" {
		lines = append(lines, "", mutedText(m.lastInfo))
	}
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

// HelpView summarises keybindings.
func (m LabEnvsModel) HelpView() string {
	return "Lab Envs\nr refresh · j/k select · e extend +1h (dev) · a archive (ops)"
}

func (m LabEnvsModel) renderHeader() string {
	return fmt.Sprintf("Lab Envs — driver %s · hot %s · cold %s",
		fallbackText(m.response.Active, "docker"),
		fallbackText(m.response.Storage.HotTier, "unset"),
		fallbackText(m.response.Storage.ColdTier, "unset"),
	)
}

func (m LabEnvsModel) renderTable() []string {
	lines := []string{"ID               OWNER     DRIVER    STATUS     IMAGE                         TTL"}
	if len(m.response.Envs) == 0 {
		return lines
	}
	for idx, env := range m.response.Envs {
		prefix := " "
		if idx == m.selected {
			prefix = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-15s %-9s %-9s %-10s %-29s %s",
			prefix,
			truncate(env.ID, 15),
			truncate(fallbackText(env.Owner, "-"), 9),
			truncate(fallbackText(env.Driver, "-"), 9),
			truncate(env.Status, 10),
			truncate(env.Image, 29),
			formatTTL(m.now, env.ExpiresAt),
		))
	}
	return lines
}

func (m LabEnvsModel) renderDetails(env apiclient.LabEnv) []string {
	lines := []string{
		fmt.Sprintf("ID: %s", env.ID),
		fmt.Sprintf("Jupyter: %s", fallbackText(env.JupyterURL, "unavailable")),
		fmt.Sprintf("Hot tier: %s", fallbackText(env.HotTierPath, "-")),
		fmt.Sprintf("Cold tier: %s", fallbackText(env.ColdTierPath, "-")),
	}
	if env.ArchiveRef != "" {
		lines = append(lines, fmt.Sprintf("Archive: %s", env.ArchiveRef))
	}
	if env.LastError != "" {
		lines = append(lines, errorText(env.LastError))
	}
	return lines
}

func (m LabEnvsModel) selectedEnv() *apiclient.LabEnv {
	if m.selected < 0 || m.selected >= len(m.response.Envs) {
		return nil
	}
	env := m.response.Envs[m.selected]
	return &env
}

func formatTTL(now time.Time, expiresAt time.Time) string {
	if expiresAt.IsZero() {
		return "—"
	}
	remaining := time.Until(expiresAt)
	if !now.IsZero() {
		remaining = expiresAt.Sub(now)
	}
	if remaining <= 0 {
		return "expired"
	}
	hours := int(remaining / time.Hour)
	minutes := int((remaining % time.Hour) / time.Minute)
	seconds := int((remaining % time.Minute) / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func loadLabEnvsCmd(token int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := apiLabEnvsFn(ctx)
		return labEnvsLoadedMsg{token: token, response: response, err: err}
	}
}

func extendLabEnvCmd(token int, id string, seconds int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := apiLabExtendFn(ctx, id, seconds)
		return labEnvsActionMsg{token: token, action: "extend", envID: id, err: err}
	}
}

func archiveLabEnvCmd(token int, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := apiLabArchiveFn(ctx, id)
		return labEnvsActionMsg{token: token, action: "archive", envID: id, err: err}
	}
}

func labEnvsTickCmd(token int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return labEnvsTickMsg{token: token}
	})
}

func labEnvsCountdownCmd(token int) tea.Cmd {
	return tea.Tick(labEnvsCountdownTick, func(time.Time) tea.Msg {
		return labEnvsCountdownMsg{token: token}
	})
}
