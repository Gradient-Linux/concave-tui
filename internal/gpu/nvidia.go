package gpu

import (
	"fmt"
	"strconv"
	"strings"
)

// NVIDIADevice is a point-in-time snapshot of one NVIDIA GPU.
type NVIDIADevice struct {
	Index         int
	Name          string
	Utilization   int
	MemoryUsedMiB int
	MemoryTotalMiB int
	DriverVersion string
}

// MemoryRatio returns used/total VRAM for the device.
func (d NVIDIADevice) MemoryRatio() float64 {
	if d.MemoryTotalMiB <= 0 {
		return 0
	}
	return float64(d.MemoryUsedMiB) / float64(d.MemoryTotalMiB)
}

// MemoryUsedBytes returns VRAM used in bytes.
func (d NVIDIADevice) MemoryUsedBytes() uint64 {
	if d.MemoryUsedMiB <= 0 {
		return 0
	}
	return uint64(d.MemoryUsedMiB) * 1024 * 1024
}

// MemoryTotalBytes returns total VRAM in bytes.
func (d NVIDIADevice) MemoryTotalBytes() uint64 {
	if d.MemoryTotalMiB <= 0 {
		return 0
	}
	return uint64(d.MemoryTotalMiB) * 1024 * 1024
}

// ComputeCapability reads the NVIDIA compute capability from nvidia-smi.
func ComputeCapability() (string, error) {
	out, err := runner.Run("nvidia-smi", "--query-gpu=compute_cap", "--format=csv,noheader")
	if err != nil {
		return "", fmt.Errorf("nvidia-smi compute capability: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DriverBranchForCapability maps a compute capability to the recommended driver branch.
func DriverBranchForCapability(capability string) (string, error) {
	switch capability {
	case "7.0", "7.2", "7.5":
		return "535", nil
	case "8.0", "8.6":
		return "560", nil
	case "8.9", "9.0":
		return "570", nil
	default:
		if strings.HasPrefix(capability, "7.") {
			return "535", nil
		}
		if strings.HasPrefix(capability, "8.") {
			if capability == "8.9" {
				return "570", nil
			}
			return "560", nil
		}
		if strings.HasPrefix(capability, "9.") {
			return "570", nil
		}
		return "", fmt.Errorf("unsupported compute capability %q", capability)
	}
}

// RecommendedDriverBranch returns the recommended driver branch for the detected NVIDIA GPU.
func RecommendedDriverBranch() (string, error) {
	capability, err := ComputeCapability()
	if err != nil {
		return "", err
	}
	return DriverBranchForCapability(capability)
}

// ToolkitConfigured reports whether nvidia-ctk runtime verification succeeds.
func ToolkitConfigured() (bool, error) {
	if _, err := runner.Run("nvidia-ctk", "runtime", "configure", "--runtime=docker", "--dry-run"); err != nil {
		return false, fmt.Errorf("nvidia-ctk runtime configure --dry-run: %w", err)
	}
	return true, nil
}

// NVIDIADevices returns live utilization, VRAM, and driver data for all visible NVIDIA GPUs.
func NVIDIADevices() ([]NVIDIADevice, error) {
	out, err := runner.Run(
		"nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,driver_version",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi gpu query: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	devices := make([]NVIDIADevice, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			return nil, fmt.Errorf("unexpected nvidia-smi gpu query output")
		}
		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu index: %w", err)
		}
		utilization, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu utilization: %w", err)
		}
		used, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu memory used: %w", err)
		}
		total, err := strconv.Atoi(strings.TrimSpace(parts[4]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu memory total: %w", err)
		}
		devices = append(devices, NVIDIADevice{
			Index:          index,
			Name:           strings.TrimSpace(parts[1]),
			Utilization:    utilization,
			MemoryUsedMiB:  used,
			MemoryTotalMiB: total,
			DriverVersion:  strings.TrimSpace(parts[5]),
		})
	}
	return devices, nil
}

// CUDAVersion returns the CUDA version reported by nvidia-smi.
func CUDAVersion() (string, error) {
	out, err := runner.Run("nvidia-smi")
	if err != nil {
		return "", fmt.Errorf("nvidia-smi: %w", err)
	}
	text := string(out)
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "CUDA Version:") {
			continue
		}
		parts := strings.Split(line, "CUDA Version:")
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if fields := strings.Fields(value); len(fields) > 0 {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("cuda version not found")
}

// VerifyPassthrough verifies Docker GPU passthrough with a CUDA base image.
func VerifyPassthrough() error {
	if _, err := runner.Run("docker", "run", "--rm", "--gpus", "all", "nvidia/cuda:12.4-base-ubuntu24.04", "nvidia-smi"); err != nil {
		return fmt.Errorf("docker gpu passthrough: %w", err)
	}
	return nil
}

// SecureBootEnabled reports whether Secure Boot is enabled on the host.
func SecureBootEnabled() (bool, error) {
	out, err := runner.Run("mokutil", "--sb-state")
	if err != nil {
		return false, fmt.Errorf("mokutil --sb-state: %w", err)
	}
	text := strings.ToLower(strings.TrimSpace(string(out)))
	text = strings.ReplaceAll(text, " ", "")
	return strings.Contains(text, "securebootenabled"), nil
}
