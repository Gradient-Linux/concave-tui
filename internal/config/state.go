package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

// State tracks installed suites and the last mutation time.
type State struct {
	Installed   []string  `json:"installed"`
	LastUpdated time.Time `json:"last_updated"`
}

// LoadState reads ~/gradient/config/state.json or returns an empty state when missing.
func LoadState() (State, error) {
	if err := workspace.EnsureLayout(); err != nil {
		return State{}, err
	}

	path := workspace.ConfigPath("state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Installed: []string{}}, nil
		}
		return State{}, fmt.Errorf("read %s: %w", path, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if state.Installed == nil {
		state.Installed = []string{}
	}
	sort.Strings(state.Installed)
	return state, nil
}

// SaveState writes ~/gradient/config/state.json atomically.
func SaveState(state State) error {
	sort.Strings(state.Installed)
	if state.LastUpdated.IsZero() {
		state.LastUpdated = time.Now().UTC()
	}
	return writeJSONAtomically(workspace.ConfigPath("state.json"), state)
}

// AddSuite records a suite as installed.
func AddSuite(name string) error {
	state, err := LoadState()
	if err != nil {
		return err
	}
	for _, installed := range state.Installed {
		if installed == name {
			state.LastUpdated = time.Now().UTC()
			return SaveState(state)
		}
	}
	state.Installed = append(state.Installed, name)
	state.LastUpdated = time.Now().UTC()
	return SaveState(state)
}

// RemoveSuite removes a suite from the installed set.
func RemoveSuite(name string) error {
	state, err := LoadState()
	if err != nil {
		return err
	}

	filtered := state.Installed[:0]
	for _, installed := range state.Installed {
		if installed != name {
			filtered = append(filtered, installed)
		}
	}
	state.Installed = filtered
	state.LastUpdated = time.Now().UTC()
	return SaveState(state)
}

// IsInstalled reports whether a suite is already installed.
func IsInstalled(name string) (bool, error) {
	state, err := LoadState()
	if err != nil {
		return false, err
	}
	for _, installed := range state.Installed {
		if installed == name {
			return true, nil
		}
	}
	return false, nil
}

func writeJSONAtomically(path string, value any) error {
	if err := workspace.EnsureLayout(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmp, path, err)
	}
	return nil
}
