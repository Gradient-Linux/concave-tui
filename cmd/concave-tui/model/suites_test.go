package model

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	cfgstore "github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

func TestPerformInstallSuccess(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return false, nil }
	loadStateFn = func() (cfgstore.State, error) { return cfgstore.State{Installed: nil}, nil }
	loadManifestFn = func() (cfgstore.VersionManifest, error) { return cfgstore.VersionManifest{}, nil }
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }

	var pulled []string
	dockerTagPreviousFn = func(image string) error { return nil }
	dockerPullStreamFn = func(ctx context.Context, image string, cb func(string)) error {
		pulled = append(pulled, image)
		if cb != nil {
			cb("pulling " + image)
		}
		return nil
	}

	composeWritten := false
	dockerWriteComposeFn = func(name string) (string, error) {
		composeWritten = true
		return "/tmp/" + name + ".compose.yml", nil
	}

	saved := cfgstore.VersionManifest{}
	saveManifestFn = func(manifest cfgstore.VersionManifest) error {
		saved = manifest
		return nil
	}

	added := ""
	addSuiteFn = func(name string) error {
		added = name
		return nil
	}

	var progress []string
	if err := performInstall("boosting", func(line string) { progress = append(progress, line) }); err != nil {
		t.Fatalf("performInstall() error = %v", err)
	}
	if !composeWritten {
		t.Fatal("expected compose file to be written")
	}
	if added != "boosting" {
		t.Fatalf("added suite = %q", added)
	}
	if len(pulled) != len(suite.Registry["boosting"].Containers) {
		t.Fatalf("pulled %d images, want %d", len(pulled), len(suite.Registry["boosting"].Containers))
	}
	if len(saved["boosting"]) == 0 {
		t.Fatalf("manifest not updated: %#v", saved)
	}
	if len(progress) == 0 {
		t.Fatal("expected streamed progress")
	}
}

func TestPerformInstallStopsBeforeComposeOnPullFailure(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return false, nil }
	loadStateFn = func() (cfgstore.State, error) { return cfgstore.State{}, nil }
	dockerTagPreviousFn = func(image string) error { return nil }
	dockerPullStreamFn = func(ctx context.Context, image string, cb func(string)) error {
		return errors.New("pull failed")
	}

	composeWritten := false
	dockerWriteComposeFn = func(name string) (string, error) {
		composeWritten = true
		return "", nil
	}

	err := performInstall("boosting", func(string) {})
	if err == nil {
		t.Fatal("expected install error")
	}
	if composeWritten {
		t.Fatal("compose should not be written on pull failure")
	}
}

func TestLoadSuitesCmdReflectsInstalledState(t *testing.T) {
	restoreModelDeps(t)

	loadStateFn = func() (cfgstore.State, error) {
		return cfgstore.State{Installed: []string{"boosting"}}, nil
	}
	loadManifestFn = func() (cfgstore.VersionManifest, error) {
		return cfgstore.VersionManifest{
			"boosting": {
				"gradient-boost-core":  {Current: "python:3.12-slim", Previous: "python:3.11-slim"},
				"gradient-boost-lab":   {Current: "custom/jupyter"},
				"gradient-boost-track": {Current: "ghcr.io/mlflow/mlflow:2.14"},
			},
		}, nil
	}
	dockerStatusFn = func(ctx context.Context, name string) (string, error) {
		return "running", nil
	}

	msg := loadSuitesCmd()().(suitesLoadedMsg)
	if msg.err != nil {
		t.Fatalf("loadSuitesCmd() error = %v", msg.err)
	}
	if msg.rows[0].State != "● running" {
		t.Fatalf("row state = %q", msg.rows[0].State)
	}
	if msg.rows[3].State != "— not installed" {
		t.Fatalf("forge state = %q", msg.rows[3].State)
	}
}

