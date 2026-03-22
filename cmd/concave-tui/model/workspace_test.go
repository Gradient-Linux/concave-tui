package model

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
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

func TestLoadWorkspaceCmdReturnsUsageData(t *testing.T) {
	restoreModelDeps(t)

	apiWorkspaceFn = func(ctx context.Context) (apiclient.WorkspacePayload, error) {
		return apiclient.WorkspacePayload{
			Root:  "~/gradient",
			Total: 100,
			Used:  25,
			Usages: map[string]uint64{
				"models":    2048,
				"notebooks": 1024,
			},
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

func TestWorkspaceActivateAndView(t *testing.T) {
	restoreModelDeps(t)

	apiWorkspaceFn = func(ctx context.Context) (apiclient.WorkspacePayload, error) {
		return apiclient.WorkspacePayload{
			Root: "~/gradient",
			Total: 100,
			Used:  25,
			Usages: map[string]uint64{
				"models": 1024,
			},
		}, nil
	}

	m := NewWorkspaceModel()
	m.role = tuiauth.RoleOperator
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	m.loading = false
	m.loaded = true
	m.root = "~/gradient"
	m.total = 100
	m.used = 25
	m.cpuUsage = 42
	m.usages = []workspacepkg.Usage{{Name: "models", Bytes: 1024}}
	m.lastBackup = time.Now().Add(-2 * time.Hour)
	if view := m.View(); view == "" || !strings.Contains(view, "clean outputs") {
		t.Fatalf("expected workspace view, got %q", view)
	}
}

func TestWorkspaceActionsUseJobs(t *testing.T) {
	restoreModelDeps(t)

	apiWorkspaceBackupFn = func(ctx context.Context) (string, error) { return "job-1", nil }
	apiWorkspaceCleanFn = func(ctx context.Context) (string, error) { return "job-2", nil }
	jobCalls := 0
	apiJobFn = func(ctx context.Context, id string) (apiclient.JobSnapshot, error) {
		jobCalls++
		if id == "job-1" {
			return apiclient.JobSnapshot{ID: id, Status: "completed", Result: map[string]any{"path": "/tmp/backup.tar.gz"}}, nil
		}
		return apiclient.JobSnapshot{ID: id, Status: "completed"}, nil
	}

	if msg := runWorkspaceBackupCmd()().(workspaceOpMsg); !msg.success {
		t.Fatalf("backup cmd = %#v", msg)
	}
	if msg := runWorkspaceCleanCmd()().(workspaceOpMsg); !msg.success {
		t.Fatalf("clean cmd = %#v", msg)
	}
	if jobCalls < 2 {
		t.Fatalf("job calls = %d", jobCalls)
	}
}
