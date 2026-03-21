package suite

import (
	"context"
	"errors"
	"testing"

	"github.com/Gradient-Linux/concave-tui/internal/config"
)

func TestInstallRunsHappyPath(t *testing.T) {
	oldLoadState := loadInstalledState
	oldInstalled := suiteInstalled
	oldLoadManifest := loadVersionManifest
	oldSaveManifest := saveVersionManifest
	oldAddSuite := addInstalledSuite
	oldPull := pullImageWithRollback
	oldWriteCompose := writeComposeFile
	oldWriteRaw := writeRawComposeFile
	oldConflicts := checkConflicts
	t.Cleanup(func() {
		loadInstalledState = oldLoadState
		suiteInstalled = oldInstalled
		loadVersionManifest = oldLoadManifest
		saveVersionManifest = oldSaveManifest
		addInstalledSuite = oldAddSuite
		pullImageWithRollback = oldPull
		writeComposeFile = oldWriteCompose
		writeRawComposeFile = oldWriteRaw
		checkConflicts = oldConflicts
	})

	loadInstalledState = func() (config.State, error) { return config.State{}, nil }
	suiteInstalled = func(name string) (bool, error) { return false, nil }
	loadVersionManifest = func() (config.VersionManifest, error) { return config.VersionManifest{}, nil }
	saveVersionManifest = func(manifest config.VersionManifest) error { return nil }
	addInstalledSuite = func(name string) error { return nil }
	pullImageWithRollback = func(ctx context.Context, image string, onProgress func(string)) error { return nil }
	writeComposeFile = func(name string) (string, error) { return "/tmp/" + name + ".compose.yml", nil }
	checkConflicts = func(s Suite, installed []string) ([]PortConflict, error) { return []PortConflict{}, nil }

	if err := Install(context.Background(), "boosting", InstallOptions{}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestInstallStopsOnConflict(t *testing.T) {
	oldLoadState := loadInstalledState
	oldInstalled := suiteInstalled
	oldConflicts := checkConflicts
	oldPull := pullImageWithRollback
	t.Cleanup(func() {
		loadInstalledState = oldLoadState
		suiteInstalled = oldInstalled
		checkConflicts = oldConflicts
		pullImageWithRollback = oldPull
	})

	loadInstalledState = func() (config.State, error) { return config.State{Installed: []string{"flow"}}, nil }
	suiteInstalled = func(name string) (bool, error) { return false, nil }
	checkConflicts = func(s Suite, installed []string) ([]PortConflict, error) {
		return []PortConflict{{Port: 8080, ExistingSuite: "flow", Service: "Airflow"}}, nil
	}
	pullImageWithRollback = func(ctx context.Context, image string, onProgress func(string)) error {
		t.Fatal("pull should not run when conflicts exist")
		return nil
	}

	if err := Install(context.Background(), "neural", InstallOptions{}); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestInstallStopsBeforeComposeOnPullFailure(t *testing.T) {
	oldLoadState := loadInstalledState
	oldInstalled := suiteInstalled
	oldLoadManifest := loadVersionManifest
	oldSaveManifest := saveVersionManifest
	oldAddSuite := addInstalledSuite
	oldPull := pullImageWithRollback
	oldWriteCompose := writeComposeFile
	oldConflicts := checkConflicts
	t.Cleanup(func() {
		loadInstalledState = oldLoadState
		suiteInstalled = oldInstalled
		loadVersionManifest = oldLoadManifest
		saveVersionManifest = oldSaveManifest
		addInstalledSuite = oldAddSuite
		pullImageWithRollback = oldPull
		writeComposeFile = oldWriteCompose
		checkConflicts = oldConflicts
	})

	loadInstalledState = func() (config.State, error) { return config.State{}, nil }
	suiteInstalled = func(name string) (bool, error) { return false, nil }
	loadVersionManifest = func() (config.VersionManifest, error) { return config.VersionManifest{}, nil }
	saveVersionManifest = func(manifest config.VersionManifest) error { return nil }
	addInstalledSuite = func(name string) error { return nil }
	checkConflicts = func(s Suite, installed []string) ([]PortConflict, error) { return []PortConflict{}, nil }
	pullCalls := 0
	pullImageWithRollback = func(ctx context.Context, image string, onProgress func(string)) error {
		pullCalls++
		if pullCalls == 2 {
			return errors.New("pull failed")
		}
		return nil
	}
	writeComposeFile = func(name string) (string, error) {
		t.Fatal("compose write should not run after pull failure")
		return "", nil
	}

	if err := Install(context.Background(), "boosting", InstallOptions{}); err == nil {
		t.Fatal("expected pull failure")
	}
}

func TestInstallAlreadyInstalledCanShortCircuit(t *testing.T) {
	oldInstalled := suiteInstalled
	oldPull := pullImageWithRollback
	t.Cleanup(func() {
		suiteInstalled = oldInstalled
		pullImageWithRollback = oldPull
	})

	suiteInstalled = func(name string) (bool, error) { return true, nil }
	pullImageWithRollback = func(ctx context.Context, image string, onProgress func(string)) error {
		t.Fatal("pull should not run when suite is already installed")
		return nil
	}

	if err := Install(context.Background(), "boosting", InstallOptions{}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestSetConflictCheckerOverridesAndResets(t *testing.T) {
	oldConflicts := checkConflicts
	t.Cleanup(func() { checkConflicts = oldConflicts })

	SetConflictChecker(func(s Suite, installed []string) ([]PortConflict, error) {
		return []PortConflict{{Port: 5000, ExistingSuite: "flow", NewSuite: s.Name, Service: "MLflow"}}, nil
	})
	conflicts, err := checkConflicts(Registry["boosting"], []string{"flow"})
	if err != nil {
		t.Fatalf("checkConflicts() error = %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].ExistingSuite != "flow" {
		t.Fatalf("checkConflicts() = %#v", conflicts)
	}

	SetConflictChecker(nil)
	conflicts, err = checkConflicts(Registry["boosting"], []string{"flow"})
	if err != nil {
		t.Fatalf("checkConflicts() reset error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected reset checker to return no conflicts, got %#v", conflicts)
	}
}

func TestInstallTargetAndWriteSelectedCompose(t *testing.T) {
	oldPrompt := promptChecklist
	oldWriteCompose := writeComposeFile
	oldWriteRaw := writeRawComposeFile
	t.Cleanup(func() {
		promptChecklist = oldPrompt
		writeComposeFile = oldWriteCompose
		writeRawComposeFile = oldWriteRaw
	})

	target, selection, err := installTarget("boosting")
	if err != nil {
		t.Fatalf("installTarget(boosting) error = %v", err)
	}
	if target.Name != "boosting" || len(selection.Containers) != 0 {
		t.Fatalf("unexpected boosting target %#v %#v", target, selection)
	}

	composeWrites := 0
	writeComposeFile = func(name string) (string, error) {
		composeWrites++
		if name != "boosting" {
			t.Fatalf("writeComposeFile() suite = %q", name)
		}
		return "/tmp/boosting.compose.yml", nil
	}
	if err := writeSelectedCompose(Registry["boosting"]); err != nil {
		t.Fatalf("writeSelectedCompose(boosting) error = %v", err)
	}
	if composeWrites != 1 {
		t.Fatalf("expected one compose write, got %d", composeWrites)
	}

	promptChecklist = func(items []string) []string {
		return []string{
			"Boosting | Classical ML stack (~1 GB)",
			"Boosting | JupyterLab (~1 GB, shared with Neural)",
		}
	}
	target, selection, err = installTarget("forge")
	if err != nil {
		t.Fatalf("installTarget(forge) error = %v", err)
	}
	if target.Name != "forge" || len(selection.Containers) == 0 || len(target.Containers) == 0 {
		t.Fatalf("unexpected forge target %#v %#v", target, selection)
	}

	rawWrites := 0
	writeRawComposeFile = func(name string, data []byte) (string, error) {
		rawWrites++
		if name != "forge" {
			t.Fatalf("writeRawComposeFile() suite = %q", name)
		}
		if len(data) == 0 {
			t.Fatal("expected forge compose data")
		}
		return "/tmp/forge.compose.yml", nil
	}
	if err := writeSelectedCompose(target); err != nil {
		t.Fatalf("writeSelectedCompose(forge) error = %v", err)
	}
	if rawWrites != 1 {
		t.Fatalf("expected one forge raw compose write, got %d", rawWrites)
	}
}
