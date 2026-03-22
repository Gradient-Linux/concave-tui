package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	oldWorkspaceReadMem := workspaceReadMemFn
	oldWorkspaceGPUStats := workspaceGPUStatsFn
	oldWorkspaceReadCPU := workspaceReadCPUSnapshotFn
	oldGPUDetect := gpuDetectFn
	oldGPURecommended := gpuRecommendedFn
	oldGPUToolkit := gpuToolkitFn
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
		workspaceReadMemFn = oldWorkspaceReadMem
		workspaceGPUStatsFn = oldWorkspaceGPUStats
		workspaceReadCPUSnapshotFn = oldWorkspaceReadCPU
		gpuDetectFn = oldGPUDetect
		gpuRecommendedFn = oldGPURecommended
		gpuToolkitFn = oldGPUToolkit
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

func TestRootSidebarToggleFromLogsView(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 140
	m.height = 40
	m.applyLayout()
	m.activeView = ViewLogs
	if m.sidebar != SidebarExpanded {
		t.Fatalf("sidebar = %v, want expanded", m.sidebar)
	}

	updated, _ := m.Update(keyRunes("b"))
	root := updated.(*RootModel)
	if root.sidebar != SidebarCollapsed {
		t.Fatalf("sidebar = %v, want collapsed", root.sidebar)
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
	root.settings.current.Display.SidebarDefault = "collapsed"
	updated, _ = root.Update(settingsSavedMsg{Config: root.settings.current})
	root = updated.(*RootModel)
	if root.showSettings {
		t.Fatal("expected settings to close on save")
	}
	if root.cfg.Display.SidebarDefault != "collapsed" {
		t.Fatalf("sidebar default = %q", root.cfg.Display.SidebarDefault)
	}
	if msg := saveTUIConfigCmd(root.cfg)(); msg.(rootConfigSavedMsg).err != nil {
		t.Fatalf("saveTUIConfigCmd() error = %v", msg.(rootConfigSavedMsg).err)
	}
	if saved.Display.SidebarDefault != "collapsed" {
		t.Fatalf("saved sidebar default = %q", saved.Display.SidebarDefault)
	}

	updated, _ = root.Update(keyRunes(","))
	root = updated.(*RootModel)
	root.settings.current.Display.SidebarDefault = "expanded"
	updated, _ = root.Update(settingsDiscardedMsg{})
	root = updated.(*RootModel)
	if root.cfg.Display.SidebarDefault != "collapsed" {
		t.Fatalf("discard should keep saved config, got %q", root.cfg.Display.SidebarDefault)
	}
}

func TestRootRendersCenteredModalAndModeIndicator(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 140
	m.height = 40
	m.applyLayout()
	m.showSettings = true

	view := m.View()
	if !strings.Contains(view, "Settings") {
		t.Fatalf("expected settings modal in view, got %q", view)
	}
	if !strings.Contains(view, "INSERT") {
		t.Fatalf("expected insert mode footer while settings numeric input is focused, got %q", view)
	}

	m.settings.focusedField = settingsFieldRefresh
	m.settings.insertMode = true
	if !strings.Contains(m.footerView(), "INSERT") {
		t.Fatalf("expected insert mode indicator, got %q", m.footerView())
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
	workspaceEnsureFn = func() error { return nil }
	workspaceStatusFn = func() ([]workspace.Usage, error) {
		return []workspace.Usage{{Name: "models", Bytes: 2048}}, nil
	}
	gpuDetectFn = func() (gpu.GPUState, error) { return gpu.GPUStateNone, nil }
	workspaceReadMemFn = func() (uint64, uint64, error) { return 1024, 2048, nil }
	workspaceGPUStatsFn = func() ([]gpu.NVIDIADevice, error) { return nil, nil }
	workspaceReadCPUSnapshotFn = func() (cpuSnapshot, error) {
		return cpuSnapshot{
			total: cpuTotals{total: 100, idle: 40},
			cores: []cpuTotals{
				{total: 100, idle: 30},
				{total: 100, idle: 50},
			},
		}, nil
	}

	msg := loadWorkspaceCmd(1)().(workspaceLoadedMsg)
	if msg.loadErr != nil {
		t.Fatalf("loadWorkspaceCmd() error = %v", msg.loadErr)
	}
	if msg.root != tmp {
		t.Fatalf("root = %q, want %q", msg.root, tmp)
	}
	if msg.ramUsedBytes != 1024 || msg.ramTotalBytes != 2048 {
		t.Fatalf("ram = %d/%d", msg.ramUsedBytes, msg.ramTotalBytes)
	}
	if len(msg.coreUsage) != 2 {
		t.Fatalf("core usage count = %d", len(msg.coreUsage))
	}
}
