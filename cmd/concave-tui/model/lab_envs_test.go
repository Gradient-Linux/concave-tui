package model

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestFormatTTLRendersRemainingTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		expires time.Time
		want    string
	}{
		"hours":   {base.Add(3*time.Hour + 15*time.Minute), "3h 15m"},
		"minutes": {base.Add(15*time.Minute + 7*time.Second), "15m 07s"},
		"seconds": {base.Add(42 * time.Second), "42s"},
		"expired": {base.Add(-1 * time.Minute), "expired"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := formatTTL(base, tc.expires); got != tc.want {
				t.Fatalf("formatTTL(%v) = %q, want %q", tc.expires, got, tc.want)
			}
		})
	}
}

func TestLabEnvsModelRendersTableAndBlocksViewers(t *testing.T) {
	restoreModelDeps(t)

	restoreLabEnvs := apiLabEnvsFn
	t.Cleanup(func() { apiLabEnvsFn = restoreLabEnvs })
	expires := time.Now().Add(2 * time.Hour)
	apiLabEnvsFn = func(ctx context.Context) (apiclient.LabEnvsResponse, error) {
		return apiclient.LabEnvsResponse{
			Envs: []apiclient.LabEnv{
				{ID: "env-aaa", Driver: "docker", Owner: "alice", Image: "jupyter/min", Status: "running", ExpiresAt: expires},
			},
			Storage: apiclient.LabStorage{HotTier: "/hot", ColdTier: "/cold"},
			Drivers: []string{"docker"},
			Active:  "docker",
		}, nil
	}

	m := NewLabEnvsModel()
	m.SetRole(tuiauth.RoleDeveloper)
	m.Activate()
	msg := loadLabEnvsCmd(m.token)()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "env-aaa") || !strings.Contains(view, "docker") {
		t.Fatalf("expected env row in view, got: %q", view)
	}
	if !strings.Contains(view, "/cold") {
		t.Fatalf("expected cold tier in header, got: %q", view)
	}

}

func TestLabEnvsExtendRequiresDeveloper(t *testing.T) {
	restoreModelDeps(t)

	restoreExtend := apiLabExtendFn
	t.Cleanup(func() { apiLabExtendFn = restoreExtend })
	called := 0
	apiLabExtendFn = func(ctx context.Context, id string, seconds int) (apiclient.LabEnv, error) {
		called++
		if id != "env-bbb" || seconds != 3600 {
			return apiclient.LabEnv{}, errors.New("unexpected args")
		}
		return apiclient.LabEnv{ID: id}, nil
	}

	m := NewLabEnvsModel()
	m.SetRole(tuiauth.RoleViewer)
	m.response = apiclient.LabEnvsResponse{Envs: []apiclient.LabEnv{{ID: "env-bbb", Driver: "docker", Status: "running"}}}

	m, _ = m.Update(keyRunes("e"))
	if called != 0 {
		t.Fatalf("viewer should not trigger extend, called=%d", called)
	}

	m.SetRole(tuiauth.RoleDeveloper)
	_, cmd := m.Update(keyRunes("e"))
	if cmd == nil {
		t.Fatalf("developer extend should produce a command")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("extend command returned nil msg")
	}
	if called != 1 {
		t.Fatalf("extend called=%d, want 1", called)
	}
}
