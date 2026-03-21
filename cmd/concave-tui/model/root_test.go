package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

func restoreModelDeps(t *testing.T) {
	t.Helper()

	oldLoadState := loadStateFn
	oldSaveState := saveStateFn
	oldAddSuite := addSuiteFn
	oldRemoveSuite := removeSuiteFn
	oldIsInstalled := isInstalledFn
	oldLoadManifest := loadManifestFn
	oldSaveManifest := saveManifestFn
	oldRecordInstall := recordInstallFn
	oldRecordUpdate := recordUpdateFn
	oldSwapRollback := swapRollbackFn
	oldSuiteGet := suiteGetFn
	oldSuiteAll := suiteAllFn
	oldSuiteNames := suiteNamesFn
	oldSuitePrimary := suitePrimaryFn
	oldSuiteJupyter := suiteJupyterFn
	oldSuiteSelection := suiteSelectionFn
	oldSuiteBuildForge := suiteBuildForgeFn
	oldSuitePickForge := suitePickForgeFn
	oldDockerPull := dockerPullFn
	oldDockerPullStream := dockerPullStreamFn
	oldDockerTagPrevious := dockerTagPreviousFn
	oldDockerRevertPrev := dockerRevertPrevFn
	oldDockerComposeUp := dockerComposeUpFn
	oldDockerComposeDown := dockerComposeDownFn
	oldDockerComposePath := dockerComposePathFn
	oldDockerWriteCompose := dockerWriteComposeFn
	oldDockerWriteRaw := dockerWriteRawFn
	oldDockerStatus := dockerStatusFn
	oldDockerOutput := dockerOutputFn
	oldDockerLogStream := dockerLogStreamFn
	oldSystemRegister := systemRegisterFn
	oldSystemDeregister := systemDeregisterFn
	oldSystemDocker := systemDockerFn
	oldSystemGroup := systemGroupFn
	oldSystemInternet := systemInternetFn
	oldSystemOpenURL := systemOpenURLFn
	oldWorkspaceRoot := workspaceRootFn
	oldWorkspaceEnsure := workspaceEnsureFn
	oldWorkspaceStatus := workspaceStatusFn
	oldWorkspaceBackup := workspaceBackupFn
	oldWorkspaceClean := workspaceCleanFn
	oldGPUDetect := gpuDetectFn
	oldGPURecommended := gpuRecommendedFn
	oldGPUToolkit := gpuToolkitFn
	oldDashboardNVIDIA := dashboardNVIDIAInfoFn
	oldRunComposeRemove := runComposeRemoveFn

	t.Cleanup(func() {
		loadStateFn = oldLoadState
		saveStateFn = oldSaveState
		addSuiteFn = oldAddSuite
		removeSuiteFn = oldRemoveSuite
		isInstalledFn = oldIsInstalled
		loadManifestFn = oldLoadManifest
		saveManifestFn = oldSaveManifest
		recordInstallFn = oldRecordInstall
		recordUpdateFn = oldRecordUpdate
		swapRollbackFn = oldSwapRollback
		suiteGetFn = oldSuiteGet
		suiteAllFn = oldSuiteAll
		suiteNamesFn = oldSuiteNames
		suitePrimaryFn = oldSuitePrimary
		suiteJupyterFn = oldSuiteJupyter
		suiteSelectionFn = oldSuiteSelection
		suiteBuildForgeFn = oldSuiteBuildForge
		suitePickForgeFn = oldSuitePickForge
		dockerPullFn = oldDockerPull
		dockerPullStreamFn = oldDockerPullStream
		dockerTagPreviousFn = oldDockerTagPrevious
		dockerRevertPrevFn = oldDockerRevertPrev
		dockerComposeUpFn = oldDockerComposeUp
		dockerComposeDownFn = oldDockerComposeDown
		dockerComposePathFn = oldDockerComposePath
		dockerWriteComposeFn = oldDockerWriteCompose
		dockerWriteRawFn = oldDockerWriteRaw
		dockerStatusFn = oldDockerStatus
		dockerOutputFn = oldDockerOutput
		dockerLogStreamFn = oldDockerLogStream
		systemRegisterFn = oldSystemRegister
		systemDeregisterFn = oldSystemDeregister
		systemDockerFn = oldSystemDocker
		systemGroupFn = oldSystemGroup
		systemInternetFn = oldSystemInternet
		systemOpenURLFn = oldSystemOpenURL
		workspaceRootFn = oldWorkspaceRoot
		workspaceEnsureFn = oldWorkspaceEnsure
		workspaceStatusFn = oldWorkspaceStatus
		workspaceBackupFn = oldWorkspaceBackup
		workspaceCleanFn = oldWorkspaceClean
		gpuDetectFn = oldGPUDetect
		gpuRecommendedFn = oldGPURecommended
		gpuToolkitFn = oldGPUToolkit
		dashboardNVIDIAInfoFn = oldDashboardNVIDIA
		runComposeRemoveFn = oldRunComposeRemove
	})
}

