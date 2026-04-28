package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	"github.com/Gradient-Linux/concave-tui/internal/monitoring"
)

const monitoringRefreshInterval = 15 * time.Second

type monitoringProbe struct {
	Reachability monitoring.Reachability
	Samples      map[string]monitoring.Sample
}

type monitoringLoadedMsg struct {
	token int
	probe monitoringProbe
	err   error
}

type monitoringTickMsg struct {
	token int
}

type monitoringMetric struct {
	id    string
	label string
	query string
	unit  string
}

var monitoringMetrics = []monitoringMetric{
	{id: "up", label: "Scrape targets up", query: "count(up == 1)", unit: "count"},
	{id: "cpu", label: "CPU busy", query: `100 - (avg by() (rate(node_cpu_seconds_total{mode="idle"}[1m])) * 100)`, unit: "percent"},
	{id: "mem", label: "Memory available", query: `node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100`, unit: "percent"},
	{id: "fs", label: "Root FS free", query: `node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"} * 100`, unit: "percent"},
	{id: "gpu", label: "GPU utilisation", query: "avg(DCGM_FI_DEV_GPU_UTIL)", unit: "percent"},
}

// MonitoringModel renders Prometheus + Grafana reachability and a small
// snapshot of PromQL samples. It mirrors concave-web's MonitoringView.
type MonitoringModel struct {
	width   int
	height  int
	active  bool
	role    tuiauth.Role
	token   int
	loading bool
	cfg     monitoringSettings
	probe   monitoringProbe
	lastErr error
}

type monitoringSettings struct {
	PrometheusURL string
	GrafanaURL    string
}

// NewMonitoringModel builds an unconfigured monitoring model.
func NewMonitoringModel() MonitoringModel {
	return MonitoringModel{}
}

// SetRole records the active viewer role; Monitoring requires Viewer or above.
func (m *MonitoringModel) SetRole(role tuiauth.Role) { m.role = role }

// SetSize resizes the content area.
func (m *MonitoringModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetConfig updates the upstream URLs without triggering a refresh.
func (m *MonitoringModel) SetConfig(prometheusURL, grafanaURL string) {
	m.cfg = monitoringSettings{
		PrometheusURL: strings.TrimSpace(prometheusURL),
		GrafanaURL:    strings.TrimSpace(grafanaURL),
	}
}

// Activate starts the refresh loop.
func (m *MonitoringModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	m.token++
	token := m.token
	return tea.Batch(loadMonitoringCmd(token, m.cfg), monitoringTickCmd(token, monitoringRefreshInterval))
}

// Deactivate stops refresh ticks.
func (m *MonitoringModel) Deactivate() {
	m.active = false
	m.token++
}

// Update processes Bubble Tea messages for the monitoring view.
func (m MonitoringModel) Update(msg tea.Msg) (MonitoringModel, tea.Cmd) {
	if m.role < tuiauth.RoleViewer {
		return m, nil
	}
	switch msg := msg.(type) {
	case monitoringLoadedMsg:
		if msg.token != m.token {
			return m, nil
		}
		m.loading = false
		m.probe = msg.probe
		m.lastErr = msg.err
	case monitoringTickMsg:
		if m.active && msg.token == m.token {
			return m, tea.Batch(loadMonitoringCmd(msg.token, m.cfg), monitoringTickCmd(msg.token, monitoringRefreshInterval))
		}
	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			m.token++
			token := m.token
			return m, tea.Batch(loadMonitoringCmd(token, m.cfg), monitoringTickCmd(token, monitoringRefreshInterval))
		}
	}
	return m, nil
}

