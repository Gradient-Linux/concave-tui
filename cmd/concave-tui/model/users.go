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

const usersRefreshInterval = 5 * time.Second

type usersLoadedMsg struct {
	users []apiclient.UserActivity
	err   error
}

type usersTickMsg struct{}

type UsersModel struct {
	width       int
	height      int
	active      bool
	role        tuiauth.Role
	loading     bool
	users       []apiclient.UserActivity
	selected    int
	expanded    map[string]bool
	lastUpdated time.Time
	lastErr     error
}

func NewUsersModel() UsersModel {
	return UsersModel{
		loading:  true,
		expanded: map[string]bool{},
	}
}

func (m *UsersModel) SetRole(role tuiauth.Role) {
	m.role = role
}

func (m *UsersModel) Activate() tea.Cmd {
	m.active = true
	if m.role < tuiauth.RoleAdmin {
		return nil
	}
	m.loading = true
	return tea.Batch(loadUsersCmd(), usersTickCmd(usersRefreshInterval))
}

func (m *UsersModel) Deactivate() { m.active = false }

func (m *UsersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m UsersModel) Update(msg tea.Msg) (UsersModel, tea.Cmd) {
	if m.role < tuiauth.RoleAdmin {
		return m, nil
	}
	switch msg := msg.(type) {
	case usersLoadedMsg:
		m.loading = false
		m.users = msg.users
		m.lastErr = msg.err
		m.lastUpdated = time.Now()
		if m.selected >= len(m.users) && len(m.users) > 0 {
			m.selected = len(m.users) - 1
		}
	case usersTickMsg:
		if m.active {
			return m, tea.Batch(loadUsersCmd(), usersTickCmd(usersRefreshInterval))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.selected < len(m.users)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "enter":
			if len(m.users) > 0 {
				name := m.users[m.selected].Username
				m.expanded[name] = !m.expanded[name]
			}
		case "g":
			m.selected = 0
		case "G":
			if len(m.users) > 0 {
				m.selected = len(m.users) - 1
			}
		}
	}
	return m, nil
}

func (m UsersModel) View() string {
	if m.role < tuiauth.RoleAdmin {
		return mutedText("Users view is available to Admin only")
	}
	if m.loading && len(m.users) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading user activity…")
	}

	lines := []string{
		fmt.Sprintf("Active Users                                    last updated %s", relativeCheckTime(m.lastUpdated)),
		"",
		"User        Role        GPU Mem    Active",
		"──────────  ──────────  ─────────  ──────────",
	}
	totalGPU := 0
	for idx, user := range m.users {
		prefix := " "
		if idx == m.selected {
			prefix = "►"
		}
		totalGPU += user.GPUMemoryMiB
		lines = append(lines, fmt.Sprintf("%s %-10s %-10s %-9s %s", prefix, user.Username, user.Role.String(), formatMiB(user.GPUMemoryMiB), relativeCheckTime(user.LastActive)))
		if m.expanded[user.Username] {
			if len(user.Containers) == 0 {
				lines = append(lines, "    (no containers)")
			}
			for _, container := range user.Containers {
				lines = append(lines, fmt.Sprintf("    %s (%s)", container.Name, container.Status))
			}
		}
	}
	lines = append(lines, "", fmt.Sprintf("GPU total: %s used", formatMiB(totalGPU)))
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m UsersModel) HelpView() string {
	return "Users\nj/k move · enter expand"
}

func loadUsersCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		users, err := apiUsersActivityFn(ctx)
		return usersLoadedMsg{users: users, err: err}
	}
}

func usersTickCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return usersTickMsg{} })
}

func formatMiB(value int) string {
	if value <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f GB", float64(value)/1024)
}
