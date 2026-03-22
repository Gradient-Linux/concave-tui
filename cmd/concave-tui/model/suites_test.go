package model

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestLoadSuitesCmdReflectsAPIState(t *testing.T) {
	restoreModelDeps(t)

	apiSuitesFn = func(ctx context.Context) ([]apiclient.SuiteSummary, error) {
		return []apiclient.SuiteSummary{
			{
				Name:      "boosting",
				Installed: true,
				State:     "running",
				Containers: []apiclient.ContainerInfo{
					{Name: "gradient-boost-core", Image: "python:3.12-slim", Status: "running", Current: "python:3.12-slim"},
				},
			},
			{
				Name:      "forge",
				Installed: true,
				State:     "unconfigured",
				Error:     "forge has no recorded component selection",
			},
		}, nil
	}

	msg := loadSuitesCmd()().(suitesLoadedMsg)
	if msg.err != nil {
		t.Fatalf("loadSuitesCmd() error = %v", msg.err)
	}
	if msg.rows[0].State != "● running" {
		t.Fatalf("boosting state = %q", msg.rows[0].State)
	}
	if msg.rows[1].State != "⚠ unconfigured" {
		t.Fatalf("forge state = %q", msg.rows[1].State)
	}
}

func TestPerformInstallUsesServerJob(t *testing.T) {
	restoreModelDeps(t)

	started := ""
	apiSuiteActionFn = func(ctx context.Context, name, action string, body any) (string, error) {
		started = name + ":" + action
		return "job-1", nil
	}
	jobCalls := 0
	apiJobFn = func(ctx context.Context, id string) (apiclient.JobSnapshot, error) {
		jobCalls++
		if jobCalls == 1 {
			return apiclient.JobSnapshot{ID: id, Status: "running", Lines: []string{"pulling image"}}, nil
		}
		return apiclient.JobSnapshot{ID: id, Status: "completed", Lines: []string{"pulling image", "done"}}, nil
	}

	var lines []string
	if err := performInstall("boosting", nil, func(line string) { lines = append(lines, line) }); err != nil {
		t.Fatalf("performInstall() error = %v", err)
	}
	if started != "boosting:install" {
		t.Fatalf("started = %q", started)
	}
	if len(lines) == 0 {
		t.Fatal("expected streamed job lines")
	}
}

func TestPerformInstallForgeSendsSelectionKeys(t *testing.T) {
	restoreModelDeps(t)

	var gotBody any
	apiSuiteActionFn = func(ctx context.Context, name, action string, body any) (string, error) {
		gotBody = body
		return "job-1", nil
	}
	apiJobFn = func(ctx context.Context, id string) (apiclient.JobSnapshot, error) {
		return apiclient.JobSnapshot{ID: id, Status: "completed"}, nil
	}

	if err := performInstall("forge", []string{"boosting-core", "neural-infer"}, func(string) {}); err != nil {
		t.Fatalf("performInstall() error = %v", err)
	}

	payload, ok := gotBody.(map[string][]string)
	if !ok {
		t.Fatalf("body type = %T", gotBody)
	}
	if len(payload["forge_components"]) != 2 {
		t.Fatalf("forge_components = %#v", payload["forge_components"])
	}
}

func TestRunServerSuiteJobReturnsFailure(t *testing.T) {
	restoreModelDeps(t)

	apiSuiteActionFn = func(ctx context.Context, name, action string, body any) (string, error) {
		return "job-1", nil
	}
	apiJobFn = func(ctx context.Context, id string) (apiclient.JobSnapshot, error) {
		return apiclient.JobSnapshot{ID: id, Status: "failed", Error: "suite failed"}, nil
	}

	err := runServerSuiteJob("boosting", "start", nil, nil, func(string, int, int) {})
	if err == nil || !strings.Contains(err.Error(), "suite failed") {
		t.Fatalf("runServerSuiteJob() error = %v", err)
	}
}

func TestSuitesModelViewerCannotInstall(t *testing.T) {
	m := NewSuitesModel()
	m.role = tuiauth.RoleViewer
	m.rows = []suiteRow{{Name: "boosting", Installed: false}}

	updated, cmd := m.Update(keyRunes("i"))
	if cmd != nil {
		t.Fatal("viewer should not start install command")
	}
	if updated.forgePrompt {
		t.Fatal("viewer should not open forge picker")
	}
}

func TestSuitesModelDeveloperCanOpenLab(t *testing.T) {
	m := NewSuitesModel()
	m.role = tuiauth.RoleDeveloper
	lines := m.detailView(suiteRow{
		Name:      "boosting",
		Installed: true,
		Summary: apiclient.SuiteSummary{
			Name: "boosting",
			Containers: []apiclient.ContainerInfo{
				{Name: "gradient-boost-lab", Status: "running", Image: "lab"},
			},
		},
	})
	if !strings.Contains(strings.Join(lines, "\n"), "[l]lab") {
		t.Fatal("developer should see lab action")
	}
}

func TestSuitesModelOperatorCanInstall(t *testing.T) {
	m := NewSuitesModel()
	m.role = tuiauth.RoleOperator
	lines := m.detailView(suiteRow{
		Name:      "boosting",
		Installed: false,
		Summary:   apiclient.SuiteSummary{Name: "boosting"},
	})
	if !strings.Contains(strings.Join(lines, "\n"), "[i]install") {
		t.Fatal("operator should see install action")
	}
}

func TestSuitesDetailViewShowsRemoveForProblemState(t *testing.T) {
	m := NewSuitesModel()
	m.role = tuiauth.RoleOperator
	lines := m.detailView(suiteRow{
		Name:      "forge",
		Installed: true,
		State:     "⚠ unconfigured",
		Problem:   "forge has no recorded component selection",
	})
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "[r]remove stale install") {
		t.Fatalf("detailView() = %q", rendered)
	}
}

func TestSuitesViewKeepsListVisibleWhenLastErrSet(t *testing.T) {
	m := NewSuitesModel()
	m.role = tuiauth.RoleOperator
	m.loading = false
	m.width = 120
	m.lastErr = errors.New("port 8888 is reserved by an installed forge suite")
	m.rows = []suiteRow{
		{Name: "boosting", Installed: false, State: "— not installed"},
		{Name: "forge", Installed: true, State: "⚠ unconfigured", Problem: "Forge is marked installed but its component selection was not saved."},
	}
	m.selected = 1

	rendered := m.View()
	if !strings.Contains(rendered, "forge") || !strings.Contains(rendered, "[r]remove stale install") {
		t.Fatalf("View() = %q", rendered)
	}
	if !strings.Contains(rendered, "port 8888 is reserved") {
		t.Fatalf("View() missing error detail: %q", rendered)
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
