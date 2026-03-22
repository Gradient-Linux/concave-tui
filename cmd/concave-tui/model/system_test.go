package model

import (
	"context"
	"strings"
	"testing"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestSystemModel_OnlyRendersForAdmin(t *testing.T) {
	restoreModelDeps(t)

	m := NewSystemModel()
	m.SetRole(tuiauth.RoleViewer)
	if !strings.Contains(m.View(), "Admin only") {
		t.Fatalf("View() = %q", m.View())
	}

	apiSystemInfoFn = func(ctx context.Context) (apiclient.SystemInfo, error) {
		return apiclient.SystemInfo{Hostname: "gradient", Services: []apiclient.SystemService{{Name: "docker", Status: "running"}}}, nil
	}
	m.SetRole(tuiauth.RoleAdmin)
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd for admin")
	}
}
