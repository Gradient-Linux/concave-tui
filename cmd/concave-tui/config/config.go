package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	workspacepkg "github.com/Gradient-Linux/concave-tui/internal/workspace"
)

type DisplayConfig struct {
	GraphStyle               string `toml:"graph_style"`
	GraphAutoWidthThreshold  int    `toml:"graph_auto_width_threshold"`
	GraphAutoHeightThreshold int    `toml:"graph_auto_height_threshold"`
	SidebarDefault           string `toml:"sidebar_default"`
	RefreshIntervalMs        int    `toml:"refresh_interval_ms"`
}

type Config struct {
	Display      DisplayConfig
	ActivePreset string
	Presets      []Preset
}

type Preset struct {
	Name        string
	Description string
	Widgets     []string
}

func (c Config) PresetByName(name string) (Preset, bool) {
	return PresetByName(c.Presets, name)
}

func (c Config) PresetNames() []string {
	return PresetNames(c.Presets)
}

var (
	userConfigDirFn      = os.UserConfigDir
	ensureWorkspaceFn    = workspacepkg.EnsureLayout
	workspacePresetPathFn = func() string {
		return workspacepkg.ConfigPath("concave-tui.toml")
	}
)

func DefaultConfig() Config {
	return Config{
		Display: DisplayConfig{
			GraphStyle:               "auto",
			GraphAutoWidthThreshold:  120,
			GraphAutoHeightThreshold: 40,
			SidebarDefault:           "expanded",
			RefreshIntervalMs:        1000,
		},
		ActivePreset: "default",
		Presets: []Preset{
			{
				Name:        "default",
				Description: "Balance view",
				Widgets:     []string{"gpu-graph", "vram-bar", "ram-bar", "suite-status", "system-health"},
			},
			{
				Name:        "training",
				Description: "Training view",
				Widgets:     []string{"gpu-graph", "gpu-graph-2", "vram-bar", "suite-status"},
			},
			{
				Name:        "mlops",
				Description: "MLOps view",
				Widgets:     []string{"suite-status", "port-map", "flow-services", "system-health"},
			},
			{
				Name:        "inference",
				Description: "Inference view",
				Widgets:     []string{"gpu-graph", "vram-bar", "neural-containers", "suite-status"},
			},
		},
	}
}

func Load() (Config, error) {
	defaults := DefaultConfig()

	xdgPath, err := xdgConfigPath()
	if err != nil {
		return Config{}, err
	}
	if err := ensureFile(xdgPath, marshalXDG(defaults)); err != nil {
		return Config{}, err
	}
	if err := ensureWorkspaceFn(); err != nil {
		return Config{}, err
	}

	presetPath := workspacePresetPathFn()
	if err := ensureFile(presetPath, marshalPresets(defaults.Presets)); err != nil {
		return Config{}, err
	}

	xdgData, err := os.ReadFile(xdgPath)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", xdgPath, err)
	}
	workspaceData, err := os.ReadFile(presetPath)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", presetPath, err)
	}

	result := defaults
	parseXDGInto(&result, xdgData)
	presets := parsePresets(workspaceData)
	if len(presets) == 0 {
		presets = defaults.Presets
	}
	result.Presets = presets
	if !hasPreset(result.Presets, result.ActivePreset) {
		result.ActivePreset = "default"
	}
	return result, nil
}

func Save(c Config) error {
	xdgPath, err := xdgConfigPath()
	if err != nil {
		return err
	}
	return writeAtomically(xdgPath, marshalXDG(c))
}

func ResolveGraphStyle(cfg Config, termWidth, termHeight int) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Display.GraphStyle)) {
	case "line":
		return "line"
	case "bar":
		return "bar"
	default:
		if termWidth >= cfg.Display.GraphAutoWidthThreshold && termHeight >= cfg.Display.GraphAutoHeightThreshold {
			return "line"
		}
		return "bar"
	}
}

func xdgConfigPath() (string, error) {
	dir, err := userConfigDirFn()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "concave-tui", "config.toml"), nil
}

