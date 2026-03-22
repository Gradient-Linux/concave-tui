package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	workspacepkg "github.com/Gradient-Linux/concave-tui/internal/workspace"
)

const workspaceRefreshInterval = time.Second

type cpuTotals struct {
	total uint64
	idle  uint64
}

type cpuSnapshot struct {
	total cpuTotals
	cores []cpuTotals
}

type workspaceLoadedMsg struct {
	token         int
	used          uint64
	total         uint64
	usages        []workspacepkg.Usage
	root          string
	lastBackup    time.Time
	ramUsedBytes  uint64
	ramTotalBytes uint64
	gpuState      gpu.GPUState
	gpuDevices    []gpu.NVIDIADevice
	cpuUsage      float64
	coreUsage     []float64
	snapshot      cpuSnapshot
	loadErr       error
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

var (
	workspaceReadMemFn         = readMemInfo
	workspaceGPUDetectFn       = gpu.Detect
	workspaceGPUStatsFn        = gpu.NVIDIADevices
	workspaceReadCPUSnapshotFn = readCPUSnapshot
)

type WorkspaceModel struct {
	width          int
	height         int
	active         bool
	loaded         bool
	loading        bool
	confirmClean   bool
	loadToken      int
	cfg            tuiconfig.Config
	root           string
	used           uint64
	total          uint64
	usages         []workspacepkg.Usage
	lastBackup     time.Time
	lastErr        error
	busyMessage    string
	completionNote string

	ramUsedBytes  uint64
	ramTotalBytes uint64
	gpuState      gpu.GPUState
	gpuDevices    []gpu.NVIDIADevice
	cpuUsage      float64
	coreUsage     []float64
	cpuSnapshot   cpuSnapshot
}

func NewWorkspaceModel() WorkspaceModel {
	return WorkspaceModel{
		loading: true,
		cfg:     tuiconfig.DefaultConfig(),
	}
}

func (m *WorkspaceModel) SetConfig(cfg tuiconfig.Config) {
	m.cfg = cfg
}

func (m *WorkspaceModel) Activate() tea.Cmd {
	m.active = true
	m.loadToken++
	token := m.loadToken
	if m.loaded {
		m.loading = false
		return workspaceTickCmd(token, m.refreshInterval())
	}
	m.loading = true
	return tea.Batch(loadWorkspaceCmd(token, m.cpuSnapshot), workspaceTickCmd(token, m.refreshInterval()))
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
			return m, tea.Batch(loadWorkspaceCmd(token, m.cpuSnapshot), workspaceTickCmd(token, m.refreshInterval()))
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
		m.loaded = true
		m.loading = false
		m.root = msg.root
		m.used = msg.used
		m.total = msg.total
		m.usages = msg.usages
		m.lastBackup = msg.lastBackup
		m.ramUsedBytes = msg.ramUsedBytes
		m.ramTotalBytes = msg.ramTotalBytes
		m.gpuState = msg.gpuState
		m.gpuDevices = msg.gpuDevices
		m.cpuUsage = msg.cpuUsage
		m.coreUsage = msg.coreUsage
		m.cpuSnapshot = msg.snapshot
		m.lastErr = msg.loadErr
	case workspaceTickMsg:
		if !m.active || msg.token != m.loadToken {
			return m, nil
		}
		return m, tea.Batch(loadWorkspaceCmd(msg.token, m.cpuSnapshot), workspaceTickCmd(msg.token, m.refreshInterval()))
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
		return m, tea.Batch(loadWorkspaceCmd(token, m.cpuSnapshot), workspaceNoteExpiryCmd())
	case workspaceNoteExpiredMsg:
		m.completionNote = ""
	}
	return m, nil
}

func (m WorkspaceModel) View() string {
	if m.loading && !m.loaded {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading workspace…")
	}
	if m.lastErr != nil && !m.loaded {
		return errorText(m.lastErr.Error())
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render(m.root),
		fmt.Sprintf("Disk  %s  %s free / %s (%.0f%% used)", m.totalBar(), humanBytes(m.total-m.used), humanBytes(m.total), m.usedRatio()*100),
		"",
		m.renderHardwareOverview(),
		"",
		m.renderWorkspaceUsage(),
		"",
		m.renderWorkspaceActions(),
	}
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) HelpView() string {
	return "Workspace\nr refresh · b backup · x clean outputs"
}