// View renders the monitoring panel.
func (m MonitoringModel) View() string {
	if m.role < tuiauth.RoleViewer {
		return mutedText("Monitoring view is available to Viewer and above")
	}
	if m.cfg.PrometheusURL == "" && m.cfg.GrafanaURL == "" {
		return strings.Join([]string{
			"Monitoring",
			"",
			warnText("No monitoring URLs configured"),
			"",
			mutedText("Set prometheus_url and grafana_url under [monitoring] in ~/.config/concave-tui/concave-tui.toml."),
			mutedText("The Flow suite ships Prometheus on :9090 and Grafana on :3000; the Forge suite makes them optional."),
		}, "\n")
	}
	if m.loading && m.probe.Samples == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading monitoring…")
	}

	lines := []string{
		"Monitoring                               [r] refresh",
		"",
		m.renderReachability(),
		"",
	}
	lines = append(lines, "PromQL snapshot")
	lines = append(lines, m.renderSamples()...)
	if m.lastErr != nil {
		lines = append(lines, "", errorText(m.lastErr.Error()))
	}
	return strings.Join(lines, "\n")
}

// HelpView returns a short key legend for the help overlay.
func (m MonitoringModel) HelpView() string {
	return "Monitoring\nr refresh"
}

func (m MonitoringModel) renderReachability() string {
	prom := formatReachability("Prometheus", m.cfg.PrometheusURL, m.probe.Reachability.Prometheus)
	graf := formatReachability("Grafana", m.cfg.GrafanaURL, m.probe.Reachability.Grafana)
	return strings.Join([]string{prom, graf}, "\n")
}

func (m MonitoringModel) renderSamples() []string {
	if m.probe.Samples == nil {
		return []string{mutedText("No samples yet")}
	}
	lines := make([]string, 0, len(monitoringMetrics))
	for _, metric := range monitoringMetrics {
		sample, ok := m.probe.Samples[metric.id]
		switch {
		case !ok:
			lines = append(lines, fmt.Sprintf("%-20s %s", metric.label, mutedText("-")))
		case sample.Error != "":
			lines = append(lines, fmt.Sprintf("%-20s %s", metric.label, warnText(sample.Error)))
		default:
			lines = append(lines, fmt.Sprintf("%-20s %s", metric.label, successText(formatMonitoringValue(metric.unit, sample.Value))))
		}
	}
	return lines
}

func formatReachability(label, rawURL string, status monitoring.ServiceStatus) string {
	if !status.Configured {
		return fmt.Sprintf("%s: %s", label, mutedText("not configured"))
	}
	target := fallbackText(rawURL, "unknown")
	if status.Reachable {
		version := fallbackText(status.Version, "unknown version")
		return fmt.Sprintf("%s: %s  %s  %s", label, successText("reachable"), mutedText(target), mutedText(version))
	}
	reason := fallbackText(status.Error, "unreachable")
	return fmt.Sprintf("%s: %s  %s  %s", label, warnText("unreachable"), mutedText(target), mutedText(reason))
}

func formatMonitoringValue(unit string, value float64) string {
	switch unit {
	case "percent":
		return fmt.Sprintf("%.1f%%", value)
	case "count":
		return fmt.Sprintf("%.0f", value)
	default:
		return fmt.Sprintf("%.3f", value)
	}
}

var newMonitoringProberFn = func(cfg monitoringSettings) monitoringProber {
	return monitoring.NewProber(cfg.PrometheusURL, cfg.GrafanaURL)
}

type monitoringProber interface {
	Probe(ctx context.Context) monitoring.Reachability
	Query(ctx context.Context, expr string) monitoring.Sample
}

func loadMonitoringCmd(token int, cfg monitoringSettings) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		prober := newMonitoringProberFn(cfg)
		probe := monitoringProbe{
			Reachability: prober.Probe(ctx),
			Samples:      make(map[string]monitoring.Sample, len(monitoringMetrics)),
		}
		if probe.Reachability.Prometheus.Reachable {
			for _, metric := range monitoringMetrics {
				probe.Samples[metric.id] = prober.Query(ctx, metric.query)
			}
		} else {
			for _, metric := range monitoringMetrics {
				probe.Samples[metric.id] = monitoring.Sample{Query: metric.query, Error: "Prometheus unreachable"}
			}
		}
		return monitoringLoadedMsg{token: token, probe: probe}
	}
}

func monitoringTickCmd(token int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg { return monitoringTickMsg{token: token} })
}