func keyRunes(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

func prependMockDocker(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	script := "#!/bin/sh\nset -eu\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestRootSwitchesViewsAndTogglesHelp(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev")
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	root := updated.(*RootModel)
	if root.activeView != ViewSuites {
		t.Fatalf("activeView = %v, want %v", root.activeView, ViewSuites)
	}

	updated, _ = root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	root = updated.(*RootModel)
	if !root.showHelp {
		t.Fatal("expected help overlay to be visible")
	}
}

func TestRootViewTooNarrow(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev")
	m.width = 79
	if got := m.View(); got != "Terminal too narrow — resize to at least 80 columns" {
		t.Fatalf("View() = %q", got)
	}
}

func TestRootInitViewHelpersAndChrome(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev")
	m.width = 120
	m.height = 40
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected init cmd")
	}
	if m.contentWidth() <= 0 || m.contentHeight() <= 0 {
		t.Fatal("expected positive content size")
	}
	if m.headerView() == "" || m.footerView() == "" {
		t.Fatal("expected chrome output")
	}
	if m.activeContent() == "" {
		t.Fatal("expected active content")
	}
	if m.activeHelp() == "" || m.helpOverlay() == "" {
		t.Fatal("expected help output")
	}
	if gradientText("gradient") == "" {
		t.Fatal("expected gradient title")
	}
	if view := m.View(); view == "" {
		t.Fatal("expected full root view")
	}
}

func TestExtractLabURL(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		url, err := extractLabURL(`{"url":"http://0.0.0.0:8888/","token":"abc123"}`)
		if err != nil {
			t.Fatalf("extractLabURL() error = %v", err)
		}
		if url != "http://localhost:8888/lab?token=abc123" {
			t.Fatalf("extractLabURL() = %q", url)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		url, err := extractLabURL(`http://0.0.0.0:8888/?token=abc123`)
		if err != nil {
			t.Fatalf("extractLabURL() error = %v", err)
		}
		if url != "http://127.0.0.1:8888/lab?token=abc123" {
			t.Fatalf("extractLabURL() = %q", url)
		}
	})
}

