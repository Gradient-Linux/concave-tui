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

const fleetRefreshInterval = 10 * time.Second

type fleetLoadedMsg struct {
	token       int
	self        apiclient.FleetNode
	peers       []apiclient.FleetNode
	unavailable bool
	message     string
	err         error
}

type fleetTickMsg struct {
	token int
}

type FleetModel struct {
	width       int
	height      int
	active      bool
	role        tuiauth.Role
	loading     bool
	token       int
	selected    int
	self        apiclient.FleetNode
	peers       []apiclient.FleetNode
	unavailable bool
	message     string
	lastErr     error
}

func NewFleetModel() FleetModel {
	return FleetModel{loading: true}
}

func (m *FleetModel) SetRole(role tuiauth.Role) {
	m.role = role
}

func (m *FleetModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.token++
	token := m.token
	return tea.Batch(loadFleetCmd(token), fleetTickCmd(token, fleetRefreshInterval))
}

func (m *FleetModel) Deactivate() {
	m.active = false
	m.token++
}

func (m *FleetModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m FleetModel) Update(msg tea.Msg) (FleetModel, tea.Cmd) {
	if m.role < tuiauth.RoleViewer {
		return m, nil
	}
	switch msg := msg.(type) {
	case fleetLoadedMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.loading = false
		m.self = msg.self
		m.peers = msg.peers
		m.unavailable = msg.unavailable
		m.message = msg.message
		m.lastErr = msg.err
		if m.selected >= len(m.peers) && len(m.peers) > 0 {
			m.selected = len(m.peers) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
	case fleetTickMsg:
		if m.active && msg.token == m.token {
			return m, tea.Batch(loadFleetCmd(msg.token), fleetTickCmd(msg.token, fleetRefreshInterval))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			m.token++
			token := m.token
			return m, tea.Batch(loadFleetCmd(token), fleetTickCmd(token, fleetRefreshInterval))
		case "j", "down":
			if len(m.peers) > 0 && m.selected < len(m.peers)-1 {
				m.selected++
			}
		case "k", "up":
			if len(m.peers) > 0 && m.selected > 0 {
				m.selected--
			}
		case "g":
			m.selected = 0
		case "G":
			if len(m.peers) > 0 {
				m.selected = len(m.peers) - 1
			}
		}
	}
	return m, nil
}

func (m FleetModel) View() string {
	if m.role < tuiauth.RoleViewer {
		return mutedText("Fleet view is available to Viewer and above")
	}
	if m.loading && m.self.Hostname == "" && len(m.peers) == 0 && !m.unavailable {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading fleet…")
	}

	lines := []string{
		fmt.Sprintf("Fleet                                    [r] refresh · j/k peers"),
		"",
	}

	if m.unavailable {
		lines = append(lines,
			warnText("Mesh unavailable"),
			"",
			mutedText("The fleet view will populate after gradient-mesh is available."),
		)
		if msg := strings.TrimSpace(m.message); msg != "" {
			lines = append(lines, "", mutedText(msg))
		}
		return strings.Join(lines, "\n")
	}

	lines = append(lines, m.renderSelfSummary(), "")
	if len(m.peers) == 0 {
		lines = append(lines, mutedText("No peers visible - running in single node mode"))
	} else {
		lines = append(lines, m.renderPeerTable()...)
		if selected := m.selectedPeer(); selected != nil {
			lines = append(lines, "", "Selected peer")
			lines = append(lines, m.renderPeerDetails(*selected)...)
		}
	}
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m FleetModel) HelpView() string {
	return "Fleet\nr refresh · j/k choose peer"
}

func (m FleetModel) renderSelfSummary() string {
	visibility := fallbackText(m.self.Visibility, "unknown")
	resolver := warnText("offline")
	if m.self.ResolverRunning {
		resolver = successText("running")
	}
	address := fallbackText(m.self.Address, "unknown")
	suites := "none"
	if len(m.self.InstalledSuites) > 0 {
		suites = strings.Join(m.self.InstalledSuites, ", ")
	}
	return fmt.Sprintf("This node: %-20s visibility %s · resolver %s · %s", fallbackText(m.self.Hostname, "unknown"), visibility, resolver, address) + "\n" + fmt.Sprintf("Suites: %s", suites)
}

func (m FleetModel) renderPeerTable() []string {
	lines := []string{"Node                Version    Suites              Resolver  Online"}
	for idx, peer := range m.peers {
		prefix := " "
		if idx == m.selected {
			prefix = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-18s %-10s %-18s %-8s %s", prefix, truncate(fallbackText(peer.Hostname, "unknown"), 18), truncate(fallbackText(peer.GradientVersion, "unknown"), 10), truncate(peerSuites(peer.InstalledSuites), 18), resolverNodeText(peer.ResolverRunning), relativeCheckTime(peer.LastSeen)))
	}
	return lines
}

func (m FleetModel) renderPeerDetails(peer apiclient.FleetNode) []string {
	return []string{
		fmt.Sprintf("Address: %s", fallbackText(peer.Address, "unknown")),
		fmt.Sprintf("Machine ID: %s", fallbackText(peer.MachineID, "unknown")),
		fmt.Sprintf("Visibility: %s", fallbackText(peer.Visibility, "unknown")),
		fmt.Sprintf("Suites: %s", peerSuites(peer.InstalledSuites)),
	}
}

func (m FleetModel) selectedPeer() *apiclient.FleetNode {
	if len(m.peers) == 0 || m.selected < 0 || m.selected >= len(m.peers) {
		return nil
	}
	peer := m.peers[m.selected]
	return &peer
}

func peerSuites(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func resolverNodeText(running bool) string {
	if running {
		return successText("✓")
	}
	return warnText("✗")
}

func loadFleetCmd(token int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		self, selfErr := apiNodeStatusFn(ctx)
		if selfErr != nil && !isUnavailableAPIError(selfErr) {
			return fleetLoadedMsg{token: token, err: selfErr}
		}
		status, err := apiFleetStatusFn(ctx)
		if err != nil {
			if isUnavailableAPIError(err) {
				return fleetLoadedMsg{token: token, self: self, unavailable: true, message: "mesh not configured"}
			}
			return fleetLoadedMsg{token: token, self: self, err: err}
		}
		if status.Available != nil && !*status.Available {
			return fleetLoadedMsg{token: token, self: self, peers: status.Peers, unavailable: true, message: status.Message}
		}
		return fleetLoadedMsg{token: token, self: self, peers: status.Peers}
	}
}

func fleetTickCmd(token int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return fleetTickMsg{token: token} })
}
