package model

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

func TestWorkspaceUpdateCleanCancel(t *testing.T) {
	m := NewWorkspaceModel()
	m.confirmClean = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.confirmClean {
		t.Fatal("expected clean confirmation to cancel")
	}
}

func TestWorkspaceUpdateBackupCompletion(t *testing.T) {
	m := NewWorkspaceModel()
	updated, _ := m.Update(workspaceOpMsg{kind: "backup", success: true, detail: "backup created"})
	if updated.completionNote != "backup created" {
		t.Fatalf("completionNote = %q", updated.completionNote)
	}
}

func TestWorkspaceUsageBar(t *testing.T) {
	m := NewWorkspaceModel()
	m.total = 100
	m.used = 90

	if got := m.usageBar(); !strings.Contains(got, "90%") {
		t.Fatalf("usageBar() = %q", got)
	}
}

func TestLoadWorkspaceCmdReturnsUsageData(t *testing.T) {
	restoreModelDeps(t)

	tmp := t.TempDir()
	workspaceRootFn = func() string { return tmp }
	workspaceEnsureFn = func() error { return nil }
	workspaceStatusFn = func() ([]workspace.Usage, error) {
		return []workspace.Usage{
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
	m.usages = []string{"models 1.0 GB"}
	if view := m.View(); view == "" {
		t.Fatal("expected workspace view")
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
	m.usages = []string{"models 1.0 GB"}

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
	updated, _ = updated.Update(workspaceLoadedMsg{token: 3, root: "~/gradient", total: 100, used: 25, usages: []string{"models 1 GB"}})
	if updated.root == "" {
		t.Fatal("expected loaded workspace state")
	}
}
