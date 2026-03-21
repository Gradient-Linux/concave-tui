package suite

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Gradient-Linux/concave-tui/internal/ui"
)

// ForgeSelection captures the enabled Forge components.
type ForgeSelection struct {
	Containers []Container
	Ports      []PortMapping
	Volumes    []VolumeMount
}

type forgeComponent struct {
	Key       string
	Label     string
	Container Container
	Ports     []PortMapping
	Volumes   []VolumeMount
	SharedKey string
}

var promptChecklist = ui.Checklist

var forgeComponents = []forgeComponent{
	{
		Key:       "boosting-core",
		Label:     "Boosting | Classical ML stack (~1 GB)",
		Container: Registry["boosting"].Containers[0],
		Volumes:   Registry["boosting"].Volumes,
	},
	{
		Key:       "boosting-lab",
		Label:     "Boosting | JupyterLab (~1 GB, shared with Neural)",
		Container: Registry["boosting"].Containers[1],
		Ports:     []PortMapping{{Port: 8888, Service: "JupyterLab"}},
		Volumes:   Registry["boosting"].Volumes,
		SharedKey: "jupyterlab",
	},
	{
		Key:       "boosting-track",
		Label:     "Boosting | Experiment tracking (~500 MB)",
		Container: Registry["boosting"].Containers[2],
		Ports:     []PortMapping{{Port: 5000, Service: "MLflow"}},
		Volumes:   []VolumeMount{{HostPath: "mlruns", ContainerPath: "/mlruns"}, {HostPath: "outputs", ContainerPath: "/outputs"}},
		SharedKey: "mlflow",
	},
	{
		Key:       "neural-train",
		Label:     "Neural | PyTorch training stack (~15 GB)",
		Container: Registry["neural"].Containers[0],
		Volumes:   Registry["neural"].Volumes,
	},
	{
		Key:       "neural-infer",
		Label:     "Neural | Inference server (vLLM) (~3 GB)",
		Container: Registry["neural"].Containers[1],
		Ports:     []PortMapping{{Port: 8000, Service: "vLLM API"}, {Port: 8080, Service: "llama.cpp"}},
		Volumes:   Registry["neural"].Volumes,
	},
	{
		Key:       "neural-lab",
		Label:     "Neural | JupyterLab (~1 GB, shared with Boosting)",
		Container: Registry["neural"].Containers[2],
		Ports:     []PortMapping{{Port: 8888, Service: "JupyterLab"}},
		Volumes:   Registry["neural"].Volumes,
		SharedKey: "jupyterlab",
	},
	{
		Key:       "flow-airflow",
		Label:     "Flow | Pipeline orchestration (~1 GB)",
		Container: Registry["flow"].Containers[1],
		Ports:     []PortMapping{{Port: 8080, Service: "Airflow"}},
		Volumes:   []VolumeMount{{HostPath: "dags", ContainerPath: "/dags"}, {HostPath: "outputs", ContainerPath: "/outputs"}},
	},
	{
		Key:       "flow-monitoring",
		Label:     "Flow | System monitoring (~500 MB)",
		Container: Registry["flow"].Containers[2],
		Ports:     []PortMapping{{Port: 9090, Service: "Prometheus"}},
		Volumes:   []VolumeMount{{HostPath: "outputs", ContainerPath: "/outputs"}},
	},
	{
		Key:       "flow-grafana",
		Label:     "Flow | Dashboards (~500 MB)",
		Container: Registry["flow"].Containers[3],
		Ports:     []PortMapping{{Port: 3000, Service: "Grafana"}},
		Volumes:   []VolumeMount{{HostPath: "outputs", ContainerPath: "/outputs"}},
	},
	{
		Key:       "flow-store",
		Label:     "Flow | Artifact storage (~200 MB)",
		Container: Registry["flow"].Containers[4],
		Ports:     []PortMapping{{Port: 9001, Service: "MinIO console"}},
		Volumes:   []VolumeMount{{HostPath: "models", ContainerPath: "/data"}, {HostPath: "outputs", ContainerPath: "/outputs"}},
	},
	{
		Key:       "flow-serve",
		Label:     "Flow | Model serving (~800 MB)",
		Container: Registry["flow"].Containers[5],
		Ports:     []PortMapping{{Port: 3100, Service: "BentoML endpoint"}},
		Volumes:   []VolumeMount{{HostPath: "models", ContainerPath: "/models"}, {HostPath: "outputs", ContainerPath: "/outputs"}},
	},
	{
		Key:       "flow-mlflow",
		Label:     "Flow | MLflow tracking (~500 MB, shared with Boosting)",
		Container: Registry["flow"].Containers[0],
		Ports:     []PortMapping{{Port: 5000, Service: "MLflow"}},
		Volumes:   []VolumeMount{{HostPath: "mlruns", ContainerPath: "/mlruns"}, {HostPath: "outputs", ContainerPath: "/outputs"}},
		SharedKey: "mlflow",
	},
}

// PickComponents collects the selected Forge components through the shared UI prompt.
func PickComponents() (ForgeSelection, error) {
	items := make([]string, 0, len(forgeComponents))
	byLabel := make(map[string]forgeComponent, len(forgeComponents))
	for _, component := range forgeComponents {
		items = append(items, component.Label)
		byLabel[component.Label] = component
	}

	selected := promptChecklist(items)
	if len(selected) == 0 {
		return ForgeSelection{}, fmt.Errorf("no components selected")
	}

	components := make([]forgeComponent, 0, len(selected))
	for _, label := range selected {
		component, ok := byLabel[label]
		if !ok {
			continue
		}
		components = append(components, component)
	}

	return selectionFromComponents(components, nil)
}

