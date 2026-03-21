package model

import (
	"context"
	"strings"
	"testing"

	"github.com/Gradient-Linux/concave-tui/internal/config"
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
	m.gpuLine = "GPU  not detected · CPU-only mode"
	m.workspace = "Workspace  ~/gradient/"
	m.suites = []dashboardSuiteState{
		{
			Name:      "neural",
			Installed: true,
			Total:     3,
			Running:   2,
			Containers: []dashboardContainerState{
				{Name: "gradient-neural-torch", Status: "running"},
				{Name: "gradient-neural-lab", Status: "running"},
				{Name: "gradient-neural-infer", Status: "stopped", Command: "concave start neural"},
			},
		},
	}

	view := m.View()
	if !strings.Contains(view, "concave start neural") {
		t.Fatalf("View() = %q", view)
	}
}

func TestDashboardGPULineCPUOnly(t *testing.T) {
	restoreModelDeps(t)
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }

	if got := dashboardGPULine(); !strings.Contains(got, "CPU-only mode") {
		t.Fatalf("dashboardGPULine() = %q", got)
	}
}

func TestDashboardGPUVariantsAndHelpers(t *testing.T) {
	restoreModelDeps(t)

	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNVIDIA, nil }
	dashboardNVIDIAInfoFn = func() (string, string, error) { return "RTX 4090", "24 GB", nil }
	gpuRecommendedFn = func() (string, error) { return "570", nil }
	gpuToolkitFn = func() (bool, error) { return true, nil }
	if got := dashboardGPULine(); !strings.Contains(got, "RTX 4090") {
		t.Fatalf("dashboardGPULine() = %q", got)
	}

	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateAMD, nil }
	if got := dashboardGPULine(); !strings.Contains(got, "AMD detected") {
		t.Fatalf("dashboardGPULine() = %q", got)
	}
	if dashboardTickCmd(2)() == nil {
		t.Fatal("expected tick message")
	}
}

func TestDashboardActivateUpdateAndHelp(t *testing.T) {
	m := NewDashboardModel()
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	m.SetSize(90, 20)

	updated, _ := m.Update(dashboardLoadedMsg{
		token:     m.loadToken,
		gpuLine:   "GPU",
		workspace: "Workspace",
		suites:    []dashboardSuiteState{{Name: "boosting", Installed: true, Total: 1, Running: 1}},
	})
	if updated.HelpView() == "" {
		t.Fatal("expected help text")
	}

	updated, _ = updated.Update(keyRunes("r"))
	if !updated.loading {
		t.Fatal("expected refresh to set loading")
	}
	updated.Deactivate()
	if updated.active {
		t.Fatal("expected deactivate to clear active flag")
	}
}

func TestLoadDashboardCmdBuildsInstalledSuiteState(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	loadStateFn = func() (config.State, error) {
		return config.State{Installed: []string{"boosting"}}, nil
	}
	workspaceRootFn = func() string { return tmp }
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	dockerStatusFn = func(ctx context.Context, name string) (string, error) {
		switch name {
		case "gradient-boost-track":
			return "stopped", nil
		default:
			return "running", nil
		}
	}

	msg := loadDashboardCmd(3)().(dashboardLoadedMsg)
	if msg.loadErr != nil {
		t.Fatalf("loadDashboardCmd() error = %v", msg.loadErr)
	}
	if len(msg.suites) != len(viewOrder) {
		t.Fatalf("suite count = %d, want %d", len(msg.suites), len(viewOrder))
	}
	if msg.suites[0].Name != "boosting" || msg.suites[0].Running != 2 {
		t.Fatalf("boosting state = %#v", msg.suites[0])
	}
}
