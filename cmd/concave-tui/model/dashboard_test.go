package model

import (
	"context"
	"strings"
	"testing"
	"time"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	cfgstore "github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
)

func TestDashboardViewFirstRun(t *testing.T) {
	m := NewDashboardModel()
	m.loading = false
	m.firstRun = true

	view := m.View()
	if !strings.Contains(view, "No suites installed yet.") {
		t.Fatalf("View() = %q", view)
	}
}

func TestDashboardViewDegradedSuiteShowsRecovery(t *testing.T) {
	m := NewDashboardModel()
	m.loading = false
	m.width = 120
	m.height = 40
	m.cfg = tuiconfig.DefaultConfig()
	m.metrics = dashboardMetrics{
		GPUState: gpu.GPUStateNone,
		Suites: []dashboardSuiteState{
			{
				Name:      "neural",
				Installed: true,
				Total:     3,
				Running:   2,
				Containers: []dashboardContainerState{
					{Name: "gradient-neural-torch", Status: "running"},
					{Name: "gradient-neural-lab", Status: "running"},
					{Name: "gradient-neural-infer", Status: "stopped"},
				},
			},
		},
	}

	view := m.View()
	if !strings.Contains(view, "gradient-neural-infer") || !strings.Contains(view, "stopped") {
		t.Fatalf("View() = %q", view)
	}
}

func TestDashboardPresetMappingAndColumns(t *testing.T) {
	m := NewDashboardModel()
	m.SetConfig(tuiconfig.DefaultConfig())
	if len(m.widgets()) == 0 {
		t.Fatal("expected default preset widgets")
	}
	if dashboardColumnsForWidth(79) != 1 || dashboardColumnsForWidth(80) != 2 || dashboardColumnsForWidth(159) != 2 || dashboardColumnsForWidth(160) != 3 {
		t.Fatal("unexpected dashboard column mapping")
	}
	m.SetPreset("mlops")
	if m.cfg.ActivePreset != "mlops" {
		t.Fatalf("active preset = %q", m.cfg.ActivePreset)
	}
}

func TestDashboardHistoryCapsAt60AndBarFallback(t *testing.T) {
	m := NewDashboardModel()
	for idx := 0; idx < 70; idx++ {
		m.appendHistory([]gpu.NVIDIADevice{{Index: 0, Name: "RTX", Utilization: idx % 100}})
	}
	if len(m.history) != 1 || len(m.history[0]) != 60 {
		t.Fatalf("history len = %#v", m.history)
	}

	m.loading = false
	m.width = 100
	m.height = 30
	m.metrics = dashboardMetrics{
		GPUState: gpu.GPUStateNVIDIA,
		GPUs:     []gpu.NVIDIADevice{{Index: 0, Name: "RTX", Utilization: 67, MemoryUsedMiB: 10, MemoryTotalMiB: 20}},
	}
	if got := m.renderGPUWidget(0, 40, 10, "bar"); !strings.Contains(got, "67%") {
		t.Fatalf("renderGPUWidget() = %q", got)
	}
}

func TestDashboardVRAMThresholdColorAndCPUOnly(t *testing.T) {
	m := NewDashboardModel()
	m.metrics = dashboardMetrics{
		GPUState: gpu.GPUStateNVIDIA,
		GPUs:     []gpu.NVIDIADevice{{Name: "RTX", Utilization: 90, MemoryUsedMiB: 96, MemoryTotalMiB: 100}},
	}
	if got := m.renderVRAMWidget(50); !strings.Contains(got, "96%") {
		t.Fatalf("renderVRAMWidget() = %q", got)
	}

	m.metrics.GPUState = gpu.GPUStateNone
	if got := m.renderGPUWidget(0, 40, 10, "bar"); !strings.Contains(got, "CPU-only mode") {
		t.Fatalf("renderGPUWidget() = %q", got)
	}
}

func TestLoadDashboardCmdBuildsInstalledSuiteState(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	loadStateFn = func() (cfgstore.State, error) {
		return cfgstore.State{Installed: []string{"boosting"}}, nil
	}
	workspaceRootFn = func() string { return tmp }
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	dashboardReadMemFn = func() (uint64, uint64, error) { return 1024, 2048, nil }
	dashboardSystemDocker = func() (bool, error) { return true, nil }
	dashboardInternetFn = func() (bool, error) { return true, nil }
	dockerStatusFn = func(ctx context.Context, name string) (string, error) {
		switch name {
		case "gradient-boost-track":
			return "stopped", nil
		default:
			return "running", nil
		}
	}
	dashboardTickNowFn = func() time.Time { return time.Unix(100, 0) }

	msg := loadDashboardCmd(3)().(dashboardLoadedMsg)
	if msg.loadErr != nil {
		t.Fatalf("loadDashboardCmd() error = %v", msg.loadErr)
	}
	if len(msg.metrics.Suites) != len(viewOrder) {
		t.Fatalf("suite count = %d, want %d", len(msg.metrics.Suites), len(viewOrder))
	}
	if msg.metrics.Suites[0].Name != "boosting" || msg.metrics.Suites[0].Running != 2 {
		t.Fatalf("boosting state = %#v", msg.metrics.Suites[0])
	}
}

func TestDashboardTickAndHelpers(t *testing.T) {
	m := NewDashboardModel()
	m.SetConfig(tuiconfig.DefaultConfig())
	if dashboardTickCmd(2, time.Second)() == nil {
		t.Fatal("expected tick message")
	}
	if m.HelpView() == "" {
		t.Fatal("expected help text")
	}
	if humanBytes(1024) == "" {
		t.Fatal("expected dashboard helpers")
	}
}

func TestDashboardLayoutAssignsHeightsAndActivationKeepsSnapshot(t *testing.T) {
	m := NewDashboardModel()
	m.SetConfig(tuiconfig.DefaultConfig())
	m.loading = false
	m.loaded = true
	m.width = 120
	m.height = 24
	m.metrics = dashboardMetrics{
		GPUState: gpu.GPUStateNVIDIA,
		GPUs:     []gpu.NVIDIADevice{{Index: 0, Name: "RTX", Utilization: 42, MemoryUsedMiB: 8, MemoryTotalMiB: 16}},
		Suites:   []dashboardSuiteState{{Name: "boosting", Installed: true, Total: 3, Running: 3}},
	}

	layout := m.layoutWidgets(m.widgets(), m.width, m.height, "bar")
	if len(layout) == 0 {
		t.Fatal("expected widget layout")
	}
	for _, column := range layout {
		for _, item := range column {
			if item.height < 4 {
				t.Fatalf("widget height = %d, want at least 4", item.height)
			}
		}
	}
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected tick command on activate")
	}
	if m.loading {
		t.Fatal("expected activate to preserve existing snapshot")
	}
}