func TestOrderedInstalledSuites(t *testing.T) {
	got := orderedInstalledSuites([]string{"flow", "boosting", "forge"})
	want := []string{"boosting", "flow", "forge"}
	if len(got) != len(want) {
		t.Fatalf("orderedInstalledSuites() len = %d, want %d", len(got), len(want))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("orderedInstalledSuites()[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}
}

func TestCurrentSuiteDefinitionForgeUsesManifestSelection(t *testing.T) {
	restoreModelDeps(t)

	loadManifestFn = func() (config.VersionManifest, error) {
		return config.VersionManifest{
			"forge": {
				"gradient-boost-lab":  {Current: "custom/jupyter"},
				"gradient-flow-serve": {Current: "custom/bento"},
			},
		}, nil
	}
	suiteSelectionFn = func(names []string, overrides map[string]string) (suite.ForgeSelection, error) {
		return suite.ForgeSelection{
			Containers: []suite.Container{
				{Name: names[0], Image: overrides[names[0]], Role: "JupyterLab"},
				{Name: names[1], Image: overrides[names[1]], Role: "Model serving"},
			},
			Ports:   []suite.PortMapping{{Port: 8888, Service: "JupyterLab"}},
			Volumes: []suite.VolumeMount{{HostPath: "models", ContainerPath: "/models"}},
		}, nil
	}

	got, err := currentSuiteDefinition("forge")
	if err != nil {
		t.Fatalf("currentSuiteDefinition() error = %v", err)
	}
	if len(got.Containers) != 2 {
		t.Fatalf("forge containers = %d, want 2", len(got.Containers))
	}
}

func TestWriteComposeForSuiteForgeUsesRawWriter(t *testing.T) {
	restoreModelDeps(t)

	loadManifestFn = func() (config.VersionManifest, error) {
		return config.VersionManifest{
			"forge": {
				"gradient-boost-lab": {Current: "custom/jupyter"},
			},
		}, nil
	}
	suiteSelectionFn = func(names []string, overrides map[string]string) (suite.ForgeSelection, error) {
		return suite.ForgeSelection{
			Containers: []suite.Container{{Name: "gradient-boost-lab", Image: "custom/jupyter", Role: "JupyterLab"}},
			Ports:      []suite.PortMapping{{Port: 8888, Service: "JupyterLab"}},
			Volumes:    []suite.VolumeMount{{HostPath: "notebooks", ContainerPath: "/notebooks"}},
		}, nil
	}
	suiteBuildForgeFn = func(selection suite.ForgeSelection) ([]byte, error) {
		if len(selection.Containers) != 1 {
			t.Fatalf("BuildForgeCompose selection len = %d, want 1", len(selection.Containers))
		}
		return []byte("services:\n"), nil
	}

	var writtenName string
	dockerWriteRawFn = func(name string, data []byte) (string, error) {
		writtenName = name
		return "/tmp/" + name + ".compose.yml", nil
	}

	path, err := writeComposeForSuite("forge")
	if err != nil {
		t.Fatalf("writeComposeForSuite() error = %v", err)
	}
	if writtenName != "forge" {
		t.Fatalf("written suite = %q, want forge", writtenName)
	}
	if path != "/tmp/forge.compose.yml" {
		t.Fatalf("path = %q", path)
	}
}

func TestOpenLabURLPrefersJupyterJSON(t *testing.T) {
	restoreModelDeps(t)

	loadManifestFn = func() (config.VersionManifest, error) { return config.VersionManifest{}, nil }
	dockerStatusFn = func(ctx context.Context, name string) (string, error) { return "running", nil }
	suiteJupyterFn = func(s suite.Suite) (string, bool) { return "gradient-boost-lab", true }
	workspaceRootFn = workspace.Root

	dockerOutputFn = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte(`{"url":"http://0.0.0.0:8888/","token":"xyz"}`), nil
	}

	url, err := openLabURL("boosting")
	if err != nil {
		t.Fatalf("openLabURL() error = %v", err)
	}
	if url != "http://localhost:8888/lab?token=xyz" {
		t.Fatalf("openLabURL() = %q", url)
	}
}

func TestDefaultGPUFnsRemainCallableInTests(t *testing.T) {
	restoreModelDeps(t)
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	if got := dashboardGPULine(); got == "" {
		t.Fatal("expected non-empty GPU line")
	}
}

func TestRootActivateAndDeactivateView(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev")
	if cmd := m.activateView(ViewDoctor); cmd == nil {
		t.Fatal("expected doctor activate cmd")
	}
	m.deactivateView(ViewDoctor)
	if m.doctor.active {
		t.Fatal("expected doctor view to deactivate")
	}
}

func TestRunDockerOutputUsesDockerOnPath(t *testing.T) {
	restoreModelDeps(t)
	prependMockDocker(t, `printf '{"token":"abc"}\n'`)

	out, err := runDockerOutput(context.Background(), "exec", "demo")
	if err != nil {
		t.Fatalf("runDockerOutput() error = %v", err)
	}
	if string(out) != "{\"token\":\"abc\"}\n" {
		t.Fatalf("runDockerOutput() = %q", string(out))
	}
}

func TestRootUpdateRoutesAcrossViews(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev")
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for _, key := range []string{"1", "2", "3", "4", "5", "tab", "shift+tab"} {
		updated, _ := m.Update(keyRunes(key))
		m = updated.(*RootModel)
	}

	for _, view := range []View{ViewDashboard, ViewSuites, ViewLogs, ViewWorkspace, ViewDoctor} {
		m.activeView = view
		if m.activeContent() == "" {
			t.Fatalf("activeContent() empty for view %v", view)
		}
		if m.activeHelp() == "" {
			t.Fatalf("activeHelp() empty for view %v", view)
		}
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("expected quit cmd")
	}
}
