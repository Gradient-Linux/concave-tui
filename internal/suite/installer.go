package suite

import (
	"context"
	"fmt"

	"github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/docker"
	"github.com/Gradient-Linux/concave-tui/internal/ui"
)

// PortConflict captures a suite install conflict without importing internal/system.
type PortConflict struct {
	Port          int
	ExistingSuite string
	NewSuite      string
	Service       string
}

// InstallOptions controls suite installation behavior.
type InstallOptions struct {
	GPUAvailable bool
	Force        bool
}

var checkConflicts = func(Suite, []string) ([]PortConflict, error) {
	return []PortConflict{}, nil
}

var (
	loadInstalledState    = config.LoadState
	suiteInstalled        = config.IsInstalled
	loadVersionManifest   = config.LoadManifest
	saveVersionManifest   = config.SaveManifest
	addInstalledSuite     = config.AddSuite
	recordSuiteInstall    = config.RecordInstall
	pullImageWithRollback = docker.PullWithRollbackSafety
	writeComposeFile      = docker.WriteCompose
	writeRawComposeFile   = docker.WriteRawCompose
)

// SetConflictChecker wires the port-conflict implementation without introducing a package cycle.
func SetConflictChecker(fn func(Suite, []string) ([]PortConflict, error)) {
	if fn == nil {
		checkConflicts = func(Suite, []string) ([]PortConflict, error) { return []PortConflict{}, nil }
		return
	}
	checkConflicts = fn
}

// Install runs the suite installation flow for a named suite.
func Install(ctx context.Context, suiteName string, opts InstallOptions) error {
	selectedSuite, _, err := installTarget(suiteName)
	if err != nil {
		return fmt.Errorf("step 1 validate suite: %w", err)
	}

	installed, err := suiteInstalled(selectedSuite.Name)
	if err != nil {
		return fmt.Errorf("step 2 check installed state: %w", err)
	}
	if installed && !opts.Force {
		ui.Info("Install", selectedSuite.Name+" already installed")
		return nil
	}

	if selectedSuite.GPURequired && !opts.GPUAvailable {
		ui.Warn("GPU", selectedSuite.Name+" benefits from NVIDIA support; continuing without GPU")
	}

	state, err := loadInstalledState()
	if err != nil {
		return fmt.Errorf("step 4 load installed suites: %w", err)
	}
	conflicts, err := checkConflicts(selectedSuite, state.Installed)
	if err != nil {
		return fmt.Errorf("step 4 check port conflicts: %w", err)
	}
	if len(conflicts) > 0 {
		for _, conflict := range conflicts {
			ui.Fail("Port conflict", fmt.Sprintf("%d owned by %s (%s)", conflict.Port, conflict.ExistingSuite, conflict.Service))
		}
		return fmt.Errorf("step 4 check port conflicts: conflicts detected for %s", selectedSuite.Name)
	}

	spinner := ui.NewSpinner("Pulling images")
	spinner.Start()
	for _, container := range selectedSuite.Containers {
		ui.Info("Pulling", container.Image)
		if err := pullImageWithRollback(ctx, container.Image, nil); err != nil {
			spinner.Stop("")
			return fmt.Errorf("step 5 pull images: %w", err)
		}
	}
	spinner.Stop("images ready")

	if err := writeSelectedCompose(selectedSuite); err != nil {
		return fmt.Errorf("step 6 write compose file: %w", err)
	}

	manifest, err := loadVersionManifest()
	if err != nil {
		return fmt.Errorf("step 7 load manifest: %w", err)
	}
	manifest = recordSuiteInstall(manifest, selectedSuite)
	if err := saveVersionManifest(manifest); err != nil {
		return fmt.Errorf("step 7 save manifest: %w", err)
	}

	if err := addInstalledSuite(selectedSuite.Name); err != nil {
		return fmt.Errorf("step 8 update state: %w", err)
	}

	ui.Pass("Install", selectedSuite.Name+" installed successfully")
	return nil
}

func installTarget(name string) (Suite, ForgeSelection, error) {
	base, err := Get(name)
	if err != nil {
		return Suite{}, ForgeSelection{}, err
	}
	if name != "forge" {
		return base, ForgeSelection{}, nil
	}

	selection, err := PickComponents()
	if err != nil {
		return Suite{}, ForgeSelection{}, err
	}
	base.Containers = selection.Containers
	base.Ports = selection.Ports
	base.Volumes = selection.Volumes
	return base, selection, nil
}

func writeSelectedCompose(selectedSuite Suite) error {
	if selectedSuite.Name != "forge" {
		_, err := writeComposeFile(selectedSuite.Name)
		return err
	}

	selection := ForgeSelection{
		Containers: selectedSuite.Containers,
		Ports:      selectedSuite.Ports,
		Volumes:    selectedSuite.Volumes,
	}
	data, err := BuildForgeCompose(selection)
	if err != nil {
		return err
	}
	_, err = writeRawComposeFile(selectedSuite.Name, data)
	return err
}
