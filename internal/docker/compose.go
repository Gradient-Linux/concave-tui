package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

const composeNetwork = "gradient-network"

var readTemplateFile = os.ReadFile

// ComposePath returns the expected compose file path for a suite.
func ComposePath(name string) string {
	return workspace.ComposePath(name)
}

// WriteCompose renders a suite compose template into the workspace.
func WriteCompose(name string) (string, error) {
	data, err := renderCompose(name)
	if err != nil {
		return "", err
	}
	return WriteRawCompose(name, data)
}

// WriteRawCompose validates and writes a rendered compose file into the workspace.
func WriteRawCompose(name string, data []byte) (string, error) {
	if err := workspace.EnsureLayout(); err != nil {
		return "", err
	}

	path := ComposePath(name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}

	if err := validateCompose(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename %s to %s: %w", tmp, path, err)
	}

	return path, nil
}

func renderCompose(name string) ([]byte, error) {
	data, err := readTemplate(name)
	if err != nil {
		return nil, err
	}

	rendered := strings.ReplaceAll(string(data), "{{WORKSPACE_ROOT}}", workspace.Root())
	rendered = strings.ReplaceAll(rendered, "{{COMPOSE_NETWORK}}", composeNetwork)
	rendered, err = applyManifestOverrides(name, rendered)
	if err != nil {
		return nil, err
	}
	return []byte(rendered), nil
}

func readTemplate(name string) ([]byte, error) {
	filename := name + ".compose.yml"
	candidates := []string{filepath.Join("templates", filename)}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
		candidates = append(candidates, filepath.Join(repoRoot, "templates", filename))
	}

	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "templates", filename))
	}

	var (
		failures []string
		missing  = true
	)
	for _, candidate := range uniquePaths(candidates) {
		data, err := readTemplateFile(candidate)
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			missing = false
		}
		failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
	}
	if missing {
		return nil, fmt.Errorf("no compose template found for suite: %s", name)
	}
	return nil, fmt.Errorf("read template %s: %s", filename, strings.Join(failures, "; "))
}

func applyManifestOverrides(suiteName, rendered string) (string, error) {
	manifest, err := config.LoadManifest()
	if err != nil {
		return "", err
	}
	containers, ok := manifest[suiteName]
	if !ok || len(containers) == 0 {
		return rendered, nil
	}

	lines := strings.Split(rendered, "\n")
	currentService := ""
	inServices := false
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "services:":
			inServices = true
			currentService = ""
		case inServices && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":"):
			currentService = strings.TrimSuffix(trimmed, ":")
		case inServices && strings.HasPrefix(trimmed, "image:") && currentService != "":
			version, exists := containers[currentService]
			if !exists || version.Current == "" {
				continue
			}
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[idx] = indent + "image: " + version.Current
		case trimmed == "networks:":
			inServices = false
			currentService = ""
		}
	}

	return strings.Join(lines, "\n"), nil
}

func validateCompose(path string) error {
	ctx, cancel := withDefaultTimeout(context.Background())
	defer cancel()

	if _, err := commandRunner.RunCommand(ctx, "docker", "compose", "-f", path, "config", "--quiet"); err != nil {
		return fmt.Errorf("docker compose config %s: %w", path, err)
	}
	return nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}
