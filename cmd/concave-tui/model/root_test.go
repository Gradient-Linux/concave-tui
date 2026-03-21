package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	cfgstore "github.com/Gradient-Linux/concave-tui/internal/config"
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
	oldSaveTUIConfig := saveTUIConfigFn
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
	oldDashboardGPUStats := dashboardGPUStatsFn
	oldDashboardCUDA := dashboardCUDAFn
	oldDashboardReadMem := dashboardReadMemFn
	oldDashboardTickNow := dashboardTickNowFn
	oldDashboardDocker := dashboardSystemDocker
	oldDashboardInternet := dashboardInternetFn
	oldDoctorGPUStats := doctorGPUStatsFn
	oldDoctorCUDA := doctorCUDAFn
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
		saveTUIConfigFn = oldSaveTUIConfig
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
		dashboardGPUStatsFn = oldDashboardGPUStats
		dashboardCUDAFn = oldDashboardCUDA
		dashboardReadMemFn = oldDashboardReadMem
		dashboardTickNowFn = oldDashboardTickNow
		dashboardSystemDocker = oldDashboardDocker
		dashboardInternetFn = oldDashboardInternet
		doctorGPUStatsFn = oldDoctorGPUStats
		doctorCUDAFn = oldDoctorCUDA
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

func testConfig() tuiconfig.Config {
	return tuiconfig.DefaultConfig()
}

func TestRootSwitchesViewsAndTogglesChrome(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})

	updated, _ := m.Update(keyRunes("2"))
	root := updated.(*RootModel)
	if root.activeView != ViewSuites {
		t.Fatalf("activeView = %v, want %v", root.activeView, ViewSuites)
	}

	updated, _ = root.Update(keyRunes("?"))
	root = updated.(*RootModel)
	if !root.showHelp {
		t.Fatal("expected help overlay to be visible")
	}

	updated, _ = root.Update(keyRunes(","))
	root = updated.(*RootModel)
	if !root.showSettings {
		t.Fatal("expected settings overlay to be visible")
	}
}

func TestRootSidebarToggleAndPresetCycle(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 140
	m.height = 40
	m.applyLayout()
	if m.sidebar != SidebarExpanded {
		t.Fatalf("sidebar = %v, want expanded", m.sidebar)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	root := updated.(*RootModel)
	if root.sidebar != SidebarCollapsed {
		t.Fatalf("sidebar = %v, want collapsed", root.sidebar)
	}

	current := root.cfg.ActivePreset
	updated, _ = root.Update(keyRunes("p"))
	root = updated.(*RootModel)
	if root.cfg.ActivePreset == current {
		t.Fatal("expected preset cycle")
	}
}

func TestRootViewTooNarrow(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 79
	if got := m.View(); got != "Terminal too narrow — resize to at least 80 columns" {
		t.Fatalf("View() = %q", got)
	}
}

func TestRootSettingsSaveAndDiscard(t *testing.T) {
	restoreModelDeps(t)

	saved := testConfig()
	saveTUIConfigFn = func(cfg tuiconfig.Config) error {
		saved = cfg
		return nil
	}

	m := NewRootModel("dev", testConfig())
	m.width = 140
	m.height = 40
	m.applyLayout()

	updated, _ := m.Update(keyRunes(","))
	root := updated.(*RootModel)
	root.settings.current.ActivePreset = "mlops"
	updated, _ = root.Update(settingsSavedMsg{Config: root.settings.current})
	root = updated.(*RootModel)
	if root.showSettings {
		t.Fatal("expected settings to close on save")
	}
	if root.cfg.ActivePreset != "mlops" {
		t.Fatalf("active preset = %q", root.cfg.ActivePreset)
	}
	if msg := saveTUIConfigCmd(root.cfg)(); msg.(rootConfigSavedMsg).err != nil {
		t.Fatalf("saveTUIConfigCmd() error = %v", msg.(rootConfigSavedMsg).err)
	}
	if saved.ActivePreset != "mlops" {
		t.Fatalf("saved preset = %q", saved.ActivePreset)
	}

	updated, _ = root.Update(keyRunes(","))
	root = updated.(*RootModel)
	root.settings.current.ActivePreset = "training"
	updated, _ = root.Update(settingsDiscardedMsg{})
	root = updated.(*RootModel)
	if root.cfg.ActivePreset != "mlops" {
		t.Fatalf("discard should keep saved config, got %q", root.cfg.ActivePreset)
	}
}

func TestRootHelpersAndLabURL(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 140
	m.height = 40
	m.applyLayout()
	if m.headerView() == "" || m.footerView() == "" || m.sidebarView() == "" || m.contentView() == "" {
		t.Fatal("expected root chrome output")
	}
	if gradientText("gradient") == "" || gradientBar(10, 0.5, false) == "" {
		t.Fatal("expected gradient helpers")
	}

	url, err := extractLabURL(`{"url":"http://0.0.0.0:8888/","token":"abc123"}`)
	if err != nil {
		t.Fatalf("extractLabURL() error = %v", err)
	}
	if url != "http://localhost:8888/lab?token=abc123" {
		t.Fatalf("extractLabURL() = %q", url)
	}
}

func TestOrderedInstalledSuitesAndForgeHelpers(t *testing.T) {
	restoreModelDeps(t)

	got := orderedInstalledSuites([]string{"flow", "boosting", "forge"})
	want := []string{"boosting", "flow", "forge"}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("orderedInstalledSuites()[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}

	loadManifestFn = func() (cfgstore.VersionManifest, error) {
		return cfgstore.VersionManifest{
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
	if _, err := currentSuiteDefinition("forge"); err != nil {
		t.Fatalf("currentSuiteDefinition() error = %v", err)
	}
}

func TestOpenLabURLAndDockerHelper(t *testing.T) {
	restoreModelDeps(t)

	dockerStatusFn = func(ctx context.Context, name string) (string, error) { return "running", nil }
	dockerOutputFn = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte(`{"url":"http://0.0.0.0:8888/","token":"xyz"}`), nil
	}
	url, err := openLabURL("boosting")
	if err != nil {
		t.Fatalf("openLabURL() error = %v", err)
	}
	if !strings.Contains(url, "token=xyz") {
		t.Fatalf("openLabURL() = %q", url)
	}
	_, _ = runDockerOutput(context.Background(), "version")
}

func TestRootUsesWorkspaceAndGPUHelpers(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	workspaceRootFn = func() string { return tmp }
	loadStateFn = func() (cfgstore.State, error) { return cfgstore.State{Installed: []string{}}, nil }
	workspaceEnsureFn = func() error { return nil }
	workspaceStatusFn = func() ([]workspace.Usage, error) {
		return []workspace.Usage{{Name: "models", Bytes: 2048}}, nil
	}
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	dashboardGPUStatsFn = func() ([]gpu.NVIDIADevice, error) { return nil, nil }
	dashboardCUDAFn = func() (string, error) { return "", nil }
	dashboardReadMemFn = func() (uint64, uint64, error) { return 1024, 2048, nil }
	dashboardTickNowFn = func() time.Time { return time.Unix(100, 0) }
	dashboardSystemDocker = func() (bool, error) { return true, nil }
	dashboardInternetFn = func() (bool, error) { return true, nil }

	msg := loadDashboardCmd(1)().(dashboardLoadedMsg)
	if msg.loadErr != nil {
		t.Fatalf("loadDashboardCmd() error = %v", msg.loadErr)
	}
	if len(msg.metrics.Suites) != len(viewOrder) {
		t.Fatalf("suite count = %d, want %d", len(msg.metrics.Suites), len(viewOrder))
	}
}