// SelectionFromContainerNames rebuilds a Forge selection from concrete container names.
func SelectionFromContainerNames(names []string, imageOverrides map[string]string) (ForgeSelection, error) {
	if len(names) == 0 {
		return ForgeSelection{}, fmt.Errorf("no components selected")
	}

	componentByContainer := make(map[string]forgeComponent, len(forgeComponents))
	for _, component := range forgeComponents {
		componentByContainer[component.Container.Name] = component
	}

	components := make([]forgeComponent, 0, len(names))
	for _, name := range names {
		component, ok := componentByContainer[name]
		if !ok {
			continue
		}
		components = append(components, component)
	}

	return selectionFromComponents(components, imageOverrides)
}

// BuildForgeCompose filters the forge template down to the selected services.
func BuildForgeCompose(selection ForgeSelection) ([]byte, error) {
	if len(selection.Containers) == 0 {
		return nil, fmt.Errorf("no components selected")
	}

	data, err := readForgeTemplate()
	if err != nil {
		return nil, err
	}

	selected := make(map[string]string, len(selection.Containers))
	for _, container := range selection.Containers {
		selected[container.Name] = container.Image
	}

	lines := strings.Split(string(data), "\n")
	var (
		result         []string
		block          []string
		currentService string
		inServices     bool
		inNetworks     bool
	)

	flush := func() {
		if len(block) == 0 {
			return
		}
		image, ok := selected[currentService]
		if currentService == "" || ok {
			for _, line := range block {
				if strings.Contains(line, "profiles: [\"disabled\"]") {
					continue
				}
				trimmed := strings.TrimSpace(line)
				if ok && strings.HasPrefix(trimmed, "image:") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
					line = indent + "image: " + image
				}
				result = append(result, line)
			}
		}
		block = nil
		currentService = ""
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "services:"):
			inServices = true
			inNetworks = false
			result = append(result, line)
		case strings.HasPrefix(line, "networks:"):
			flush()
			inServices = false
			inNetworks = true
			result = append(result, line)
		case inServices && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":"):
			flush()
			currentService = strings.TrimSuffix(strings.TrimSpace(line), ":")
			block = append(block, line)
		case inNetworks:
			result = append(result, line)
		case inServices:
			block = append(block, line)
		default:
			result = append(result, line)
		}
	}
	flush()

	return []byte(strings.Join(result, "\n") + "\n"), nil
}

func selectionFromComponents(components []forgeComponent, imageOverrides map[string]string) (ForgeSelection, error) {
	if len(components) == 0 {
		return ForgeSelection{}, fmt.Errorf("no components selected")
	}

	containers := make([]Container, 0, len(components))
	ports := make([]PortMapping, 0, len(components))
	volumes := make([]VolumeMount, 0, len(components))

	sharedSelections := map[string]string{}
	containerSeen := map[string]struct{}{}
	portSeen := map[int]struct{}{}
	volumeSeen := map[string]struct{}{}

	for _, component := range components {
		if component.SharedKey != "" {
			if existing, ok := sharedSelections[component.SharedKey]; ok {
				if strings.Contains(existing, "boost") {
					continue
				}
				if strings.Contains(component.Container.Name, "boost") {
					for idx := range containers {
						if containers[idx].Name == existing {
							containers[idx] = applyContainerImageOverride(component.Container, imageOverrides)
							sharedSelections[component.SharedKey] = component.Container.Name
							goto includeMappings
						}
					}
				}
				continue
			}
			sharedSelections[component.SharedKey] = component.Container.Name
		}

		if _, ok := containerSeen[component.Container.Name]; !ok {
			containers = append(containers, applyContainerImageOverride(component.Container, imageOverrides))
			containerSeen[component.Container.Name] = struct{}{}
		}

	includeMappings:
		for _, port := range component.Ports {
			if _, ok := portSeen[port.Port]; ok {
				continue
			}
			portSeen[port.Port] = struct{}{}
			ports = append(ports, port)
		}
		for _, volume := range component.Volumes {
			key := volume.HostPath + ":" + volume.ContainerPath
			if _, ok := volumeSeen[key]; ok {
				continue
			}
			volumeSeen[key] = struct{}{}
			volumes = append(volumes, volume)
		}
	}

	return ForgeSelection{
		Containers: containers,
		Ports:      ports,
		Volumes:    volumes,
	}, nil
}

func applyContainerImageOverride(container Container, overrides map[string]string) Container {
	if overrides == nil {
		return container
	}
	if image, ok := overrides[container.Name]; ok && image != "" {
		container.Image = image
	}
	return container
}

func readForgeTemplate() ([]byte, error) {
	candidates := []string{filepath.Join("templates", "forge.compose.yml")}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
		candidates = append(candidates, filepath.Join(repoRoot, "templates", "forge.compose.yml"))
	}

	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "templates", "forge.compose.yml"))
	}

	var failures []string
	for _, candidate := range uniqueForgePaths(candidates) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
	}

	return nil, fmt.Errorf("read forge.compose.yml: %s", strings.Join(failures, "; "))
}

func uniqueForgePaths(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