func TestSuitesUpdateRemoveAndUpdateCancel(t *testing.T) {
	m := NewSuitesModel()
	m.rows = []suiteRow{{Name: "boosting", Installed: true}}
	m.confirmRemove = true
	m.confirmUpdate = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.confirmRemove || updated.confirmUpdate {
		t.Fatal("expected confirmations to cancel")
	}
}

func TestPerformRollbackRequiresPreviousVersion(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	loadManifestFn = func() (cfgstore.VersionManifest, error) {
		return cfgstore.VersionManifest{
			"boosting": {
				"gradient-boost-core": {Current: "python:3.12-slim"},
			},
		}, nil
	}

	err := performRollback("boosting", func(string) {})
	if err == nil || !strings.Contains(err.Error(), "no previous version") {
		t.Fatalf("performRollback() error = %v", err)
	}
}

func TestSuitesViewAndUpdateDiff(t *testing.T) {
	m := NewSuitesModel()
	m.loading = false
	m.width = 120
	m.rows = []suiteRow{
		{
			Name:      "boosting",
			Installed: true,
			State:     "● running",
			Image:     "python:3.12-slim",
			Detail: suiteDetail{
				Suite: suite.Suite{
					Name: "boosting",
					Containers: []suite.Container{
						{Name: "gradient-boost-core", Image: "python:3.13-slim", Role: "Core"},
					},
					Ports:   []suite.PortMapping{{Port: 8888, Service: "JupyterLab"}},
					Volumes: []suite.VolumeMount{{HostPath: "models", ContainerPath: "/models"}},
				},
				Current:  map[string]string{"gradient-boost-core": "python:3.12-slim"},
				Previous: map[string]string{"gradient-boost-core": "python:3.11-slim"},
			},
		},
	}

	if m.HelpView() == "" || m.View() == "" {
		t.Fatal("expected suites help/view output")
	}
	if len(m.detailView(m.rows[0])) == 0 || len(m.updateDiffLines(m.rows[0])) == 0 {
		t.Fatal("expected detail and update diff lines")
	}
	updated, _ := m.Update(keyRunes("u"))
	if !updated.confirmUpdate {
		t.Fatal("expected update confirmation")
	}
}

func TestPerformUpdateStartStopRestartAndRemove(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	loadManifestFn = func() (cfgstore.VersionManifest, error) {
		return cfgstore.VersionManifest{
			"boosting": {
				"gradient-boost-core": {Current: "python:3.12-slim", Previous: "python:3.11-slim"},
			},
		}, nil
	}

	dockerTagPreviousFn = func(image string) error { return nil }
	dockerPullStreamFn = func(ctx context.Context, image string, cb func(string)) error { return nil }
	saveManifestFn = func(manifest cfgstore.VersionManifest) error { return nil }
	dockerWriteComposeFn = func(name string) (string, error) { return "/tmp/" + name + ".compose.yml", nil }
	dockerComposePathFn = func(name string) string { return "/tmp/" + name + ".compose.yml" }

	upCalls := 0
	downCalls := 0
	dockerComposeUpFn = func(ctx context.Context, path string, detach bool) error {
		upCalls++
		return nil
	}
	dockerComposeDownFn = func(ctx context.Context, path string) error {
		downCalls++
		return nil
	}
	systemRegisterFn = func(s suite.Suite) error { return nil }
	systemDeregisterFn = func(s suite.Suite) error { return nil }
	runComposeRemoveFn = func(ctx context.Context, composePath string) error { return nil }
	removeSuiteFn = func(name string) error { return nil }

	if err := performUpdate("boosting", func(string) {}); err != nil {
		t.Fatalf("performUpdate() error = %v", err)
	}
	if err := performStart("boosting"); err != nil {
		t.Fatalf("performStart() error = %v", err)
	}
	if err := performStop("boosting"); err != nil {
		t.Fatalf("performStop() error = %v", err)
	}
	if err := performRestart("boosting"); err != nil {
		t.Fatalf("performRestart() error = %v", err)
	}
	if err := performRemove("boosting"); err != nil {
		t.Fatalf("performRemove() error = %v", err)
	}
	if upCalls == 0 || downCalls == 0 {
		t.Fatalf("expected compose up/down calls, got up=%d down=%d", upCalls, downCalls)
	}
}

