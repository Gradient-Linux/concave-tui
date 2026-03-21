package suite

import (
	"fmt"
)

var orderedNames = []string{"boosting", "neural", "flow", "forge"}

// Container describes a concrete container in a suite.
type Container struct {
	Name  string
	Image string
	Role  string
}

// PortMapping describes a published suite port.
type PortMapping struct {
	Port    int
	Service string
}

// VolumeMount describes a workspace-backed mount.
type VolumeMount struct {
	HostPath      string
	ContainerPath string
}

// Suite defines a named collection of containers, ports, and mounts.
type Suite struct {
	Name            string
	Containers      []Container
	Ports           []PortMapping
	Volumes         []VolumeMount
	ComposeTemplate string
	GPURequired     bool
}

// Registry is the single source of truth for suite metadata.
var Registry = map[string]Suite{
	"boosting": {
		Name: "boosting",
		Containers: []Container{
			{Name: "gradient-boost-core", Image: "python:3.12-slim", Role: "Core ML stack"},
			{Name: "gradient-boost-lab", Image: "quay.io/jupyter/base-notebook:python-3.11.6", Role: "JupyterLab"},
			{Name: "gradient-boost-track", Image: "ghcr.io/mlflow/mlflow:2.14", Role: "MLflow tracking"},
		},
		Ports: []PortMapping{
			{Port: 8888, Service: "JupyterLab"},
			{Port: 5000, Service: "MLflow"},
		},
		Volumes: []VolumeMount{
			{HostPath: "data", ContainerPath: "/data"},
			{HostPath: "notebooks", ContainerPath: "/notebooks"},
			{HostPath: "models", ContainerPath: "/models"},
			{HostPath: "outputs", ContainerPath: "/outputs"},
			{HostPath: "mlruns", ContainerPath: "/mlruns"},
		},
		ComposeTemplate: "boosting",
		GPURequired:     false,
	},
	"neural": {
		Name: "neural",
		Containers: []Container{
			{Name: "gradient-neural-torch", Image: "pytorch/pytorch:2.6.0-cuda12.4-cudnn9-runtime", Role: "Training"},
			{Name: "gradient-neural-infer", Image: "nvidia/cuda:12.4-runtime-ubuntu24.04", Role: "Inference"},
			{Name: "gradient-neural-lab", Image: "quay.io/jupyter/base-notebook:python-3.11.6", Role: "JupyterLab"},
		},
		Ports: []PortMapping{
			{Port: 8888, Service: "JupyterLab"},
			{Port: 8000, Service: "vLLM API"},
			{Port: 8080, Service: "llama.cpp"},
		},
		Volumes: []VolumeMount{
			{HostPath: "data", ContainerPath: "/data"},
			{HostPath: "notebooks", ContainerPath: "/notebooks"},
			{HostPath: "models", ContainerPath: "/models"},
			{HostPath: "outputs", ContainerPath: "/outputs"},
		},
		ComposeTemplate: "neural",
		GPURequired:     true,
	},
	"flow": {
		Name: "flow",
		Containers: []Container{
			{Name: "gradient-flow-mlflow", Image: "ghcr.io/mlflow/mlflow:2.14", Role: "Experiment tracking"},
			{Name: "gradient-flow-airflow", Image: "apache/airflow:2.9.0", Role: "Orchestration"},
			{Name: "gradient-flow-prometheus", Image: "prom/prometheus:v2.51.0", Role: "Metrics"},
			{Name: "gradient-flow-grafana", Image: "grafana/grafana:10.4.0", Role: "Dashboards"},
			{Name: "gradient-flow-store", Image: "minio/minio:RELEASE.2024-04-06T05-26-02Z", Role: "Artifact storage"},
			{Name: "gradient-flow-serve", Image: "bentoml/bentoml:1.2.0", Role: "Model serving"},
		},
		Ports: []PortMapping{
			{Port: 5000, Service: "MLflow"},
			{Port: 8080, Service: "Airflow"},
			{Port: 9090, Service: "Prometheus"},
			{Port: 3000, Service: "Grafana"},
			{Port: 9001, Service: "MinIO console"},
			{Port: 3100, Service: "BentoML endpoint"},
		},
		Volumes: []VolumeMount{
			{HostPath: "mlruns", ContainerPath: "/mlruns"},
			{HostPath: "dags", ContainerPath: "/dags"},
			{HostPath: "outputs", ContainerPath: "/outputs"},
			{HostPath: "models", ContainerPath: "/models"},
		},
		ComposeTemplate: "flow",
		GPURequired:     false,
	},
	"forge": {
		Name: "forge",
		Containers: []Container{
			{Name: "gradient-boost-core", Image: "python:3.12-slim", Role: "Core ML stack"},
			{Name: "gradient-boost-lab", Image: "quay.io/jupyter/base-notebook:python-3.11.6", Role: "JupyterLab"},
			{Name: "gradient-boost-track", Image: "ghcr.io/mlflow/mlflow:2.14", Role: "MLflow tracking"},
			{Name: "gradient-neural-torch", Image: "pytorch/pytorch:2.6.0-cuda12.4-cudnn9-runtime", Role: "Training"},
			{Name: "gradient-neural-infer", Image: "nvidia/cuda:12.4-runtime-ubuntu24.04", Role: "Inference"},
			{Name: "gradient-neural-lab", Image: "quay.io/jupyter/base-notebook:python-3.11.6", Role: "JupyterLab"},
			{Name: "gradient-flow-mlflow", Image: "ghcr.io/mlflow/mlflow:2.14", Role: "Experiment tracking"},
			{Name: "gradient-flow-airflow", Image: "apache/airflow:2.9.0", Role: "Orchestration"},
			{Name: "gradient-flow-prometheus", Image: "prom/prometheus:v2.51.0", Role: "Metrics"},
			{Name: "gradient-flow-grafana", Image: "grafana/grafana:10.4.0", Role: "Dashboards"},
			{Name: "gradient-flow-store", Image: "minio/minio:RELEASE.2024-04-06T05-26-02Z", Role: "Artifact storage"},
			{Name: "gradient-flow-serve", Image: "bentoml/bentoml:1.2.0", Role: "Model serving"},
		},
		Ports: []PortMapping{
			{Port: 8888, Service: "JupyterLab"},
			{Port: 8000, Service: "vLLM API"},
			{Port: 8080, Service: "Airflow / llama.cpp"},
			{Port: 5000, Service: "MLflow"},
			{Port: 3000, Service: "Grafana"},
			{Port: 9090, Service: "Prometheus"},
			{Port: 9001, Service: "MinIO console"},
			{Port: 3100, Service: "BentoML endpoint"},
		},
		Volumes: []VolumeMount{
			{HostPath: "data", ContainerPath: "/data"},
			{HostPath: "notebooks", ContainerPath: "/notebooks"},
			{HostPath: "models", ContainerPath: "/models"},
			{HostPath: "outputs", ContainerPath: "/outputs"},
			{HostPath: "mlruns", ContainerPath: "/mlruns"},
			{HostPath: "dags", ContainerPath: "/dags"},
		},
		ComposeTemplate: "forge",
		GPURequired:     false,
	},
}

