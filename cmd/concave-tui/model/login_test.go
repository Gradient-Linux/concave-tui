package model

import (
	"context"
	"errors"
	"testing"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
)

func TestLoginModel_SuccessAndFailure(t *testing.T) {
	restoreModelDeps(t)

	apiLoginFn = func(ctx context.Context, username, password string) (tuiauth.Session, error) {
		if password != "secret" {
			return tuiauth.Session{}, errors.New("invalid credentials")
		}
		return tuiauth.Session{Token: "token", Username: username, Role: tuiauth.RoleViewer}, nil
	}

	m := NewLoginModel()
	m.usernameInput.SetValue("alice")
	m.passwordInput.SetValue("wrong")
	updated, cmd := m.Update(keyRunes("enter"))
	if cmd == nil || !updated.loading {
		t.Fatal("expected login command to start")
	}

	updated, _ = updated.Update(loginFailureMsg{err: errors.New("invalid credentials")})
	if updated.attempts != 1 {
		t.Fatalf("attempts = %d", updated.attempts)
	}

	updated.passwordInput.SetValue("secret")
	updated, cmd = updated.Update(keyRunes("enter"))
	if cmd == nil {
		t.Fatal("expected second login command")
	}
}
