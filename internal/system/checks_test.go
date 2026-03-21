package system

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

type mockRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("unexpected command: " + key)
}

type stubConn struct{}

func (stubConn) Read([]byte) (int, error)         { return 0, nil }
func (stubConn) Write(b []byte) (int, error)      { return len(b), nil }
func (stubConn) Close() error                     { return nil }
func (stubConn) LocalAddr() net.Addr              { return nil }
func (stubConn) RemoteAddr() net.Addr             { return nil }
func (stubConn) SetDeadline(time.Time) error      { return nil }
func (stubConn) SetReadDeadline(time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(time.Time) error { return nil }

func TestDockerRunning(t *testing.T) {
	previous := runner
	runner = &mockRunner{outputs: map[string][]byte{"docker info": []byte("ok")}}
	defer func() { runner = previous }()

	ok, err := DockerRunning()
	if err != nil || !ok {
		t.Fatalf("DockerRunning() = %v, %v", ok, err)
	}
}

func TestUserInDockerGroup(t *testing.T) {
	previous := runner
	runner = &mockRunner{outputs: map[string][]byte{"id -nG": []byte("wheel docker audio")}}
	defer func() { runner = previous }()

	ok, err := UserInDockerGroup()
	if err != nil || !ok {
		t.Fatalf("UserInDockerGroup() = %v, %v", ok, err)
	}
}

func TestInternetReachable(t *testing.T) {
	previous := dialContext
	dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return stubConn{}, nil
	}
	defer func() { dialContext = previous }()

	ok, err := InternetReachable()
	if err != nil || !ok {
		t.Fatalf("InternetReachable() = %v, %v", ok, err)
	}
}

func TestOpenURLFallsBackToGio(t *testing.T) {
	previous := runner
	runner = &mockRunner{
		errors:  map[string]error{"xdg-open https://example.com": errors.New("missing")},
		outputs: map[string][]byte{"gio open https://example.com": []byte("ok")},
	}
	defer func() { runner = previous }()

	if err := OpenURL("https://example.com"); err != nil {
		t.Fatalf("OpenURL() error = %v", err)
	}
}

func TestChecksErrorAndPortRegistry(t *testing.T) {
	previousRunner := runner
	previousDialer := dialContext
	previousRegistry := portRegistry
	portRegistry = map[int]portRegistration{}
	t.Cleanup(func() {
		runner = previousRunner
		dialContext = previousDialer
		portRegistry = previousRegistry
	})

	runner = &mockRunner{
		errors: map[string]error{
			"docker info":                  errors.New("down"),
			"id -nG":                       errors.New("id failed"),
			"xdg-open https://example.com": errors.New("missing"),
			"gio open https://example.com": errors.New("missing"),
		},
	}

	if ok, err := DockerRunning(); err == nil || ok {
		t.Fatalf("DockerRunning() = %v, %v, want wrapped error", ok, err)
	}
	if ok, err := UserInDockerGroup(); err == nil || ok {
		t.Fatalf("UserInDockerGroup() = %v, %v, want wrapped error", ok, err)
	}
	dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, errors.New("offline")
	}
	if ok, err := InternetReachable(); err == nil || ok {
		t.Fatalf("InternetReachable() = %v, %v, want wrapped error", ok, err)
	}
	if err := OpenURL("https://example.com"); err == nil {
		t.Fatal("expected OpenURL to fail when both openers fail")
	}

	t.Setenv("HOME", t.TempDir())
	if err := workspace.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if err := config.SaveState(config.State{Installed: []string{"boosting"}}); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	neural, err := suite.Get("neural")
	if err != nil {
		t.Fatalf("suite.Get(neural) error = %v", err)
	}
	conflicts, err := CheckConflicts(neural, []string{"boosting"})
	if err != nil {
		t.Fatalf("CheckConflicts() error = %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts against installed boosting suite")
	}
	if conflicts[0].Port != 8888 {
		t.Fatalf("unexpected first conflict port %d", conflicts[0].Port)
	}
	boosting, err := suite.Get("boosting")
	if err != nil {
		t.Fatalf("suite.Get(boosting) error = %v", err)
	}
	if err := Register(boosting); err != nil {
		t.Fatalf("Register(boosting) error = %v", err)
	}
	if len(portRegistry) == 0 {
		t.Fatal("expected in-memory port registry entries")
	}
	if err := Deregister(boosting); err != nil {
		t.Fatalf("Deregister(boosting) error = %v", err)
	}
	if len(portRegistry) != 0 {
		t.Fatalf("expected empty port registry after deregister, got %d", len(portRegistry))
	}

	path := workspace.ConfigPath("state.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted state at %s: %v", path, err)
	}
}

func TestExecRunnerAndRunCommand(t *testing.T) {
	out, err := runCommand("sh", "-c", "printf ok")
	if err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("runCommand() output = %q", string(out))
	}

	runner := execRunner{}
	out, err = runner.Run("sh", "-c", "printf runner")
	if err != nil {
		t.Fatalf("execRunner.Run() error = %v", err)
	}
	if string(out) != "runner" {
		t.Fatalf("execRunner.Run() output = %q", string(out))
	}
}
