package gpu

import (
	"os/exec"
)

// GPUState describes the detected GPU vendor state for the current host.
type GPUState int

const (
	// GPUStateNone means no GPU-specific runtime was detected.
	GPUStateNone GPUState = iota
	// GPUStateNVIDIA means nvidia-smi succeeded.
	GPUStateNVIDIA
	// GPUStateAMD means rocminfo succeeded.
	GPUStateAMD
)

// CommandRunner runs external commands for GPU detection.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

var runner CommandRunner = execCommandRunner{}

// Detect reports the current GPU state.
func Detect() (GPUState, error) {
	if _, err := runner.Run("nvidia-smi"); err == nil {
		return GPUStateNVIDIA, nil
	}
	if _, err := runner.Run("rocminfo"); err == nil {
		return GPUStateAMD, nil
	}
	return GPUStateNone, nil
}

// String returns a human-readable description of a GPU state.
func (s GPUState) String() string {
	switch s {
	case GPUStateNVIDIA:
		return "nvidia"
	case GPUStateAMD:
		return "amd"
	default:
		return "cpu-only"
	}
}
