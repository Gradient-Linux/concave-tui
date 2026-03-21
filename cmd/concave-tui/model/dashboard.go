package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

type Widget interface {
	ID() string
	Title() string
	Render(width, height int, style string) string
	Update(msg tea.Msg) (Widget, tea.Cmd)
}

type dashboardSuiteState struct {
	Name       string
	Installed  bool
	Total      int
	Running    int
	Ports      []string
	Containers []dashboardContainerState
	Warning    string
}

type dashboardContainerState struct {
	Name   string
	Status string
	Image  string
}

type dashboardMetrics struct {
	GPUState      gpu.GPUState
	GPUs          []gpu.NVIDIADevice
	CUDAVersion   string
	RAMUsedBytes  uint64
	RAMTotalBytes uint64
	DiskFreeBytes uint64
	DiskTotalBytes uint64
	DockerOK      bool
	InternetOK    bool
	Suites        []dashboardSuiteState
}

type dashboardLoadedMsg struct {
	token    int
	metrics  dashboardMetrics
	firstRun bool
	loadErr  error
}

type dashboardTickMsg struct {
	token int
}

type dashboardWidget struct {
	id     string
	title  string
	render func(width, height int, style string) string
}

func (w dashboardWidget) ID() string                                    { return w.id }
func (w dashboardWidget) Title() string                                 { return w.title }
func (w dashboardWidget) Render(width, height int, style string) string { return w.render(width, height, style) }
func (w dashboardWidget) Update(msg tea.Msg) (Widget, tea.Cmd)          { return w, nil }

var (
	dashboardGPUStatsFn   = gpu.NVIDIADevices
	dashboardCUDAFn       = gpu.CUDAVersion
	dashboardReadMemFn    = readMemInfo
	dashboardTickNowFn    = time.Now
	dashboardSystemDocker = systemDockerFn
	dashboardInternetFn   = systemInternetFn
)

type DashboardModel struct {
	width     int
	height    int
	active    bool
	loading   bool
	loadToken int
	cfg       tuiconfig.Config
	metrics   dashboardMetrics
	firstRun  bool
	lastErr   error
	history   [][]float64
}

func NewDashboardModel() DashboardModel {
	return DashboardModel{
		loading: true,
		cfg:     tuiconfig.DefaultConfig(),
	}
}

func (m *DashboardModel) SetConfig(cfg tuiconfig.Config) {
	m.cfg = cfg
}