func (m WorkspaceModel) refreshInterval() time.Duration {
	if m.cfg.Display.RefreshIntervalMs <= 0 {
		return workspaceRefreshInterval
	}
	return time.Duration(m.cfg.Display.RefreshIntervalMs) * time.Millisecond
}

func (m WorkspaceModel) renderHardwareOverview() string {
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Hardware"),
		m.renderGPUChart(m.width, 0),
		"",
		m.renderCPUChart(m.width, 0),
		"",
		m.renderSystemBars(),
	}
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) renderGPUChart(width, height int) string {
	_ = height
	switch m.gpuState {
	case gpu.GPUStateNone:
		return warnText("GPU not detected · CPU-only mode")
	case gpu.GPUStateAMD:
		return warnText("AMD detected · ROCm metrics not yet available")
	}
	if len(m.gpuDevices) == 0 {
		return mutedText("No NVIDIA telemetry available")
	}
	device := m.gpuDevices[0]
	barWidth := max(18, min(48, width-26))
	return fmt.Sprintf("GPU    %s  %d%%\n%s", truncate(device.Name, max(16, width-16)), device.Utilization, utilizationBar(barWidth, float64(device.Utilization)/100, false))
}

func (m WorkspaceModel) renderCPUChart(width, height int) string {
	_ = height
	barWidth := max(18, min(48, width-26))
	return fmt.Sprintf("CPU    total  %.0f%%\n%s", m.cpuUsage, utilizationBar(barWidth, m.cpuUsage/100, false))
}

func (m WorkspaceModel) renderSystemBars() string {
	lines := []string{
		renderUsageBarLine("RAM", m.ramUsedBytes, m.ramTotalBytes, m.width-20),
	}
	if len(m.gpuDevices) > 0 {
		device := m.gpuDevices[0]
		lines = append(lines, renderUsageBarLine("VRAM", device.MemoryUsedBytes(), device.MemoryTotalBytes(), m.width-20))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("CPU cores"))
	lines = append(lines, m.renderCoreBars()...)
	return strings.Join(lines, "\n")
}

func renderUsageBarLine(label string, used, total uint64, width int) string {
	if total == 0 {
		return fmt.Sprintf("%-5s %s", label, mutedText("data unavailable"))
	}
	ratio := float64(used) / float64(total)
	return fmt.Sprintf("%-5s %s / %s  %s  %.0f%%", label, humanBytes(used), humanBytes(total), utilizationBar(max(14, min(36, width)), ratio, false), ratio*100)
}

func (m WorkspaceModel) renderCoreBars() []string {
	if len(m.coreUsage) == 0 {
		return []string{mutedText("No per-core CPU data available")}
	}

	colCount := 1
	if m.width >= 120 {
		colCount = 2
	}
	maxVisible := max(4, m.height-22)
	if colCount == 2 {
		maxVisible *= 2
	}
	values := m.coreUsage
	truncated := 0
	if len(values) > maxVisible {
		truncated = len(values) - maxVisible
		values = values[:maxVisible]
	}
	colWidth := max(28, (m.width-(colCount-1)*2)/colCount)
	rowsPerCol := (len(values) + colCount - 1) / colCount
	columns := make([][]string, colCount)
	for idx, value := range values {
		column := idx / max(1, rowsPerCol)
		if column >= len(columns) {
			column = len(columns) - 1
		}
		columns[column] = append(columns[column], renderCoreBar(idx, value, colWidth))
	}
	rendered := make([]string, 0, colCount)
	for _, column := range columns {
		if len(column) == 0 {
			continue
		}
		rendered = append(rendered, lipgloss.NewStyle().Width(colWidth).Render(strings.Join(column, "\n")))
	}
	lines := []string{lipgloss.JoinHorizontal(lipgloss.Top, joinColumns(rendered)...)}
	if truncated > 0 {
		lines = append(lines, mutedText(fmt.Sprintf("… %d more cores", truncated)))
	}
	return lines
}

