package model

import (
	"context"
	"strings"
	"testing"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestUsersModel_OnlyRendersForAdmin(t *testing.T) {
	restoreModelDeps(t)

	m := NewUsersModel()
	m.SetRole(tuiauth.RoleViewer)
	if !strings.Contains(m.View(), "Admin only") {
		t.Fatalf("View() = %q", m.View())
	}

	apiUsersActivityFn = func(ctx context.Context) ([]apiclient.UserActivity, error) {
		return []apiclient.UserActivity{
			{Username: "alice", Role: tuiauth.RoleAdmin, Containers: []apiclient.ActivityContainer{{Name: "boosting"}}, GPUMemoryMiB: 1024},
		}, nil
	}
	m.SetRole(tuiauth.RoleAdmin)
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd for admin")
	}
}
