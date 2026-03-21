package main

import (
	"errors"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
)

func restoreMainDeps(t *testing.T) {
	t.Helper()

	oldTerminalSupported := terminalSupportedFn
	oldDockerRunning := dockerRunningFn
	oldLoadConfig := loadConfigFn
	oldExitProgram := exitProgram
	oldRunProgram := runProgramFn
	oldVersion := Version

	t.Cleanup(func() {
		terminalSupportedFn = oldTerminalSupported
		dockerRunningFn = oldDockerRunning
		loadConfigFn = oldLoadConfig
		exitProgram = oldExitProgram
		runProgramFn = oldRunProgram
		Version = oldVersion
	})
}

func TestRunHelpAndVersion(t *testing.T) {
	restoreMainDeps(t)

	if code := run([]string{"--help"}); code != 0 {
		t.Fatalf("run(--help) = %d, want 0", code)
	}

	Version = "v1.2.3"
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("run(--version) = %d, want 0", code)
	}
}

func TestRunRejectsUnsupportedTerminal(t *testing.T) {
	restoreMainDeps(t)
	terminalSupportedFn = func() bool { return false }

	if code := run(nil); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
}

func TestRunRejectsDockerUnavailable(t *testing.T) {
	restoreMainDeps(t)
	terminalSupportedFn = func() bool { return true }
	dockerRunningFn = func() (bool, error) { return false, errors.New("down") }

	if code := run(nil); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
}

func TestRunLaunchesProgram(t *testing.T) {
	restoreMainDeps(t)

	terminalSupportedFn = func() bool { return true }
	dockerRunningFn = func() (bool, error) { return true, nil }
	loadConfigFn = func() (tuiconfig.Config, error) { return tuiconfig.DefaultConfig(), nil }

	called := false
	runProgramFn = func(root tea.Model) error {
		called = true
		return nil
	}

	if code := run(nil); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if !called {
		t.Fatal("expected runProgramFn to be called")
	}
}

func TestRunRejectsPositionalArgumentsAndConfigError(t *testing.T) {
	restoreMainDeps(t)
	if code := run([]string{"extra"}); code != 1 {
		t.Fatalf("run(extra) = %d, want 1", code)
	}

	terminalSupportedFn = func() bool { return true }
	dockerRunningFn = func() (bool, error) { return true, nil }
	loadConfigFn = func() (tuiconfig.Config, error) { return tuiconfig.Config{}, errors.New("config failed") }
	if code := run(nil); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
}

func TestMainUsesRunPathAndExitProgram(t *testing.T) {
	restoreMainDeps(t)

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"concave-tui", "--help"}
	main()

	os.Args = []string{"concave-tui"}
	terminalSupportedFn = func() bool { return false }
	code := 0
	exitProgram = func(next int) { code = next }
	main()
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestTerminalSupportedRejectsMissingOrDumbTERM(t *testing.T) {
	t.Setenv("TERM", "")
	if terminalSupported() {
		t.Fatal("expected missing TERM to be rejected")
	}

	t.Setenv("TERM", "dumb")
	if terminalSupported() {
		t.Fatal("expected dumb TERM to be rejected")
	}
}
