package model

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	workspacepkg "github.com/Gradient-Linux/concave-tui/internal/workspace"
)

func TestWorkspaceUpdateCleanCancel(t *testing.T) {
	m := NewWorkspaceModel()
	m.confirmClean = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.confirmClean {
		t.Fatal("expected clean confirmation to cancel")
	}
}

func TestWorkspaceUpdateBackupCompletionAndExpiry(t *testing.T) {
	m := NewWorkspaceModel()
	updated, cmd := m.Update(workspaceOpMsg{kind: "backup", success: true, detail: "backup created"})
	if updated.completionNote != "backup created" {
		t.Fatalf("completionNote = %q", updated.completionNote)
	}
	if cmd == nil {
		t.Fatal("expected reload and expiry command")
	}
	updated, _ = updated.Update(workspaceNoteExpiredMsg{})
	if updated.completionNote != "" {
		t.Fatal("expected completion note to expire")
	}
}

func TestWorkspaceBarsAndThresholds(t *testing.T) {
	m := NewWorkspaceModel()
	m.width = 120
	m.total = 100
	m.used = 90
	m.usages = []workspacepkg.Usage{
		{Name: "models", Bytes: 50},
		{Name: "outputs", Bytes: 10},
	}

	if got := m.totalBar(); got == "" {
		t.Fatal("expected total bar")
	}
	if got := m.directoryBar(25, 50); got == "" {
		t.Fatal("expected directory bar")
	}
	if m.outputsUsage().Name != "outputs" {
		t.Fatal("expected outputs usage lookup")
	}
}

func TestLoadWorkspaceCmdReturnsUsageData(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	workspaceRootFn = func() string { return tmp }
	workspaceEnsureFn = func() error { return nil }
	workspaceStatusFn = func() ([]workspacepkg.Usage, error) {
		return []workspacepkg.Usage{
			{Name: "models", Bytes: 2048},
			{Name: "notebooks", Bytes: 1024},
		}, nil
	}

	msg := loadWorkspaceCmd(4)().(workspaceLoadedMsg)
	if msg.loadErr != nil {
		t.Fatalf("loadWorkspaceCmd() error = %v", msg.loadErr)
	}
	if len(msg.usages) != 2 {
		t.Fatalf("usage lines = %d, want 2", len(msg.usages))
	}
}

func TestWorkspaceActivateHelpersAndCommands(t *testing.T) {
	restoreModelDeps(t)

	m := NewWorkspaceModel()
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	if m.HelpView() == "" {
		t.Fatal("expected help text")
	}
	m.loading = false
	m.root = "~/gradient"
	m.total = 100
	m.used = 25
	m.usages = []workspacepkg.Usage{{Name: "models", Bytes: 1024}}
	m.lastBackup = time.Now().Add(-2 * time.Hour)
	if view := m.View(); view == "" || !strings.Contains(view, "Last backup") {
		t.Fatalf("expected workspace view, got %q", view)
	}
	if workspaceTickCmd(1)() != (workspaceTickMsg{token: 1}) {
		t.Fatal("expected tick message")
	}

	workspaceBackupFn = func() (string, error) { return "/tmp/backup.tar.gz", nil }
	workspaceCleanFn = func() error { return nil }
	if msg := runWorkspaceBackupCmd()().(workspaceOpMsg); !msg.success {
		t.Fatalf("backup cmd = %#v", msg)
	}
	if msg := runWorkspaceCleanCmd()().(workspaceOpMsg); !msg.success {
		t.Fatalf("clean cmd = %#v", msg)
	}

	m.Deactivate()
	if m.active {
		t.Fatal("expected deactivate to clear active flag")
	}
}

func TestWorkspaceUpdateCoversKeyAndMessagePaths(t *testing.T) {
	restoreModelDeps(t)

	workspaceBackupFn = func() (string, error) { return "/tmp/demo.tar.gz", nil }
	workspaceCleanFn = func() error { return nil }

	m := NewWorkspaceModel()
	m.loading = false
	m.root = "~/gradient"
	m.total = 100
	m.used = 10
	m.usages = []workspacepkg.Usage{{Name: "outputs", Bytes: 1024}}

	updated, cmd := m.Update(keyRunes("b"))
	if cmd == nil || updated.busyMessage == "" {
		t.Fatal("expected backup command")
	}
	updated.busyMessage = ""
	updated, _ = updated.Update(keyRunes("x"))
	if !updated.confirmClean {
		t.Fatal("expected clean confirmation")
	}
	updated, cmd = updated.Update(keyRunes("y"))
	if cmd == nil || updated.busyMessage == "" {
		t.Fatal("expected clean command")
	}
	updated.active = true
	updated.loadToken = 3
	if _, cmd = updated.Update(workspaceTickMsg{token: 3}); cmd == nil {
		t.Fatal("expected refresh tick command")
	}
	updated, _ = updated.Update(workspaceLoadedMsg{token: 3, root: "~/gradient", total: 100, used: 25, usages: []workspacepkg.Usage{{Name: "models", Bytes: 1 << 20}}})
	if updated.root == "" {
		t.Fatal("expected loaded workspace state")
	}
}
