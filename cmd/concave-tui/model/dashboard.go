package model

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

const dashboardRefreshInterval = 5 * time.Second

type dashboardSuiteState struct {
	Name       string
	Installed  bool
	Total      int
	Running    int
	Ports      []string
	Containers []dashboardContainerState
}

type dashboardContainerState struct {
	Name    string
	Status  string
	Command string
}

type dashboardLoadedMsg struct {
	token     int
	gpuLine   string
	workspace string
	suites    []dashboardSuiteState
	firstRun  bool
	loadErr   error
}

type dashboardTickMsg struct {
	token int
}

var dashboardNVIDIAInfoFn = func() (string, string, error) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader").CombinedOutput()
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected nvidia-smi output")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

type DashboardModel struct {
	width     int
	height    int
	active    bool
	loading   bool
	loadToken int
	gpuLine   string
	workspace string
	suites    []dashboardSuiteState
	firstRun  bool
	lastErr   error
}

func NewDashboardModel() DashboardModel { return DashboardModel{loading: true} }

func (m *DashboardModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.loadToken++
	token := m.loadToken
	return tea.Batch(loadDashboardCmd(token), dashboardTickCmd(token))
}

func (m *DashboardModel) Deactivate() {
	m.active = false
	m.loadToken++
}

func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			m.loadToken++
			token := m.loadToken
			return m, tea.Batch(loadDashboardCmd(token), dashboardTickCmd(token))
		}
	case dashboardLoadedMsg:
		if msg.token != m.loadToken {
			return m, nil
		}
		m.loading = false
		m.gpuLine = msg.gpuLine
		m.workspace = msg.workspace
		m.suites = msg.suites
		m.firstRun = msg.firstRun
		m.lastErr = msg.loadErr
	case dashboardTickMsg:
		if !m.active || msg.token != m.loadToken {
			return m, nil
		}
		return m, tea.Batch(loadDashboardCmd(msg.token), dashboardTickCmd(msg.token))
	}
	return m, nil
}

func (m DashboardModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading dashboard…")
	}
	if m.lastErr != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render(m.lastErr.Error())
	}
	if m.firstRun {
		return strings.Join([]string{
			lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("gradient linux"),
			"",
			"No suites installed yet.",
			"",
			"Run concave install boosting to get started,",
			"or press 2 to open the Suites view.",
		}, "\n")
	}

	lines := []string{m.gpuLine, ""}
	for _, item := range m.suites {
		lines = append(lines, m.renderSuite(item)...)
	}
	lines = append(lines, "", m.workspace)
	return strings.Join(lines, "\n")
}

func (m DashboardModel) HelpView() string {
	return "Dashboard\nr refresh"
}

func (m DashboardModel) renderSuite(item dashboardSuiteState) []string {
	if !item.Installed {
		return []string{fmt.Sprintf("%s   — not installed", mutedText(item.Name))}
	}

	dot := successText("●")
	status := fmt.Sprintf("%d running", item.Running)
	if item.Running == 0 {
		dot = errorText("○")
		status = "stopped"
	} else if item.Running < item.Total {
		dot = warnText("⚠")
		status = fmt.Sprintf("%d / %d running", item.Running, item.Total)
	}

	line := fmt.Sprintf("%-10s %s %s", item.Name, dot, status)
	if item.Running == item.Total && len(item.Ports) > 0 {
		line += "   " + strings.Join(item.Ports, " · ")
	}

	lines := []string{line}
	if item.Running > 0 && item.Running < item.Total {
		for _, container := range item.Containers {
			icon := successText("✓")
			if container.Status != "running" {
				icon = errorText("✗")
			}
			detail := fmt.Sprintf("  %s %-24s %s", icon, container.Name, container.Status)
			if container.Command != "" {
				detail += "   " + container.Command
			}
			lines = append(lines, detail)
		}
	}
	return lines
}

