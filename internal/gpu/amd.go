package gpu

import "github.com/Gradient-Linux/concave-tui/internal/ui"

// DetectAMD reports an AMD GPU state and emits the v0.3 support warning.
func DetectAMD() GPUState {
	ui.Warn("AMD GPU", "detected — ROCm support coming in Gradient Linux v0.3")
	return GPUStateAMD
}
