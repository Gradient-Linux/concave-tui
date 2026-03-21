package gpu

import (
	"fmt"
	"strings"
)

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
