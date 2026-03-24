package model

import (
	"context"
	"strings"
	"testing"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestEnvironmentViewShowsUnavailablePlaceholder(t *testing.T) {
	restoreModelDeps(t)

	apiResolverStatusFn = func(ctx context.Context) (apiclient.ResolverStatus, error) {
		return apiclient.ResolverStatus{
			Available: ptrBool(false),
			Message:   "resolver not configured",
		}, nil
	}

	m := NewEnvironmentModel()
	m.SetRole(tuiauth.RoleViewer)
	m.token = 1
	updated, _ := m.Update(loadEnvironmentCmd(1)())
	if !updated.unavailable {
		t.Fatal("expected unavailable state")
	}
	if got := updated.View(); !strings.Contains(got, "Resolver unavailable") {
		t.Fatalf("View() = %q", got)
	}
}

func TestFleetViewShowsUnavailablePlaceholder(t *testing.T) {
	restoreModelDeps(t)

	apiNodeStatusFn = func(ctx context.Context) (apiclient.FleetNode, error) {
		return apiclient.FleetNode{}, nil
	}
	apiFleetStatusFn = func(ctx context.Context) (apiclient.FleetResponse, error) {
		return apiclient.FleetResponse{
			Available: ptrBool(false),
			Message:   "mesh not configured",
		}, nil
	}

	m := NewFleetModel()
	m.SetRole(tuiauth.RoleViewer)
	m.token = 1
	updated, _ := m.Update(loadFleetCmd(1)())
	if !updated.unavailable {
		t.Fatal("expected unavailable state")
	}
	if got := updated.View(); !strings.Contains(got, "Mesh unavailable") {
		t.Fatalf("View() = %q", got)
	}
}

func TestTeamsViewShowsUnavailablePlaceholder(t *testing.T) {
	restoreModelDeps(t)

	apiTeamsFn = func(ctx context.Context) (apiclient.TeamsResponse, error) {
		return apiclient.TeamsResponse{
			Available: ptrBool(false),
			Message:   "team management is not yet implemented",
		}, nil
	}

	m := NewTeamsModel()
	m.SetRole(tuiauth.RoleAdmin)
	m.token = 1
	updated, _ := m.Update(loadTeamsCmd(1)())
	if !updated.unavailable {
		t.Fatal("expected unavailable state")
	}
	if got := updated.View(); !strings.Contains(got, "Team management unavailable") {
		t.Fatalf("View() = %q", got)
	}
}

func TestRootHelpIncludesNewViews(t *testing.T) {
	restoreModelDeps(t)

	m := NewRootModel("dev", testConfig(), authSession(tuiauth.RoleAdmin))
	help := m.activeHelp()
	if !strings.Contains(help, "Environment") || !strings.Contains(help, "Fleet") {
		t.Fatalf("activeHelp() = %q", help)
	}
}

func ptrBool(v bool) *bool {
	return &v
}