// All returns all known suites in stable order.
func All() []Suite {
	suites := make([]Suite, 0, len(orderedNames))
	for _, name := range orderedNames {
		suites = append(suites, Registry[name])
	}
	return suites
}

// Names returns suite names in stable order.
func Names() []string {
	names := make([]string, len(orderedNames))
	copy(names, orderedNames)
	return names
}

// Get returns a suite by name.
func Get(name string) (Suite, error) {
	s, ok := Registry[name]
	if !ok {
		return Suite{}, fmt.Errorf("unknown suite: %s. Valid suites: boosting, neural, flow, forge", name)
	}
	return s, nil
}

// PrimaryContainer returns the first container in a suite.
func PrimaryContainer(s Suite) string {
	if len(s.Containers) == 0 {
		return ""
	}
	return s.Containers[0].Name
}

// JupyterContainer returns the suite container that exposes JupyterLab, if any.
func JupyterContainer(s Suite) (string, bool) {
	for _, container := range s.Containers {
		if container.Role == "JupyterLab" {
			return container.Name, true
		}
	}
	return "", false
}

// RecordName exposes the suite name for manifest writers without introducing a package cycle.
func (s Suite) RecordName() string {
	return s.Name
}

// RecordImages exposes the suite container images for manifest writers.
func (s Suite) RecordImages() map[string]string {
	images := make(map[string]string, len(s.Containers))
	for _, container := range s.Containers {
		images[container.Name] = container.Image
	}
	return images
}