func TestPerformRollbackSuccess(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	loadManifestFn = func() (cfgstore.VersionManifest, error) {
		return cfgstore.VersionManifest{
			"boosting": {
				"gradient-boost-core":  {Current: "python:3.12-slim", Previous: "python:3.11-slim"},
				"gradient-boost-lab":   {Current: "lab:new", Previous: "lab:old"},
				"gradient-boost-track": {Current: "mlflow:new", Previous: "mlflow:old"},
			},
		}, nil
	}
	saveManifestFn = func(manifest cfgstore.VersionManifest) error { return nil }
	dockerComposePathFn = func(name string) string { return "/tmp/" + name + ".compose.yml" }
	dockerComposeDownFn = func(ctx context.Context, path string) error { return nil }
	dockerComposeUpFn = func(ctx context.Context, path string, detach bool) error { return nil }
	dockerWriteComposeFn = func(name string) (string, error) { return "/tmp/" + name + ".compose.yml", nil }

	if err := performRollback("boosting", func(string) {}); err != nil {
		t.Fatalf("performRollback() error = %v", err)
	}
}

func TestSuiteOperationHelpersAndInteractiveCommands(t *testing.T) {
	restoreModelDeps(t)

	m := NewSuitesModel()
	m.rows = []suiteRow{
		{
			Name:      "boosting",
			Installed: true,
			Detail:    suiteDetail{Suite: suite.Registry["boosting"]},
		},
	}
	dockerComposePathFn = func(name string) string { return "/tmp/" + name + ".compose.yml" }
	dockerComposeUpFn = func(ctx context.Context, path string, detach bool) error { return nil }
	systemRegisterFn = func(s suite.Suite) error { return nil }

	if cmd := m.openShell(); cmd == nil {
		t.Fatal("expected shell exec cmd")
	}
	if cmd := m.execSuiteCommand("python -V"); cmd == nil {
		t.Fatal("expected exec cmd")
	}

	ch := make(chan suiteOperationMsg, 1)
	ch <- suiteOperationMsg{line: "done"}
	if waitForSuiteOperation(ch)() == nil {
		t.Fatal("expected wait cmd to yield a message")
	}

	opCh := make(chan suiteOperationMsg, 4)
	runSuiteOperation("start", "boosting", opCh)
	if len(opCh) == 0 {
		t.Fatal("expected operation messages")
	}
	dockerStatusFn = func(ctx context.Context, name string) (string, error) { return "running", nil }
	dockerOutputFn = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte(`{"url":"http://0.0.0.0:8888/","token":"xyz"}`), nil
	}
	systemOpenURLFn = func(url string) error { return nil }
	opCh = make(chan suiteOperationMsg, 4)
	runSuiteOperation("lab", "boosting", opCh)
	if len(opCh) == 0 {
		t.Fatal("expected lab operation messages")
	}
}

func TestStartOperationWaitsForCompletion(t *testing.T) {
	restoreModelDeps(t)

	isInstalledFn = func(name string) (bool, error) { return true, nil }
	dockerComposePathFn = func(name string) string { return "/tmp/" + name + ".compose.yml" }
	dockerComposeUpFn = func(ctx context.Context, path string, detach bool) error { return nil }
	systemRegisterFn = func(s suite.Suite) error { return nil }

	m := NewSuitesModel()
	m.rows = []suiteRow{{Name: "boosting", Installed: true, Detail: suiteDetail{Suite: suite.Registry["boosting"]}}}
	cmd := m.startOperation("start")
	if cmd == nil || m.opCh == nil {
		t.Fatal("expected operation channel")
	}
	for range m.opCh {
	}
}
