package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Gradient-Linux/concave-tui/internal/gpu"
)

type doctorCheck struct {
	name     string
	status   string
	detail   string
	recovery string
	pending  bool
}

type doctorCheckMsg struct {
	token int
	check doctorCheck
}

type DoctorModel struct {
	width     int
	height    int
	active    bool
	runToken  int
	checkedAt time.Time
	checks    []doctorCheck
}

var (
	doctorGPUStatsFn = gpu.NVIDIADevices
	doctorCUDAFn     = gpu.CUDAVersion
)

func NewDoctorModel() DoctorModel {
	return DoctorModel{
		checks: doctorChecksTemplate(),
	}
}

func (m *DoctorModel) Activate() tea.Cmd {
	m.active = true
	m.runToken++
	m.checks = doctorChecksTemplate()
	return m.runChecks()
}

func (m *DoctorModel) Deactivate() { m.active = false }
func (m *DoctorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m DoctorModel) Update(msg tea.Msg) (DoctorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "r" {
			m.runToken++
			m.checks = doctorChecksTemplate()
			return m, m.runChecks()
		}
	case doctorCheckMsg:
		if msg.token != m.runToken {
			return m, nil
		}
		for idx := range m.checks {
			if m.checks[idx].name == msg.check.name {
				m.checks[idx] = msg.check
			}
		}
		m.checkedAt = time.Now()
	}
	return m, nil
}

func (m DoctorModel) View() string {
	lines := []string{fmt.Sprintf("System Health                          last checked %s · r re-run", relativeCheckTime(m.checkedAt)), ""}
	for _, check := range m.checks {
		if check.pending {
			lines = append(lines, fmt.Sprintf("…  %-18s pending", check.name))
			continue
		}
		prefix := mutedText("—")
		switch check.status {
		case "pass":
			prefix = successText("✓")
		case "warn":
			prefix = warnText("⚠")
		case "fail":
			prefix = errorText("✗")
		}
		lines = append(lines, fmt.Sprintf("%s  %-18s %s", prefix, check.name, check.detail))
		if check.recovery != "" {
			lines = append(lines, "   "+check.recovery)
		}
	}
	return strings.Join(lines, "\n")
}

func (m DoctorModel) HelpView() string { return "Doctor\nr re-run checks" }

func (m DoctorModel) runChecks() tea.Cmd {
	token := m.runToken
	cmds := []tea.Cmd{
		runDoctorCheckCmd(token, func() doctorCheck {
			ok, err := systemDockerFn()
			switch {
			case err != nil:
				return doctorCheck{name: "Docker", status: "fail", detail: err.Error()}
			case ok:
				version := "running"
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if out, err := dockerOutputFn(ctx, "version", "--format", "{{.Server.Version}}"); err == nil {
					if text := strings.TrimSpace(string(out)); text != "" {
						version = "running (v" + text + ")"
					}
				}
				return doctorCheck{name: "Docker", status: "pass", detail: version}
			default:
				return doctorCheck{name: "Docker", status: "fail", detail: "not running"}
			}
		}),
		runDoctorCheckCmd(token, func() doctorCheck {
			ok, err := systemGroupFn()
			switch {
			case err != nil:
				return doctorCheck{name: "Docker group", status: "fail", detail: err.Error()}
			case ok:
				return doctorCheck{name: "Docker group", status: "pass", detail: "user in docker group"}
			default:
				return doctorCheck{name: "Docker group", status: "warn", detail: "user not in docker group"}
			}
		}),
		runDoctorCheckCmd(token, func() doctorCheck {
			ok, err := systemInternetFn()
			switch {
			case err != nil:
				return doctorCheck{name: "Internet", status: "warn", detail: err.Error()}
			case ok:
				return doctorCheck{name: "Internet", status: "pass", detail: "reachable"}
			default:
				return doctorCheck{name: "Internet", status: "warn", detail: "not reachable"}
			}
		}),
		runDoctorCheckCmd(token, doctorGPUCheck),
		runDoctorCheckCmd(token, doctorWorkspaceCheck),
	}
	for _, suiteName := range viewOrder {
		name := suiteName
		cmds = append(cmds, runDoctorCheckCmd(token, func() doctorCheck {
			return doctorSuiteCheck(name)
		}))
	}
	return tea.Batch(cmds...)
}

func runDoctorCheckCmd(token int, fn func() doctorCheck) tea.Cmd {
	return func() tea.Msg {
		return doctorCheckMsg{token: token, check: fn()}
	}
}

