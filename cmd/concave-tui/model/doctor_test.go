package model

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	cfgstore "github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
)

func TestDoctorUpdateRerunResetsChecks(t *testing.T) {
	m := NewDoctorModel()
	m.checks[0] = doctorCheck{name: "Docker", status: "pass", detail: "running"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !updated.checks[0].pending {
		t.Fatal("expected checks to reset to pending")
	}
}

func TestDoctorViewShowsRecovery(t *testing.T) {
	m := NewDoctorModel()
	m.width = 40
	m.checks = []doctorCheck{
		{name: "neural", status: "warn", detail: "1 / 3 containers running", recovery: "└─ gradient-neural-infer stopped · run: concave start neural"},
	}
	m.checkedAt = time.Now()

	view := m.View()
	if !strings.Contains(view, "concave") || !strings.Contains(view, "start neural") {
		t.Fatalf("View() = %q", view)
	}
	if !strings.Contains(view, "neural") {
		t.Fatalf("expected name column in view, got %q", view)
	}
	if strings.Contains(view, "neu\nral") || strings.Contains(view, "neura\nl") {
		t.Fatalf("name should not wrap into detail column, got %q", view)
	}
}

func TestDoctorGPUCheckVariants(t *testing.T) {
	restoreModelDeps(t)

	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	if check := doctorGPUCheck(); check.status != "warn" || !strings.Contains(check.detail, "CPU-only mode") {
		t.Fatalf("doctorGPUCheck() = %#v", check)
	}

	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNVIDIA, nil }
	doctorGPUStatsFn = func() ([]gpu.NVIDIADevice, error) {
		return []gpu.NVIDIADevice{{Name: "RTX 4090", DriverVersion: "570.12"}}, nil
	}
	doctorCUDAFn = func() (string, error) { return "12.4", nil }
	gpuToolkitFn = func() (bool, error) { return true, nil }
	if check := doctorGPUCheck(); check.status != "pass" || !strings.Contains(check.detail, "CUDA 12.4") {
		t.Fatalf("doctorGPUCheck() = %#v", check)
	}

	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateAMD, nil }
	if check := doctorGPUCheck(); check.status != "warn" {
		t.Fatalf("doctorGPUCheck() = %#v", check)
	}
}

func TestDoctorSuiteCheckPartialFailure(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	dockerStatusFn = func(ctx context.Context, name string) (string, error) {
		if name == "gradient-neural-infer" {
			return "stopped", nil
		}
		return "running", nil
	}

	check := doctorSuiteCheck("neural")
	if check.status != "warn" {
		t.Fatalf("doctorSuiteCheck() status = %q", check.status)
	}
	if !strings.Contains(check.recovery, "concave start neural") {
		t.Fatalf("doctorSuiteCheck() recovery = %q", check.recovery)
	}
}

func TestDoctorSuiteCheckUnconfiguredForge(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	loadManifestFn = func() (cfgstore.VersionManifest, error) { return cfgstore.VersionManifest{}, nil }

	check := doctorSuiteCheck("forge")
	if check.status != "warn" {
		t.Fatalf("doctorSuiteCheck() status = %q", check.status)
	}
	if !strings.Contains(check.recovery, "reinstall forge") {
		t.Fatalf("doctorSuiteCheck() recovery = %q", check.recovery)
	}
}

func TestDoctorCheckResultUpdatesIndependently(t *testing.T) {
	m := NewDoctorModel()
	m.runToken = 2

	updated, _ := m.Update(doctorCheckMsg{
		token: 2,
		check: doctorCheck{name: "Docker", status: "pass", detail: "running"},
	})

	if updated.checks[0].status != "pass" {
		t.Fatalf("check status = %q", updated.checks[0].status)
	}
	if updated.checks[1].pending != true {
		t.Fatal("expected other checks to remain pending")
	}
}

func TestDoctorActivateHelpersAndWorkspaceCheck(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	workspaceRootFn = func() string { return tmp }
	workspaceEnsureFn = func() error { return nil }
	if err := os.MkdirAll(tmp+"/data", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, dir := range []string{"notebooks", "models", "outputs", "mlruns", "dags", "compose", "config", "backups"} {
		if err := os.MkdirAll(tmp+"/"+dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
	}

	m := NewDoctorModel()
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	if m.HelpView() == "" {
		t.Fatal("expected help text")
	}
	if check := doctorWorkspaceCheck(); check.status != "pass" {
		t.Fatalf("doctorWorkspaceCheck() = %#v", check)
	}
	if msg := runDoctorCheckCmd(1, func() doctorCheck { return doctorCheck{name: "Docker", status: "pass"} })(); msg == nil {
		t.Fatal("expected doctor check message")
	}
	if checks := doctorChecksTemplate(); len(checks) != 5+len(viewOrder) {
		t.Fatalf("doctorChecksTemplate() len = %d", len(checks))
	}
	m.Deactivate()
	if m.active {
		t.Fatal("expected deactivate to clear active flag")
	}
	if relativeCheckTime(time.Now()) == "" {
		t.Fatal("expected relative time output")
	}
}

func TestDoctorRunChecksProducesBatch(t *testing.T) {
	restoreModelDeps(t)
	systemDockerFn = func() (bool, error) { return true, nil }
	systemGroupFn = func() (bool, error) { return true, nil }
	systemInternetFn = func() (bool, error) { return true, nil }
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	workspaceEnsureFn = func() error { return nil }
	workspaceRootFn = t.TempDir
	isInstalledFn = func(name string) (bool, error) { return false, nil }

	m := NewDoctorModel()
	cmd := m.runChecks()
	if cmd == nil {
		t.Fatal("expected runChecks cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("runChecks() msg = %T, want tea.BatchMsg", msg)
	}
	if len(batch) != 5+len(viewOrder) {
		t.Fatalf("batch len = %d", len(batch))
	}
	for _, checkCmd := range batch {
		if checkCmd == nil {
			continue
		}
		if checkCmd() == nil {
			t.Fatal("expected each doctor check command to emit a message")
		}
	}
}
