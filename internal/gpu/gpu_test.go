package gpu

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Gradient-Linux/concave-tui/internal/ui"
)

type mockRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("unexpected command: " + key)
}

func TestDetect(t *testing.T) {
	previous := runner
	runner = &mockRunner{outputs: map[string][]byte{"nvidia-smi ": []byte("ok")}}
	defer func() { runner = previous }()

	state, err := Detect()
	if err != nil || state != GPUStateNVIDIA {
		t.Fatalf("Detect() = %v, %v", state, err)
	}
}

func TestDetectAMDAndCPUOnly(t *testing.T) {
	previous := runner
	runner = &mockRunner{
		outputs: map[string][]byte{
			"rocminfo ": []byte("ok"),
		},
		errors: map[string]error{
			"nvidia-smi ": errors.New("missing"),
		},
	}
	defer func() { runner = previous }()

	state, err := Detect()
	if err != nil || state != GPUStateAMD {
		t.Fatalf("Detect() = %v, %v", state, err)
	}

	runner = &mockRunner{
		errors: map[string]error{
			"nvidia-smi ": errors.New("missing"),
			"rocminfo ":   errors.New("missing"),
		},
	}
	state, err = Detect()
	if err != nil || state != GPUStateNone {
		t.Fatalf("Detect() cpu-only = %v, %v", state, err)
	}
}

func TestDetectPrefersNVIDIAWhenBothArePresent(t *testing.T) {
	previous := runner
	t.Cleanup(func() { runner = previous })

	runner = &mockRunner{
		outputs: map[string][]byte{
			"nvidia-smi ": []byte("ok"),
			"rocminfo ":   []byte("ok"),
		},
	}

	state, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if state != GPUStateNVIDIA {
		t.Fatalf("Detect() = %v, want %v", state, GPUStateNVIDIA)
	}
}

func TestDriverBranchForCapability(t *testing.T) {
	cases := map[string]string{
		"7.0": "535",
		"7.5": "535",
		"8.0": "560",
		"8.6": "560",
		"8.9": "570",
		"9.0": "570",
	}
	for capability, want := range cases {
		got, err := DriverBranchForCapability(capability)
		if err != nil {
			t.Fatalf("DriverBranchForCapability(%q) error = %v", capability, err)
		}
		if got != want {
			t.Fatalf("DriverBranchForCapability(%q) = %q, want %q", capability, got, want)
		}
	}

	if _, err := DriverBranchForCapability("6.1"); err == nil {
		t.Fatal("expected unsupported capability error")
	}
}

func TestStringAndNVIDIAHelpers(t *testing.T) {
	if GPUStateNone.String() != "cpu-only" || GPUStateNVIDIA.String() != "nvidia" || GPUStateAMD.String() != "amd" {
		t.Fatal("unexpected GPUState string values")
	}

	previous := runner
	runner = &mockRunner{
		outputs: map[string][]byte{
			"nvidia-smi --query-gpu=compute_cap --format=csv,noheader":                []byte("8.9\n"),
			"nvidia-ctk runtime configure --runtime=docker --dry-run":                 []byte("ok"),
			"docker run --rm --gpus all nvidia/cuda:12.4-base-ubuntu24.04 nvidia-smi": []byte("ok"),
			"mokutil --sb-state": []byte("SecureBoot enabled"),
		},
	}
	defer func() { runner = previous }()

	capability, err := ComputeCapability()
	if err != nil || capability != "8.9" {
		t.Fatalf("ComputeCapability() = %q, %v", capability, err)
	}
	branch, err := RecommendedDriverBranch()
	if err != nil || branch != "570" {
		t.Fatalf("RecommendedDriverBranch() = %q, %v", branch, err)
	}
	ok, err := ToolkitConfigured()
	if err != nil || !ok {
		t.Fatalf("ToolkitConfigured() = %v, %v", ok, err)
	}
	if err := VerifyPassthrough(); err != nil {
		t.Fatalf("VerifyPassthrough() error = %v", err)
	}
	enabled, err := SecureBootEnabled()
	if err != nil || !enabled {
		t.Fatalf("SecureBootEnabled() = %v, %v", enabled, err)
	}
}

func TestNVIDIAHelpersErrorPathsAndSecureBootParsing(t *testing.T) {
	previous := runner
	t.Cleanup(func() { runner = previous })

	runner = &mockRunner{
		errors: map[string]error{
			"nvidia-smi --query-gpu=compute_cap --format=csv,noheader":                errors.New("missing"),
			"nvidia-ctk runtime configure --runtime=docker --dry-run":                 errors.New("not configured"),
			"docker run --rm --gpus all nvidia/cuda:12.4-base-ubuntu24.04 nvidia-smi": errors.New("failed"),
		},
		outputs: map[string][]byte{
			"mokutil --sb-state": []byte("Secure Boot enabled"),
		},
	}

	if _, err := ComputeCapability(); err == nil {
		t.Fatal("expected compute capability error")
	}
	if _, err := ToolkitConfigured(); err == nil {
		t.Fatal("expected toolkit error")
	}
	if err := VerifyPassthrough(); err == nil {
		t.Fatal("expected passthrough error")
	}
	enabled, err := SecureBootEnabled()
	if err != nil {
		t.Fatalf("SecureBootEnabled() error = %v", err)
	}
	if !enabled {
		t.Fatal("expected secure boot to be detected as enabled")
	}
}

func TestDetectAMDWarns(t *testing.T) {
	var buf bytes.Buffer
	ui.SetOutput(&buf)
	defer ui.ResetOutput()

	state := DetectAMD()
	if state != GPUStateAMD {
		t.Fatalf("DetectAMD() = %v", state)
	}
	if !strings.Contains(buf.String(), "ROCm support coming in Gradient Linux v0.3") {
		t.Fatalf("expected AMD warning output, got %q", buf.String())
	}
}

func TestExecCommandRunnerRun(t *testing.T) {
	runner := execCommandRunner{}
	out, err := runner.Run("sh", "-c", "printf ok")
	if err != nil {
		t.Fatalf("execCommandRunner.Run() error = %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("execCommandRunner.Run() output = %q", string(out))
	}
}