func doctorGPUCheck() doctorCheck {
	state, err := gpuDetectFn()
	if err != nil {
		return doctorCheck{name: "GPU", status: "warn", detail: err.Error()}
	}
	switch state {
	case gpu.GPUStateNVIDIA:
		detail := "NVIDIA detected"
		if devices, err := doctorGPUStatsFn(); err == nil && len(devices) > 0 {
			device := devices[0]
			detail = fmt.Sprintf("%s · driver %s", device.Name, device.DriverVersion)
		} else if branch, err := gpuRecommendedFn(); err == nil {
			detail = "NVIDIA detected · driver " + branch
		}
		if cudaVersion, err := doctorCUDAFn(); err == nil && cudaVersion != "" {
			detail += " · CUDA " + cudaVersion
		}
		if ok, _ := gpuToolkitFn(); ok {
			detail += " · toolkit configured"
		} else {
			detail += " · toolkit not configured"
		}
		return doctorCheck{name: "GPU", status: "pass", detail: detail}
	case gpu.GPUStateAMD:
		return doctorCheck{name: "GPU", status: "warn", detail: "AMD detected — ROCm support coming in v0.3"}
	default:
		return doctorCheck{name: "GPU", status: "warn", detail: "not detected — running in CPU-only mode"}
	}
}

func doctorWorkspaceCheck() doctorCheck {
	if err := workspaceEnsureFn(); err != nil {
		return doctorCheck{name: "Workspace", status: "fail", detail: err.Error()}
	}
	entries := []string{"data", "notebooks", "models", "outputs", "mlruns", "dags", "compose", "config", "backups"}
	for _, entry := range entries {
		if _, err := os.Stat(workspaceRootFn() + "/" + entry); err != nil {
			return doctorCheck{name: "Workspace", status: "warn", detail: "missing " + entry}
		}
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(workspaceRootFn(), &stat); err != nil {
		return doctorCheck{name: "Workspace", status: "warn", detail: workspaceRootFn() + " · size unavailable"}
	}
	free := stat.Bavail * uint64(stat.Bsize)
	return doctorCheck{name: "Workspace", status: "pass", detail: workspaceRootFn() + " · all subdirs present · " + humanBytes(free) + " free"}
}

func doctorSuiteCheck(name string) doctorCheck {
	installed, err := isInstalledFn(name)
	if err != nil {
		return doctorCheck{name: name, status: "fail", detail: err.Error()}
	}
	if !installed {
		return doctorCheck{name: name, status: "skip", detail: "not installed"}
	}

	s, err := currentSuiteDefinition(name)
	if err != nil {
		return doctorCheck{name: name, status: "fail", detail: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	running := 0
	stopped := ""
	for _, container := range s.Containers {
		status, err := dockerStatusFn(ctx, container.Name)
		if err != nil {
			return doctorCheck{name: name, status: "fail", detail: err.Error()}
		}
		if status == "running" {
			running++
			continue
		}
		if stopped == "" {
			stopped = container.Name + " " + status
		}
	}

	switch {
	case running == len(s.Containers):
		return doctorCheck{name: name, status: "pass", detail: fmt.Sprintf("%d / %d containers running", running, len(s.Containers))}
	case running == 0:
		return doctorCheck{
			name:     name,
			status:   "fail",
			detail:   fmt.Sprintf("0 / %d containers running", len(s.Containers)),
			recovery: "└─ run: concave start " + name,
		}
	default:
		recovery := "└─ run: concave start " + name
		if stopped != "" {
			recovery = "└─ " + stopped + " · run: concave start " + name
		}
		return doctorCheck{
			name:     name,
			status:   "warn",
			detail:   fmt.Sprintf("%d / %d containers running", running, len(s.Containers)),
			recovery: recovery,
		}
	}
}

func doctorChecksTemplate() []doctorCheck {
	checks := []doctorCheck{
		{name: "Docker", pending: true},
		{name: "Docker group", pending: true},
		{name: "Internet", pending: true},
		{name: "GPU", pending: true},
		{name: "Workspace", pending: true},
	}
	for _, name := range viewOrder {
		checks = append(checks, doctorCheck{name: name, pending: true})
	}
	return checks
}

func relativeCheckTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	delta := time.Since(t).Round(time.Second)
	if delta < time.Second {
		return "now"
	}
	return delta.String() + " ago"
}
