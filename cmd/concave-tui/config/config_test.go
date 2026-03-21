package config

import (
	"os"
	"path/filepath"
	"testing"

	workspacepkg "github.com/Gradient-Linux/concave-tui/internal/workspace"
)

func TestLoadWritesDefaultsOnFirstRun(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ActivePreset != "default" {
		t.Fatalf("ActivePreset = %q", cfg.ActivePreset)
	}

	if _, err := os.Stat(filepath.Join(xdg, "concave-tui", "config.toml")); err != nil {
		t.Fatalf("missing xdg config: %v", err)
	}
	if _, err := os.Stat(workspacepkg.ConfigPath("concave-tui.toml")); err != nil {
		t.Fatalf("missing preset config: %v", err)
	}
}

func TestLoadMergesXDGAndWorkspacePresets(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := workspacepkg.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(xdg, "concave-tui"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "concave-tui", "config.toml"), []byte(`[display]
graph_style = "bar"
graph_auto_width_threshold = 130
graph_auto_height_threshold = 50
sidebar_default = "collapsed"
refresh_interval_ms = 1500

[layout]
active_preset = "training"
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(workspacepkg.ConfigPath("concave-tui.toml"), []byte(`[[preset]]
name = "default"
description = "Default"
widgets = ["suite-status"]

[[preset]]
name = "training"
description = "Training"
widgets = ["gpu-graph", "vram-bar"]
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Display.GraphStyle != "bar" || cfg.ActivePreset != "training" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if len(cfg.Presets) != 2 {
		t.Fatalf("preset count = %d", len(cfg.Presets))
	}
}

func TestLoadFallsBackToDefaultPresetWhenMissing(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := workspacepkg.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(xdg, "concave-tui"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "concave-tui", "config.toml"), []byte(`[layout]
active_preset = "missing"
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ActivePreset != "default" {
		t.Fatalf("ActivePreset = %q", cfg.ActivePreset)
	}
}

func TestSaveWritesAtomically(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := DefaultConfig()
	cfg.Display.GraphStyle = "line"
	cfg.ActivePreset = "mlops"
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path := filepath.Join(xdg, "concave-tui", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) == "" {
		t.Fatal("expected saved config content")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("unexpected temp file state: %v", err)
	}
}

func TestResolveGraphStyleBoundaries(t *testing.T) {
	cfg := DefaultConfig()
	if got := ResolveGraphStyle(cfg, 119, 40); got != "bar" {
		t.Fatalf("ResolveGraphStyle(119,40) = %q", got)
	}
	if got := ResolveGraphStyle(cfg, 120, 40); got != "line" {
		t.Fatalf("ResolveGraphStyle(120,40) = %q", got)
	}
	if got := ResolveGraphStyle(cfg, 120, 39); got != "bar" {
		t.Fatalf("ResolveGraphStyle(120,39) = %q", got)
	}
}
