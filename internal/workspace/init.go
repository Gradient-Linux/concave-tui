package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

var layout = []struct {
	name string
	mode os.FileMode
}{
	{"data", 0o755},
	{"notebooks", 0o755},
	{"models", 0o755},
	{"outputs", 0o755},
	{"mlruns", 0o755},
	{"dags", 0o755},
	{"compose", 0o755},
	{"config", 0o700},
	{"backups", 0o755},
}

// Root returns the fixed Gradient workspace root.
func Root() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/gradient"
	}
	return filepath.Join(home, "gradient")
}

// Exists reports whether the workspace root currently exists.
func Exists() bool {
	info, err := os.Stat(Root())
	return err == nil && info.IsDir()
}

// EnsureLayout creates the full Gradient workspace tree.
func EnsureLayout() error {
	root := Root()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", root, err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", root, err)
	}

	for _, item := range layout {
		path := filepath.Join(root, item.name)
		if err := os.MkdirAll(path, item.mode); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
		if err := os.Chmod(path, item.mode); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}

	return nil
}

// ComposePath returns the compose file path for a suite.
func ComposePath(name string) string {
	return filepath.Join(Root(), "compose", name+".compose.yml")
}

// ConfigPath returns a path within ~/gradient/config.
func ConfigPath(name string) string {
	return filepath.Join(Root(), "config", name)
}

// CleanOutputs removes all contents under ~/gradient/outputs while preserving the directory itself.
func CleanOutputs() error {
	outputsDir := filepath.Join(Root(), "outputs")
	entries, err := os.ReadDir(outputsDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", outputsDir, err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(outputsDir, entry.Name())); err != nil {
			return fmt.Errorf("remove %s: %w", entry.Name(), err)
		}
	}
	return nil
}
