package model

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
)

func restoreModelDeps(t *testing.T) {
	t.Helper()

	oldSaveTUIConfig := saveTUIConfigFn
	oldLoadSession := loadSessionFn
	oldSaveSession := saveSessionFn
	oldClearSession := clearSessionFn
	oldNewClient := newClientFn
	oldSharedClient := sharedClient
	oldAPILogin := apiLoginFn
	oldAPIRefresh := apiRefreshFn
	oldAPISuites := apiSuitesFn
	oldAPISuite := apiSuiteFn
	oldAPIWorkspace := apiWorkspaceFn
	oldAPIWorkspaceBackup := apiWorkspaceBackupFn
	oldAPIWorkspaceClean := apiWorkspaceCleanFn
	oldAPIDoctor := apiDoctorFn
	oldAPISystemInfo := apiSystemInfoFn
	oldAPIUsersActivity := apiUsersActivityFn
	oldAPISystemAction := apiSystemActionFn
	oldAPISuiteAction := apiSuiteActionFn
	oldAPIJob := apiJobFn
	oldAPILabURL := apiLabURLFn
	oldAPIChangelog := apiChangelogFn
	oldAPILogsDial := apiLogsDialFn

	t.Cleanup(func() {
		saveTUIConfigFn = oldSaveTUIConfig
		loadSessionFn = oldLoadSession
		saveSessionFn = oldSaveSession
		clearSessionFn = oldClearSession
		newClientFn = oldNewClient
		sharedClient = oldSharedClient
		apiLoginFn = oldAPILogin
		apiRefreshFn = oldAPIRefresh
		apiSuitesFn = oldAPISuites
		apiSuiteFn = oldAPISuite
		apiWorkspaceFn = oldAPIWorkspace
		apiWorkspaceBackupFn = oldAPIWorkspaceBackup
		apiWorkspaceCleanFn = oldAPIWorkspaceClean
		apiDoctorFn = oldAPIDoctor
		apiSystemInfoFn = oldAPISystemInfo
		apiUsersActivityFn = oldAPIUsersActivity
		apiSystemActionFn = oldAPISystemAction
		apiSuiteActionFn = oldAPISuiteAction
		apiJobFn = oldAPIJob
		apiLabURLFn = oldAPILabURL
		apiChangelogFn = oldAPIChangelog
		apiLogsDialFn = oldAPILogsDial
	})
}

func keyRunes(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

func testConfig() tuiconfig.Config {
	return tuiconfig.DefaultConfig()
}

func authSession(role tuiauth.Role) tuiauth.Session {
	return tuiauth.Session{Token: "token", Username: "alice", Role: role}
}

func TestRootShowsLoginWhenUnauthenticated(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig())
	m.width = 120
	m.height = 40
	m.applyLayout()

	if got := m.View(); got == "" || !containsAll(got, "Username", "Password") {
		t.Fatalf("View() = %q", got)
	}
}

func TestRootSwitchesViewsWhenAuthenticated(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleAdmin))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})

	updated, _ := m.Update(keyRunes("2"))
	root := updated.(*RootModel)
	if root.activeView != ViewSuites {
		t.Fatalf("activeView = %v, want %v", root.activeView, ViewSuites)
	}

	updated, _ = root.Update(keyRunes("5"))
	root = updated.(*RootModel)
	if root.activeView != ViewSystem {
		t.Fatalf("activeView = %v, want %v", root.activeView, ViewSystem)
	}
}

func TestRootViewTooNarrowWhenAuthenticated(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleViewer))
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

	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleViewer))
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
}

func TestHelpOverlayShowsRoleFilteredActions(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleViewer))
	m.width = 140
	m.height = 40
	m.applyLayout()
	m.activeView = ViewSuites

	help := m.activeHelp()
	if containsAll(help, "install suite", "remove suite") {
		t.Fatalf("viewer help should not expose operator actions: %q", help)
	}
}

func TestAdminVisibleViewsIncludeSystemAndUsers(t *testing.T) {
	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleAdmin))
	views := m.visibleViews()
	if len(views) != 6 || views[4] != ViewSystem || views[5] != ViewUsers {
		t.Fatalf("visibleViews() = %#v", views)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
