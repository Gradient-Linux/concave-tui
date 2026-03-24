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

const environmentRefreshInterval = 10 * time.Second

type environmentLoadedMsg struct {
	token       int
	status      apiclient.ResolverStatus
	unavailable bool
	err         error
}

type environmentTickMsg struct {
	token int
}

type EnvironmentModel struct {
	width       int
	height      int
	active      bool
	role        tuiauth.Role
	loading     bool
	token       int
	selected    int
	status      apiclient.ResolverStatus
	unavailable bool
	lastErr     error
}

func NewEnvironmentModel() EnvironmentModel {
	return EnvironmentModel{loading: true}
}

func (m *EnvironmentModel) SetRole(role tuiauth.Role) {
	m.role = role
}

func (m *EnvironmentModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.token++
	token := m.token
	return tea.Batch(loadEnvironmentCmd(token), environmentTickCmd(token, environmentRefreshInterval))
}

func (m *EnvironmentModel) Deactivate() {
	m.active = false
	m.token++
}

func (m *EnvironmentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m EnvironmentModel) Update(msg tea.Msg) (EnvironmentModel, tea.Cmd) {
	if m.role < tuiauth.RoleViewer {
		return m, nil
	}
	switch msg := msg.(type) {
	case environmentLoadedMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.loading = false
		m.status = msg.status
		m.unavailable = msg.unavailable
		m.lastErr = msg.err
		if len(m.status.GroupReports) > 0 && m.selected >= len(m.status.GroupReports) {
			m.selected = len(m.status.GroupReports) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
	case environmentTickMsg:
		if m.active && msg.token == m.token {
			return m, tea.Batch(loadEnvironmentCmd(msg.token), environmentTickCmd(msg.token, environmentRefreshInterval))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			m.token++
			token := m.token
			return m, tea.Batch(loadEnvironmentCmd(token), environmentTickCmd(token, environmentRefreshInterval))
		case "j", "down":
			if len(m.status.GroupReports) > 0 && m.selected < len(m.status.GroupReports)-1 {
				m.selected++
			}
		case "k", "up":
			if len(m.status.GroupReports) > 0 && m.selected > 0 {
				m.selected--
			}
		case "g":
			m.selected = 0
		case "G":
			if len(m.status.GroupReports) > 0 {
				m.selected = len(m.status.GroupReports) - 1
			}
		}
	}
	return m, nil
}

func (m EnvironmentModel) View() string {
	if m.role < tuiauth.RoleViewer {
		return mutedText("Environment view is available to Viewer and above")
	}
	if m.loading && !m.unavailable && m.status.SocketPath == "" && len(m.status.GroupReports) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading environment…")
	}

	lines := []string{
		fmt.Sprintf("Environment                              [r] refresh · j/k reports"),
		"",
	}

	if m.unavailable {
		lines = append(lines,
			warnText("Resolver unavailable"),
			"",
			mutedText("The environment view will populate after concave-resolver is available."),
		)
		if msg := strings.TrimSpace(m.status.Message); msg != "" {
			lines = append(lines, "", mutedText(msg))
		}
		return strings.Join(lines, "\n")
	}

	lines = append(lines, m.renderStatusSummary(), "", m.renderSnapshotSummary(), "", "Reports")
	lines = append(lines, m.renderReportTable()...)

	if selected := m.selectedReport(); selected != nil {
		lines = append(lines, "", "Selected report")
		lines = append(lines, m.renderReportDetails(*selected)...)
	}
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m EnvironmentModel) HelpView() string {
	return "Environment\nr refresh · j/k choose report"
}

func (m EnvironmentModel) renderStatusSummary() string {
	state := warnText("not running")
	if m.status.Running {
		state = successText("running")
	}
	lastScan := "never"
	if !m.status.LastScan.IsZero() {
		lastScan = relativeCheckTime(m.status.LastScan)
	}
	return fmt.Sprintf("Resolver: %-9s last scan %s · reports %d", state, lastScan, len(m.status.GroupReports))
}

func (m EnvironmentModel) renderSnapshotSummary() string {
	count := m.status.SnapshotCount
	if count <= 0 {
		return mutedText("No snapshots stored")
	}
	return fmt.Sprintf("Snapshots stored: %d · socket: %s", count, fallbackText(m.status.SocketPath, "unknown"))
}

func (m EnvironmentModel) renderReportTable() []string {
	if len(m.status.GroupReports) == 0 {
		return []string{mutedText("No drift reports available")}
	}
	lines := []string{"Group           User         Diffs  Tier   Updated"}
	for idx, report := range m.status.GroupReports {
		prefix := " "
		if idx == m.selected {
			prefix = ">"
		}
		user := report.User
		if strings.TrimSpace(user) == "" {
			user = "group"
		}
		lines = append(lines, fmt.Sprintf("%s %-14s %-11s %-5d %-6s %s", prefix, truncate(report.Group, 14), truncate(user, 11), len(report.Diffs), reportTierLabel(report), relativeCheckTime(report.Timestamp)))
	}
	return lines
}

func (m EnvironmentModel) renderReportDetails(report apiclient.DriftReport) []string {
	if len(report.Diffs) == 0 {
		return []string{mutedText("This report is clean")}
	}
	lines := []string{fmt.Sprintf("%-16s %-16s %-16s %-8s %s", "Package", "Baseline", "Current", "Tier", "Reason")}
	for _, diff := range report.Diffs {
		lines = append(lines, renderDiffRow(diff))
	}
	return lines
}

func renderDiffRow(diff apiclient.PackageDiff) string {
	tier := driftTierText(diff.Tier)
	return fmt.Sprintf("%-16s %-16s %-16s %-8s %s", truncate(diff.Name, 16), truncate(diff.Baseline, 16), truncate(diff.Current, 16), tier, truncate(diff.Reason, 48))
}

func reportTierLabel(report apiclient.DriftReport) string {
	tier := apiclient.DriftTier(0)
	for _, diff := range report.Diffs {
		if diff.Tier > tier {
			tier = diff.Tier
		}
	}
	return driftTierText(tier)
}

func driftTierText(tier apiclient.DriftTier) string {
	switch tier {
	case 0:
		return successText("safe")
	case 1:
		return warnText("flag")
	case 2:
		return errorText("leave")
	default:
		return mutedText("unknown")
	}
}

func (m EnvironmentModel) selectedReport() *apiclient.DriftReport {
	if len(m.status.GroupReports) == 0 || m.selected < 0 || m.selected >= len(m.status.GroupReports) {
		return nil
	}
	report := m.status.GroupReports[m.selected]
	return &report
}

func loadEnvironmentCmd(token int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		status, err := apiResolverStatusFn(ctx)
		if err != nil {
			if isUnavailableAPIError(err) {
				return environmentLoadedMsg{token: token, unavailable: true, err: nil}
			}
			return environmentLoadedMsg{token: token, err: err}
		}
		if status.Available != nil && !*status.Available {
			return environmentLoadedMsg{token: token, status: status, unavailable: true}
		}
		return environmentLoadedMsg{token: token, status: status}
	}
}

func environmentTickCmd(token int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return environmentTickMsg{token: token} })
}