func loadDashboardCmd(token int) tea.Cmd {
	return func() tea.Msg {
		state, err := loadStateFn()
		if err != nil {
			return dashboardLoadedMsg{token: token, loadErr: err}
		}

		ordered := orderedInstalledSuites(state.Installed)
		if len(ordered) == 0 {
			return dashboardLoadedMsg{
				token:     token,
				gpuLine:   dashboardGPULine(),
				workspace: dashboardWorkspaceLine(),
				firstRun:  true,
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suitesState := make([]dashboardSuiteState, 0, len(viewOrder))
		installedSet := make(map[string]struct{}, len(ordered))
		for _, name := range ordered {
			installedSet[name] = struct{}{}
		}

		for _, name := range viewOrder {
			if _, ok := installedSet[name]; !ok {
				suitesState = append(suitesState, dashboardSuiteState{Name: name})
				continue
			}
			s, err := currentSuiteDefinition(name)
			if err != nil {
				return dashboardLoadedMsg{token: token, loadErr: err}
			}
			item := dashboardSuiteState{
				Name:      name,
				Installed: true,
				Total:     len(s.Containers),
				Ports:     formatSuitePorts(s),
			}
			for _, container := range s.Containers {
				status, statusErr := dockerStatusFn(ctx, container.Name)
				if statusErr != nil {
					status = "error"
				}
				if status == "running" {
					item.Running++
				}
				command := ""
				if status != "running" {
					command = "concave start " + name
				}
				item.Containers = append(item.Containers, dashboardContainerState{
					Name:    container.Name,
					Status:  status,
					Command: command,
				})
			}
			suitesState = append(suitesState, item)
		}

		return dashboardLoadedMsg{
			token:     token,
			gpuLine:   dashboardGPULine(),
			workspace: dashboardWorkspaceLine(),
			suites:    suitesState,
		}
	}
}

func dashboardTickCmd(token int) tea.Cmd {
	return tea.Tick(dashboardRefreshInterval, func(time.Time) tea.Msg {
		return dashboardTickMsg{token: token}
	})
}

func dashboardGPULine() string {
	state, err := gpuDetectFn()
	if err != nil {
		return warnText("GPU") + "  detection error · " + err.Error()
	}

	switch state {
	case gpu.GPUStateNVIDIA:
		name, vram, err := dashboardNVIDIAInfoFn()
		if err != nil {
			return successText("GPU") + "  NVIDIA detected"
		}
		branch, err := gpuRecommendedFn()
		if err != nil {
			branch = "unknown"
		}
		toolkit, _ := gpuToolkitFn()
		toolkitText := "toolkit ✗"
		if toolkit {
			toolkitText = "toolkit ✓"
		}
		return successText("GPU") + "  " + name + " · " + vram + " · driver " + branch + " · " + toolkitText
	case gpu.GPUStateAMD:
		return warnText("GPU") + "  AMD detected"
	default:
		return warnText("GPU") + "  not detected · CPU-only mode"
	}
}

func dashboardWorkspaceLine() string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(workspaceRootFn(), &stat); err != nil {
		return "Workspace  " + workspaceRootFn()
	}
	free := stat.Bavail * uint64(stat.Bsize)
	return fmt.Sprintf("Workspace  %s · %s free", workspaceRootFn(), humanBytes(free))
}

func formatSuitePorts(s suite.Suite) []string {
	ports := make([]string, 0, len(s.Ports))
	for _, mapping := range s.Ports {
		ports = append(ports, fmt.Sprintf("%s :%d", mapping.Service, mapping.Port))
	}
	return ports
}

func humanBytes(value uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(value)
	unit := units[0]
	for idx := 0; idx < len(units)-1 && size >= 1024; idx++ {
		size /= 1024
		unit = units[idx+1]
	}
	return fmt.Sprintf("%.0f %s", size, unit)
}

func successText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(text)
}

func warnText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarn)).Render(text)
}

func errorText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render(text)
}

func mutedText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(text)
}
