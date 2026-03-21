package config

import (
	"os"
	"testing"

	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

type testInstallRecord struct {
	name   string
	images map[string]string
}

func (t testInstallRecord) RecordName() string {
	return t.name
}

func (t testInstallRecord) RecordImages() map[string]string {
	return t.images
}

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ok, err := IsInstalled("boosting")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if ok {
		t.Fatal("expected empty state to report not installed")
	}

	if err := AddSuite("boosting"); err != nil {
		t.Fatalf("AddSuite() error = %v", err)
	}
	if err := AddSuite("boosting"); err != nil {
		t.Fatalf("AddSuite() second call error = %v", err)
	}

	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(state.Installed) != 1 || state.Installed[0] != "boosting" {
		t.Fatalf("unexpected state %#v", state)
	}
	if state.LastUpdated.IsZero() {
		t.Fatal("expected last_updated to be set")
	}

	if err := RemoveSuite("flow"); err != nil {
		t.Fatalf("RemoveSuite(non-installed) error = %v", err)
	}
	if err := RemoveSuite("boosting"); err != nil {
		t.Fatalf("RemoveSuite() error = %v", err)
	}
	ok, err = IsInstalled("boosting")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if ok {
		t.Fatal("expected boosting to be removed")
	}
}

func TestManifestRoundTripAndRollback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := workspace.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	manifest := RecordInstall(VersionManifest{}, testInstallRecord{
		name: "boosting",
		images: map[string]string{
			"gradient-boost-core": "python:3.12-slim",
		},
	})
	manifest = RecordUpdate(manifest, "boosting", "gradient-boost-core", "python:3.12-alpine")
	if err := SaveManifest(manifest); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	loaded, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if loaded["boosting"]["gradient-boost-core"].Previous != "python:3.12-slim" {
		t.Fatalf("unexpected previous tag %#v", loaded["boosting"]["gradient-boost-core"])
	}

	swapped, err := SwapForRollback(loaded, "boosting")
	if err != nil {
		t.Fatalf("SwapForRollback() error = %v", err)
	}
	version := swapped["boosting"]["gradient-boost-core"]
	if version.Current != "python:3.12-slim" || version.Previous != "python:3.12-alpine" {
		t.Fatalf("unexpected swapped version %#v", version)
	}
}

func TestSwapForRollbackRequiresPrevious(t *testing.T) {
	manifest := RecordInstall(VersionManifest{}, testInstallRecord{
		name: "boosting",
		images: map[string]string{
			"gradient-boost-core": "python:3.12-slim",
		},
	})
	if _, err := SwapForRollback(manifest, "boosting"); err == nil || err.Error() != "no previous version for container gradient-boost-core — run concave update first" {
		t.Fatalf("SwapForRollback() error = %v", err)
	}
}

func TestSaveManifestCreatesProtectedConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveManifest(VersionManifest{}); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}
	info, err := os.Stat(workspace.ConfigPath(""))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("config dir mode = %#o, want 0700", got)
	}
}