func joinColumns(columns []string) []string {
	if len(columns) <= 1 {
		return columns
	}
	joined := make([]string, 0, len(columns)*2-1)
	for idx, column := range columns {
		if idx > 0 {
			joined = append(joined, "  ")
		}
		joined = append(joined, column)
	}
	return joined
}

func renderCoreBar(index int, value float64, width int) string {
	barWidth := max(10, width-14)
	return fmt.Sprintf("cpu%02d %s %3.0f%%", index, utilizationBar(barWidth, value/100, false), value)
}

func (m WorkspaceModel) renderWorkspaceUsage() string {
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Workspace usage"),
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
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) renderWorkspaceActions() string {
	lines := []string{}
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
	if m.lastErr != nil {
		lines = append(lines, errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m WorkspaceModel) usedRatio() float64 {
	if m.total == 0 {
		return 0
	}
	return float64(m.used) / float64(m.total)
}

func (m WorkspaceModel) totalBar() string {
	width := max(18, min(34, m.width-32))
	ratio := m.usedRatio()
	return gradientBar(width, ratio, true)
}

func (m WorkspaceModel) directoryBar(value, maxValue int64) string {
	if maxValue <= 0 {
		return ""
	}
	width := max(16, min(38, m.width-26))
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

func loadWorkspaceCmd(token int, previous ...cpuSnapshot) tea.Cmd {
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
		ramUsed, ramTotal, _ := workspaceReadMemFn()
		gpuState, _ := workspaceGPUDetectFn()
		gpuDevices := []gpu.NVIDIADevice{}
		if gpuState == gpu.GPUStateNVIDIA {
			if devices, err := workspaceGPUStatsFn(); err == nil {
				gpuDevices = devices
			}
		}
		snapshot, err := workspaceReadCPUSnapshotFn()
		if err != nil {
			return workspaceLoadedMsg{token: token, loadErr: err}
		}
		var prior cpuSnapshot
		if len(previous) > 0 {
			prior = previous[0]
		}
		cpuUsage, coreUsage := cpuUsageFrom(prior, snapshot)

		return workspaceLoadedMsg{
			token:         token,
			root:          root,
			total:         total,
			used:          used,
			usages:        usages,
			lastBackup:    latestBackupTime(filepath.Join(root, "backups")),
			ramUsedBytes:  ramUsed,
			ramTotalBytes: ramTotal,
			gpuState:      gpuState,
			gpuDevices:    gpuDevices,
			cpuUsage:      cpuUsage,
			coreUsage:     coreUsage,
			snapshot:      snapshot,
		}
	}
}

func cpuUsageFrom(previous, current cpuSnapshot) (float64, []float64) {
	total := usageRatio(previous.total, current.total)
	cores := make([]float64, 0, len(current.cores))
	for idx, core := range current.cores {
		if idx < len(previous.cores) {
			cores = append(cores, usageRatio(previous.cores[idx], core))
			continue
		}
		cores = append(cores, 0)
	}
	return total, cores
}

func usageRatio(previous, current cpuTotals) float64 {
	if previous.total == 0 || current.total <= previous.total {
		return 0
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if totalDelta == 0 {
		return 0
	}
	used := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func readCPUSnapshot() (cpuSnapshot, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSnapshot{}, err
	}
	snapshot := cpuSnapshot{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		totals := parseCPUTotals(fields[1:])
		if fields[0] == "cpu" {
			snapshot.total = totals
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu")); err == nil {
			snapshot.cores = append(snapshot.cores, totals)
		}
	}
	return snapshot, nil
}

func parseCPUTotals(fields []string) cpuTotals {
	var values []uint64
	for _, field := range fields {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			continue
		}
		values = append(values, value)
	}
	total := uint64(0)
	for _, value := range values {
		total += value
	}
	idle := uint64(0)
	if len(values) > 3 {
		idle += values[3]
	}
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuTotals{total: total, idle: idle}
}

func workspaceTickCmd(token int, interval ...time.Duration) tea.Cmd {
	delay := workspaceRefreshInterval
	if len(interval) > 0 && interval[0] > 0 {
		delay = interval[0]
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
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