func (m *DashboardModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.loadToken++
	token := m.loadToken
	return tea.Batch(loadDashboardCmd(token), dashboardTickCmd(token, m.refreshInterval()))
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
		switch msg.String() {
		case "r":
			m.loading = true
			m.loadToken++
			token := m.loadToken
			return m, tea.Batch(loadDashboardCmd(token), dashboardTickCmd(token, m.refreshInterval()))
		case "p":
			m.cfg.ActivePreset = nextPresetName(m.cfg)
		}
	case dashboardLoadedMsg:
		if msg.token != m.loadToken {
			return m, nil
		}
		m.loading = false
		m.metrics = msg.metrics
		m.firstRun = msg.firstRun
		m.lastErr = msg.loadErr
		m.appendHistory(msg.metrics.GPUs)
	case dashboardTickMsg:
		if !m.active || msg.token != m.loadToken {
			return m, nil
		}
		return m, tea.Batch(loadDashboardCmd(msg.token), dashboardTickCmd(msg.token, m.refreshInterval()))
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
			"Press 2 to open the Suites view and install your first suite,",
			"or run: concave install boosting",
		}, "\n")
	}

	style := tuiconfig.ResolveGraphStyle(m.cfg, m.width, m.height)
	widgets := m.widgets()
	if len(widgets) == 0 {
		return mutedText("No dashboard widgets configured for the active preset")
	}

	columns := dashboardColumnsForWidth(m.width)
	columnWidth := max(22, (m.width-(columns-1)*2)/columns)
	buckets := make([][]string, columns)
	for idx, widget := range widgets {
		target := idx % columns
		buckets[target] = append(buckets[target], m.renderWidgetCard(widget, columnWidth, style))
	}

	rendered := make([]string, 0, columns)
	for _, items := range buckets {
		rendered = append(rendered, strings.Join(items, "\n\n"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m DashboardModel) HelpView() string {
	return "Dashboard\nr refresh · p cycle presets · j/k move in other views"
}

func (m DashboardModel) refreshInterval() time.Duration {
	if m.cfg.Display.RefreshIntervalMs <= 0 {
		return time.Second
	}
	return time.Duration(m.cfg.Display.RefreshIntervalMs) * time.Millisecond
}

func (m *DashboardModel) appendHistory(gpus []gpu.NVIDIADevice) {
	if len(gpus) == 0 {
		m.history = nil
		return
	}
	if len(m.history) < len(gpus) {
		next := make([][]float64, len(gpus))
		copy(next, m.history)
		m.history = next
	}
	for idx, device := range gpus {
		m.history[idx] = append(m.history[idx], float64(device.Utilization))
		if len(m.history[idx]) > 60 {
			m.history[idx] = m.history[idx][len(m.history[idx])-60:]
		}
	}
}

func (m DashboardModel) widgets() []Widget {
	preset, ok := m.cfg.PresetByName(m.cfg.ActivePreset)
	if !ok {
		preset, _ = m.cfg.PresetByName("default")
	}
	widgets := make([]Widget, 0, len(preset.Widgets))
	for _, id := range preset.Widgets {
		if widget, ok := m.widgetByID(id); ok {
			widgets = append(widgets, widget)
		}
	}
	return widgets
}

func (m DashboardModel) widgetByID(id string) (Widget, bool) {
	switch id {
	case "gpu-graph":
		return dashboardWidget{id: id, title: "GPU Utilization", render: func(width, height int, style string) string {
			return m.renderGPUWidget(0, width, height, style)
		}}, true
	case "gpu-graph-2":
		return dashboardWidget{id: id, title: "GPU 2 Utilization", render: func(width, height int, style string) string {
			return m.renderGPUWidget(1, width, height, style)
		}}, true
	case "vram-bar":
		return dashboardWidget{id: id, title: "VRAM", render: func(width, height int, style string) string {
			return m.renderVRAMWidget(width)
		}}, true
	case "ram-bar":
		return dashboardWidget{id: id, title: "System RAM", render: func(width, height int, style string) string {
			return m.renderRAMWidget(width)
		}}, true
	case "suite-status":
		return dashboardWidget{id: id, title: "Suites", render: func(width, height int, style string) string {
			return m.renderSuiteStatusWidget()
		}}, true
	case "system-health":
		return dashboardWidget{id: id, title: "System", render: func(width, height int, style string) string {
			return m.renderSystemHealthWidget(width)
		}}, true
	case "port-map":
		return dashboardWidget{id: id, title: "Ports", render: func(width, height int, style string) string {
			return m.renderPortMapWidget()
		}}, true
	case "flow-services":
		return dashboardWidget{id: id, title: "Flow Edition", render: func(width, height int, style string) string {
			return m.renderSuiteContainersWidget("flow")
		}}, true
	case "neural-containers":
		return dashboardWidget{id: id, title: "Neural", render: func(width, height int, style string) string {
			return m.renderSuiteContainersWidget("neural")
		}}, true
	default:
		return nil, false
	}
}

func (m DashboardModel) renderWidgetCard(widget Widget, width int, style string) string {
	body := widget.Render(width-4, max(6, m.height/3), style)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render(widget.Title()) + "\n" + body)
}

func (m DashboardModel) renderGPUWidget(index, width, height int, style string) string {
	if m.metrics.GPUState == gpu.GPUStateNone {
		return warnText("GPU not detected · CPU-only mode")
	}
	if m.metrics.GPUState == gpu.GPUStateAMD {
		return warnText("AMD detected · ROCm metrics not yet available")
	}
	if index >= len(m.metrics.GPUs) {
		return mutedText("No secondary GPU detected")
	}

	device := m.metrics.GPUs[index]
	if style == "bar" || len(m.history[index]) == 0 {
		ratio := float64(device.Utilization) / 100
		return fmt.Sprintf(
			"%s   %d%%\n%s",
			device.Name,
			device.Utilization,
			gradientBar(max(16, width-4), ratio, false),
		)
	}

	chartWidth := max(20, width-2)
	chartHeight := max(8, min(height, 12))
	now := dashboardTickNowFn()
	start := now.Add(-time.Duration(len(m.history[index])-1) * time.Second)
	chart := timeserieslinechart.New(chartWidth, chartHeight)
	chart.SetViewTimeAndYRange(start, now, 0, 100)
	chart.SetStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)))
	for offset, value := range m.history[index] {
		chart.Push(timeserieslinechart.TimePoint{
			Time:  now.Add(-time.Duration(len(m.history[index])-1-offset) * time.Second),
			Value: value,
		})
	}
	chart.DrawBraille()
	return fmt.Sprintf("%s · %d%%\n%s", device.Name, device.Utilization, chart.View())
}

