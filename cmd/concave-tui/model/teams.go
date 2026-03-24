package model

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

const teamsRefreshInterval = 15 * time.Second

type teamsLoadedMsg struct {
	token       int
	teams       []apiclient.TeamSummary
	unavailable bool
	message     string
	err         error
}

type teamsTickMsg struct {
	token int
}

type TeamsModel struct {
	width       int
	height      int
	active      bool
	role        tuiauth.Role
	loading     bool
	token       int
	selected    int
	expanded    map[string]bool
	teams       []apiclient.TeamSummary
	unavailable bool
	message     string
	lastErr     error
}

func NewTeamsModel() TeamsModel {
	return TeamsModel{
		loading:  true,
		expanded: map[string]bool{},
	}
}

func (m *TeamsModel) SetRole(role tuiauth.Role) {
	m.role = role
}

func (m *TeamsModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.token++
	token := m.token
	return tea.Batch(loadTeamsCmd(token), teamsTickCmd(token, teamsRefreshInterval))
}

func (m *TeamsModel) Deactivate() {
	m.active = false
	m.token++
}

func (m *TeamsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m TeamsModel) Update(msg tea.Msg) (TeamsModel, tea.Cmd) {
	if m.role < tuiauth.RoleAdmin {
		return m, nil
	}
	switch msg := msg.(type) {
	case teamsLoadedMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.loading = false
		m.teams = msg.teams
		m.unavailable = msg.unavailable
		m.message = msg.message
		m.lastErr = msg.err
		if m.selected >= len(m.teams) && len(m.teams) > 0 {
			m.selected = len(m.teams) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
	case teamsTickMsg:
		if m.active && msg.token == m.token {
			return m, tea.Batch(loadTeamsCmd(msg.token), teamsTickCmd(msg.token, teamsRefreshInterval))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			m.token++
			token := m.token
			return m, tea.Batch(loadTeamsCmd(token), teamsTickCmd(token, teamsRefreshInterval))
		case "j", "down":
			if len(m.teams) > 0 && m.selected < len(m.teams)-1 {
				m.selected++
			}
		case "k", "up":
			if len(m.teams) > 0 && m.selected > 0 {
				m.selected--
			}
		case "enter":
			if selected := m.selectedTeam(); selected != nil {
				m.expanded[selected.Name] = !m.expanded[selected.Name]
			}
		case "g":
			m.selected = 0
		case "G":
			if len(m.teams) > 0 {
				m.selected = len(m.teams) - 1
			}
		}
	}
	return m, nil
}

func (m TeamsModel) View() string {
	if m.role < tuiauth.RoleAdmin {
		return mutedText("Teams view is available to Admin only")
	}
	if m.loading && len(m.teams) == 0 && !m.unavailable {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading teams…")
	}

	lines := []string{
		"Teams                                   [r] refresh · j/k teams · enter users",
		"",
	}

	if m.unavailable {
		lines = append(lines,
			warnText("Team management unavailable"),
			"",
			mutedText("The teams view will populate after compute engine support is available."),
		)
		if msg := strings.TrimSpace(m.message); msg != "" {
			lines = append(lines, "", mutedText(msg))
		}
		return strings.Join(lines, "\n")
	}

	lines = append(lines, m.renderTeamTable()...)
	if selected := m.selectedTeam(); selected != nil {
		lines = append(lines, "", "Selected team")
		lines = append(lines, m.renderTeamDetails(*selected)...)
	}
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m TeamsModel) HelpView() string {
	return "Teams\nr refresh · j/k move · enter expand"
}

func (m TeamsModel) renderTeamTable() []string {
	if len(m.teams) == 0 {
		return []string{mutedText("No teams configured")}
	}
	lines := []string{"Team             Preset           Users  CPU quota   RAM quota   GPU"}
	for idx, team := range m.teams {
		prefix := " "
		if idx == m.selected {
			prefix = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-16s %-15s %-5s %-11s %-11s %s", prefix, truncate(fallbackText(team.Name, "unknown"), 16), truncate(fallbackText(team.Preset, "unknown"), 15), fmt.Sprintf("%d", len(team.Users)), formatTeamCPU(team.Quota.CPUCores), formatTeamRAM(team.Quota.MemoryGB), teamGPUText(team)))
	}
	return lines
}

func (m TeamsModel) renderTeamDetails(team apiclient.TeamSummary) []string {
	lines := []string{
		fmt.Sprintf("CPU quota: %s", formatTeamCPU(team.Quota.CPUCores)),
		fmt.Sprintf("RAM quota: %s", formatTeamRAM(team.Quota.MemoryGB)),
		fmt.Sprintf("GPU quota: %s", teamGPUText(team)),
		fmt.Sprintf("Users: %s", teamUsersText(team.Users)),
	}
	if !team.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", relativeCheckTime(team.CreatedAt)))
	}
	if !team.UpdatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Updated: %s", relativeCheckTime(team.UpdatedAt)))
	}
	if len(team.Users) > 0 && m.expanded[team.Name] {
		lines = append(lines, "Members:")
		for _, user := range team.Users {
			lines = append(lines, "  - "+user)
		}
	}
	return lines
}

func (m TeamsModel) selectedTeam() *apiclient.TeamSummary {
	if len(m.teams) == 0 || m.selected < 0 || m.selected >= len(m.teams) {
		return nil
	}
	team := m.teams[m.selected]
	return &team
}

func teamUsersText(users []string) string {
	if len(users) == 0 {
		return "0"
	}
	return fmt.Sprintf("%d", len(users))
}

func formatTeamCPU(value float64) string {
	if value <= 0 {
		return "—"
	}
	if math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0fc/user", value)
	}
	return fmt.Sprintf("%.1fc/user", value)
}

func formatTeamRAM(value float64) string {
	if value <= 0 {
		return "—"
	}
	if math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0fGB/user", value)
	}
	return fmt.Sprintf("%.1fGB/user", value)
}

func teamGPUText(team apiclient.TeamSummary) string {
	switch strings.ToLower(strings.TrimSpace(team.Preset)) {
	case "research-team":
		return "share"
	case "inference-node", "training-node":
		return "slice"
	case "student-lab":
		return "cpu only"
	}
	if team.Quota.GPUFraction > 0 {
		return "slice"
	}
	return "share"
}

func loadTeamsCmd(token int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		teams, err := apiTeamsFn(ctx)
		if err != nil {
			if isUnavailableAPIError(err) {
				return teamsLoadedMsg{token: token, unavailable: true, message: "team management is not yet available"}
			}
			return teamsLoadedMsg{token: token, err: err}
		}
		if teams.Available != nil && !*teams.Available {
			return teamsLoadedMsg{token: token, unavailable: true, message: teams.Message}
		}
		return teamsLoadedMsg{token: token, teams: teams.Teams}
	}
}

func teamsTickCmd(token int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return teamsTickMsg{token: token} })
}
