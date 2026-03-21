package main

import (
	"errors"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func restoreMainDeps(t *testing.T) {
	t.Helper()

	oldTerminalSupported := terminalSupportedFn
	oldDockerRunning := dockerRunningFn
	oldEnsureWorkspace := ensureWorkspaceFn
	oldExitProgram := exitProgram
	oldRunProgram := runProgramFn
	oldVersion := Version

	t.Cleanup(func() {
		terminalSupportedFn = oldTerminalSupported
		dockerRunningFn = oldDockerRunning
		ensureWorkspaceFn = oldEnsureWorkspace
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
	ensureWorkspaceFn = func() error { return nil }

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

func TestRunRejectsPositionalArguments(t *testing.T) {
	restoreMainDeps(t)
	if code := run([]string{"extra"}); code != 1 {
		t.Fatalf("run(extra) = %d, want 1", code)
	}
}

func TestRunRejectsWorkspaceError(t *testing.T) {
	restoreMainDeps(t)
	terminalSupportedFn = func() bool { return true }
	dockerRunningFn = func() (bool, error) { return true, nil }
	ensureWorkspaceFn = func() error { return errors.New("workspace failed") }

	if code := run(nil); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
}

func TestRunRejectsDockerNotRunning(t *testing.T) {
	restoreMainDeps(t)
	terminalSupportedFn = func() bool { return true }
	dockerRunningFn = func() (bool, error) { return false, nil }

	if code := run(nil); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
}

func TestMainUsesRunPath(t *testing.T) {
	restoreMainDeps(t)

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"concave-tui", "--help"}
	main()
}

func TestMainUsesExitProgramOnFailure(t *testing.T) {
	restoreMainDeps(t)

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"concave-tui"}
	terminalSupportedFn = func() bool { return false }
	code := 0
	exitProgram = func(next int) { code = next }

	main()

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestTerminalSupportedRejectsMissingTerm(t *testing.T) {
	t.Setenv("TERM", "")
	if terminalSupported() {
		t.Fatal("expected missing TERM to be rejected")
	}
}

func TestTerminalSupportedRejectsDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if terminalSupported() {
		t.Fatal("expected dumb TERM to be rejected")
	}
}