func (m DashboardModel) renderVRAMWidget(width int) string {
	if len(m.metrics.GPUs) == 0 {
		return mutedText("No GPU memory data available")
	}
	device := m.metrics.GPUs[0]
	ratio := device.MemoryRatio()
	return fmt.Sprintf(
		"%s / %s   %s   %.0f%%",
		humanBytes(device.MemoryUsedBytes()),
		humanBytes(device.MemoryTotalBytes()),
		gradientBar(max(18, width-24), ratio, true),
		ratio*100,
	)
}

func (m DashboardModel) renderRAMWidget(width int) string {
	if m.metrics.RAMTotalBytes == 0 {
		return mutedText("RAM data unavailable")
	}
	ratio := float64(m.metrics.RAMUsedBytes) / float64(m.metrics.RAMTotalBytes)
	return fmt.Sprintf(
		"%s / %s   %s   %.0f%%",
		humanBytes(m.metrics.RAMUsedBytes),
		humanBytes(m.metrics.RAMTotalBytes),
		gradientBar(max(18, width-24), ratio, true),
		ratio*100,
	)
}

func (m DashboardModel) renderSuiteStatusWidget() string {
	lines := make([]string, 0, len(m.metrics.Suites))
	for _, item := range m.metrics.Suites {
		if !item.Installed {
			lines = append(lines, fmt.Sprintf("%-10s %s not installed", item.Name, mutedText("—")))
			continue
		}
		if item.Warning != "" {
			lines = append(lines, fmt.Sprintf("%-10s %s %s", item.Name, warnText("⚠"), item.Warning))
			lines = append(lines, "  remove and reinstall forge to restore its saved component set")
			continue
		}
		icon := successText("●")
		state := fmt.Sprintf("%d / %d", item.Running, item.Total)
		if item.Running == 0 {
			icon = errorText("○")
			state = "stopped"
		} else if item.Running < item.Total {
			icon = warnText("⚠")
			state = fmt.Sprintf("%d / %d", item.Running, item.Total)
		}
		line := fmt.Sprintf("%-10s %s %s", item.Name, icon, state)
		if len(item.Ports) > 0 {
			line += "   " + strings.Join(item.Ports, "  ")
		}
		lines = append(lines, line)
		if item.Running > 0 && item.Running < item.Total {
			for _, container := range item.Containers {
				if container.Status == "running" {
					continue
				}
				lines = append(lines, fmt.Sprintf("  %s %-24s stopped   concave start %s", errorText("✗"), container.Name, item.Name))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (m DashboardModel) renderSystemHealthWidget(width int) string {
	dockerState := successText("running")
	if !m.metrics.DockerOK {
		dockerState = errorText("down")
	}
	internetState := successText("reachable")
	if !m.metrics.InternetOK {
		internetState = warnText("offline")
	}
	freeRatio := 0.0
	if m.metrics.DiskTotalBytes > 0 {
		freeRatio = float64(m.metrics.DiskFreeBytes) / float64(m.metrics.DiskTotalBytes)
	}
	freeText := humanBytes(m.metrics.DiskFreeBytes) + " free"
	if freeRatio < 0.10 {
		freeText = errorText(freeText)
	} else if freeRatio < 0.20 {
		freeText = warnText(freeText)
	}
	return strings.Join([]string{
		"Docker    " + dockerState,
		"Internet  " + internetState,
		"Disk      " + freeText,
		"Workspace " + truncate(workspaceRootFn(), max(16, width-14)),
	}, "\n")
}

func (m DashboardModel) renderPortMapWidget() string {
	lines := []string{}
	for _, suiteState := range m.metrics.Suites {
		if !suiteState.Installed || len(suiteState.Ports) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-10s %s", suiteState.Name, strings.Join(suiteState.Ports, " · ")))
	}
	if len(lines) == 0 {
		return mutedText("No active ports")
	}
	return strings.Join(lines, "\n")
}

func (m DashboardModel) renderSuiteContainersWidget(name string) string {
	for _, suiteState := range m.metrics.Suites {
		if suiteState.Name != name {
			continue
		}
		if !suiteState.Installed {
			return mutedText("Suite not installed")
		}
		lines := make([]string, 0, len(suiteState.Containers))
		for _, container := range suiteState.Containers {
			state := successText("running")
			if container.Status != "running" {
				state = errorText(container.Status)
			}
			lines = append(lines, fmt.Sprintf("%-24s %s", container.Name, state))
		}
		return strings.Join(lines, "\n")
	}
	return mutedText("Suite not installed")
}

func loadDashboardCmd(token int) tea.Cmd {
	return func() tea.Msg {
		state, err := loadStateFn()
		if err != nil {
			return dashboardLoadedMsg{token: token, loadErr: err}
		}

		ordered := orderedInstalledSuites(state.Installed)
		metrics, err := collectDashboardMetrics(ordered)
		if err != nil {
			return dashboardLoadedMsg{token: token, loadErr: err}
		}

		return dashboardLoadedMsg{
			token:    token,
			metrics:  metrics,
			firstRun: len(ordered) == 0,
		}
	}
}

func collectDashboardMetrics(ordered []string) (dashboardMetrics, error) {
	metrics := dashboardMetrics{}

	if ok, err := dashboardSystemDocker(); err == nil {
		metrics.DockerOK = ok
	}
	if ok, err := dashboardInternetFn(); err == nil {
		metrics.InternetOK = ok
	}
	if used, total, err := dashboardReadMemFn(); err == nil {
		metrics.RAMUsedBytes = used
		metrics.RAMTotalBytes = total
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(workspaceRootFn(), &stat); err == nil {
		metrics.DiskTotalBytes = stat.Blocks * uint64(stat.Bsize)
		metrics.DiskFreeBytes = stat.Bavail * uint64(stat.Bsize)
	}

	state, err := gpuDetectFn()
	if err == nil {
		metrics.GPUState = state
	}
	if state == gpu.GPUStateNVIDIA {
		if devices, err := dashboardGPUStatsFn(); err == nil {
			metrics.GPUs = devices
		}
		if cudaVersion, err := dashboardCUDAFn(); err == nil {
			metrics.CUDAVersion = cudaVersion
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	installedSet := make(map[string]struct{}, len(ordered))
	for _, name := range ordered {
		installedSet[name] = struct{}{}
	}
	for _, name := range viewOrder {
		if _, ok := installedSet[name]; !ok {
			metrics.Suites = append(metrics.Suites, dashboardSuiteState{Name: name})
			continue
		}
		s, err := currentSuiteDefinition(name)
		if err != nil {
			if isMissingForgeSelection(err) && name == "forge" {
				metrics.Suites = append(metrics.Suites, dashboardSuiteState{
					Name:      name,
					Installed: true,
					Warning:   "installed but not configured",
				})
				continue
			}
			return dashboardMetrics{}, err
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
			item.Containers = append(item.Containers, dashboardContainerState{
				Name:   container.Name,
				Status: status,
				Image:  container.Image,
			})
		}
		metrics.Suites = append(metrics.Suites, item)
	}

	return metrics, nil
}

func dashboardTickCmd(token int, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return dashboardTickMsg{token: token}
	})
}

func dashboardColumnsForWidth(width int) int {
	switch {
	case width >= 160:
		return 3
	case width >= 80:
		return 2
	default:
		return 1
	}
}

func formatSuitePorts(s suite.Suite) []string {
	ports := make([]string, 0, len(s.Ports))
	for _, mapping := range s.Ports {
		ports = append(ports, fmt.Sprintf(":%d", mapping.Port))
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
	if unit == "B" {
		return fmt.Sprintf("%.0f %s", size, unit)
	}
	return fmt.Sprintf("%.1f %s", size, unit)
}

func readMemInfo() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	var totalKB, availableKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			fmt.Sscan(fields[1], &totalKB)
		case "MemAvailable:":
			fmt.Sscan(fields[1], &availableKB)
		}
	}
	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	total := totalKB * 1024
	available := availableKB * 1024
	return total - available, total, nil
}