func hasPreset(items []Preset, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func ensureFile(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return writeAtomically(path, data)
}

func writeAtomically(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

func marshalXDG(c Config) []byte {
	var buf bytes.Buffer
	buf.WriteString("[display]\n")
	fmt.Fprintf(&buf, "graph_style = %q\n", normalizedGraphStyle(c.Display.GraphStyle))
	fmt.Fprintf(&buf, "graph_auto_width_threshold = %d\n", max(1, c.Display.GraphAutoWidthThreshold))
	fmt.Fprintf(&buf, "graph_auto_height_threshold = %d\n", max(1, c.Display.GraphAutoHeightThreshold))
	fmt.Fprintf(&buf, "sidebar_default = %q\n", normalizedSidebar(c.Display.SidebarDefault))
	fmt.Fprintf(&buf, "refresh_interval_ms = %d\n", max(100, c.Display.RefreshIntervalMs))
	buf.WriteString("\n[layout]\n")
	fmt.Fprintf(&buf, "active_preset = %q\n", activePresetName(c))
	return buf.Bytes()
}

func marshalPresets(items []Preset) []byte {
	var buf bytes.Buffer
	for idx, preset := range items {
		if idx > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString("[[preset]]\n")
		fmt.Fprintf(&buf, "name = %q\n", preset.Name)
		fmt.Fprintf(&buf, "description = %q\n", preset.Description)
		buf.WriteString("widgets = [")
		for i, widget := range preset.Widgets {
			if i > 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%q", widget)
		}
		buf.WriteString("]\n")
	}
	return buf.Bytes()
}

func parseXDGInto(cfg *Config, data []byte) {
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := stripComment(raw)
		if line == "" {
			continue
		}
		switch line {
		case "[display]":
			section = "display"
			continue
		case "[layout]":
			section = "layout"
			continue
		}
		key, value, ok := splitAssignment(line)
		if !ok {
			continue
		}
		switch section {
		case "display":
			switch key {
			case "graph_style":
				cfg.Display.GraphStyle = parseString(value, cfg.Display.GraphStyle)
			case "graph_auto_width_threshold":
				cfg.Display.GraphAutoWidthThreshold = parseInt(value, cfg.Display.GraphAutoWidthThreshold)
			case "graph_auto_height_threshold":
				cfg.Display.GraphAutoHeightThreshold = parseInt(value, cfg.Display.GraphAutoHeightThreshold)
			case "sidebar_default":
				cfg.Display.SidebarDefault = parseString(value, cfg.Display.SidebarDefault)
			case "refresh_interval_ms":
				cfg.Display.RefreshIntervalMs = parseInt(value, cfg.Display.RefreshIntervalMs)
			}
		case "layout":
			if key == "active_preset" {
				cfg.ActivePreset = parseString(value, cfg.ActivePreset)
			}
		}
	}
	cfg.Display.GraphStyle = normalizedGraphStyle(cfg.Display.GraphStyle)
	cfg.Display.SidebarDefault = normalizedSidebar(cfg.Display.SidebarDefault)
	if cfg.Display.GraphAutoWidthThreshold <= 0 {
		cfg.Display.GraphAutoWidthThreshold = DefaultConfig().Display.GraphAutoWidthThreshold
	}
	if cfg.Display.GraphAutoHeightThreshold <= 0 {
		cfg.Display.GraphAutoHeightThreshold = DefaultConfig().Display.GraphAutoHeightThreshold
	}
	if cfg.Display.RefreshIntervalMs <= 0 {
		cfg.Display.RefreshIntervalMs = DefaultConfig().Display.RefreshIntervalMs
	}
	if strings.TrimSpace(cfg.ActivePreset) == "" {
		cfg.ActivePreset = "default"
	}
}

func parsePresets(data []byte) []Preset {
	var (
		presets []Preset
		current *Preset
	)
	for _, raw := range strings.Split(string(data), "\n") {
		line := stripComment(raw)
		if line == "" {
			continue
		}
		if line == "[[preset]]" {
			presets = append(presets, Preset{})
			current = &presets[len(presets)-1]
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := splitAssignment(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			current.Name = parseString(value, current.Name)
		case "description":
			current.Description = parseString(value, current.Description)
		case "widgets":
			current.Widgets = parseArray(value)
		}
	}
	valid := make([]Preset, 0, len(presets))
	for _, preset := range presets {
		if strings.TrimSpace(preset.Name) == "" || len(preset.Widgets) == 0 {
			continue
		}
		valid = append(valid, preset)
	}
	return valid
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func splitAssignment(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

func parseString(value, fallback string) string {
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return fallback
	}
	return unquoted
}

func parseInt(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return n
}

func parseArray(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		text, err := strconv.Unquote(strings.TrimSpace(part))
		if err != nil || text == "" {
			continue
		}
		items = append(items, text)
	}
	return items
}

func normalizedGraphStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "line":
		return "line"
	case "bar":
		return "bar"
	default:
		return "auto"
	}
}

func normalizedSidebar(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "collapsed":
		return "collapsed"
	default:
		return "expanded"
	}
}

func activePresetName(c Config) string {
	if hasPreset(c.Presets, c.ActivePreset) {
		return c.ActivePreset
	}
	return "default"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func PresetByName(items []Preset, name string) (Preset, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return Preset{}, false
}

func PresetNames(items []Preset) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	slices.Sort(names)
	return names
}
