package model

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
	"github.com/Gradient-Linux/concave-tui/internal/system"
)

type suiteRow struct {
	Name      string
	Installed bool
	State     string
	Image     string
	Running   int
	Total     int
	Detail    suiteDetail
}

type suiteDetail struct {
	Suite    suite.Suite
	Current  map[string]string
	Previous map[string]string
}

type suitesLoadedMsg struct {
	rows []suiteRow
	err  error
}

type suiteOperationMsg struct {
	line string
	done bool
	err  error
	note string
	url  string
}

type suiteOperationEnvelope struct {
	msg suiteOperationMsg
	ch  <-chan suiteOperationMsg
	ok  bool
}

var runComposeRemoveFn = func(ctx context.Context, composePath string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "down", "--rmi", "all")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type SuitesModel struct {
	width         int
	height        int
	active        bool
	loading       bool
	selected      int
	showDetail    bool
	confirmRemove bool
	execPrompt    bool
	execInput     textinput.Model
	rows          []suiteRow
	lastErr       error
	note          string
	opLines       []string
	opCh          <-chan suiteOperationMsg
}

func NewSuitesModel() SuitesModel {
	input := textinput.New()
	input.Placeholder = "command"
	input.Prompt = "exec> "
	return SuitesModel{loading: true, execInput: input}
}

func (m *SuitesModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	return loadSuitesCmd()
}

func (m *SuitesModel) Deactivate() { m.active = false }
func (m *SuitesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m SuitesModel) Update(msg tea.Msg) (SuitesModel, tea.Cmd) {
	if m.execPrompt {
		var cmd tea.Cmd
		switch typed := msg.(type) {
		case tea.KeyMsg:
			switch typed.String() {
			case "esc":
				m.execPrompt = false
				m.execInput.Blur()
				m.execInput.SetValue("")
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.execInput.Value())
				m.execPrompt = false
				m.execInput.Blur()
				m.execInput.SetValue("")
				if value == "" {
					return m, nil
				}
				return m, m.execSuiteCommand(value)
			}
		}
		m.execInput, cmd = m.execInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case suitesLoadedMsg:
		m.loading = false
		m.rows = msg.rows
		m.lastErr = msg.err
		if m.selected >= len(m.rows) && len(m.rows) > 0 {
			m.selected = len(m.rows) - 1
		}
		return m, nil
	case suiteOperationEnvelope:
		if m.opCh == nil || msg.ch != m.opCh {
			return m, nil
		}
		if !msg.ok {
			m.opCh = nil
			m.loading = true
			return m, loadSuitesCmd()
		}
		if msg.msg.line != "" {
			m.opLines = append(m.opLines, msg.msg.line)
			if len(m.opLines) > 12 {
				m.opLines = m.opLines[len(m.opLines)-12:]
			}
		}
		if msg.msg.err != nil {
			m.lastErr = msg.msg.err
		}
		if msg.msg.note != "" {
			m.note = msg.msg.note
		}
		if msg.msg.done {
			m.opCh = nil
			return m, loadSuitesCmd()
		}
		return m, waitForSuiteOperation(msg.ch)
	case suiteOperationMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		if msg.note != "" {
			m.note = msg.note
		}
		return m, loadSuitesCmd()
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < len(m.rows)-1 {
				m.selected++
			}
		case "enter":
			m.showDetail = !m.showDetail
		case "esc", "n":
			m.confirmRemove = false
			m.note = ""
		case "y":
			if m.confirmRemove {
				m.confirmRemove = false
				return m, m.startOperation("remove")
			}
		case "i":
			return m, m.startOperation("install")
		case "u":
			return m, m.startOperation("update")
		case "R":
			return m, m.startOperation("rollback")
		case "s":
			return m, m.startOperation("start")
		case "x":
			return m, m.startOperation("stop")
		case "S":
			return m, m.startOperation("restart")
		case "l":
			return m, m.startOperation("lab")
		case "b":
			return m, m.openShell()
		case "e":
			m.execPrompt = true
			m.execInput.Focus()
			return m, nil
		case "r":
			if m.currentRow().Installed {
				m.confirmRemove = true
			}
		}
	}
	return m, nil
}

func (m SuitesModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading suites…")
	}
	if m.lastErr != nil {
		return errorText(m.lastErr.Error())
	}

	lines := make([]string, 0, len(m.rows)+10)
	for idx, row := range m.rows {
		prefix := " "
		style := lipgloss.NewStyle()
		if idx == m.selected {
			prefix = "►"
			style = style.Foreground(lipgloss.Color(ColorGold)).Bold(true)
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s %-10s %-12s %-24s", prefix, row.Name, row.State, truncate(row.Image, 24))))
		if idx == m.selected {
			lines = append(lines, "  [i] install [r] remove [u] update [R] rollback [s] start [x] stop [S] restart")
			if m.showDetail {
				lines = append(lines, m.detailView(row)...)
			}
			if m.confirmRemove {
				lines = append(lines, "  Remove "+row.Name+"? Your data in ~/gradient/ will not be touched. y confirm · n cancel")
			}
			if len(m.opLines) > 0 {
				lines = append(lines, "  Progress:")
				for _, line := range m.opLines {
					lines = append(lines, "    "+line)
				}
			}
			if m.execPrompt {
				lines = append(lines, "  "+m.execInput.View())
			}
		}
	}
	if m.note != "" {
		lines = append(lines, "", successText(m.note))
	}
	return strings.Join(lines, "\n")
}

func (m SuitesModel) HelpView() string {
	return "Suites\n↑/↓ move · enter details · i install · r remove · u update · R rollback · s/x start stop · b shell · e exec · l lab"
}

func (m SuitesModel) currentRow() suiteRow {
	if len(m.rows) == 0 {
		return suiteRow{}
	}
	return m.rows[m.selected]
}

func (m SuitesModel) detailView(row suiteRow) []string {
	lines := []string{"  Containers:"}
	for _, container := range row.Detail.Suite.Containers {
		current := row.Detail.Current[container.Name]
		previous := row.Detail.Previous[container.Name]
		versionText := current
		if previous != "" {
			versionText += " (prev " + previous + ")"
		}
		lines = append(lines, "    "+container.Name+" · "+versionText)
	}
	if len(row.Detail.Suite.Ports) > 0 {
		lines = append(lines, "  Ports:")
		for _, mapping := range row.Detail.Suite.Ports {
			lines = append(lines, fmt.Sprintf("    :%d · %s", mapping.Port, mapping.Service))
		}
	}
	if len(row.Detail.Suite.Volumes) > 0 {
		lines = append(lines, "  Volumes:")
		for _, mount := range row.Detail.Suite.Volumes {
			lines = append(lines, fmt.Sprintf("    %s -> %s", mount.HostPath, mount.ContainerPath))
		}
	}
	lines = append(lines, "  Actions: s start · x stop · S restart · b shell · e exec · l lab")
	return lines
}

func loadSuitesCmd() tea.Cmd {
	return func() tea.Msg {
		state, err := loadStateFn()
		if err != nil {
			return suitesLoadedMsg{err: err}
		}
		manifest, err := loadManifestFn()
		if err != nil {
			return suitesLoadedMsg{err: err}
		}

		installed := make(map[string]struct{}, len(state.Installed))
		for _, name := range state.Installed {
			installed[name] = struct{}{}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		rows := make([]suiteRow, 0, len(viewOrder))
		for _, name := range viewOrder {
			row := suiteRow{Name: name}
			if _, ok := installed[name]; ok {
				row.Installed = true
				row.Image = currentImageForFirstContainer(name, manifest)
				s, err := currentSuiteDefinition(name)
				if err != nil {
					return suitesLoadedMsg{err: err}
				}
				row.Detail = suiteDetail{
					Suite:    s,
					Current:  map[string]string{},
					Previous: map[string]string{},
				}
				for _, container := range s.Containers {
					status, statusErr := dockerStatusFn(ctx, container.Name)
					if statusErr != nil {
						status = "error"
					}
					if status == "running" {
						row.Running++
					}
				}
				row.Total = len(s.Containers)
				switch {
				case row.Running == row.Total && row.Total > 0:
					row.State = "● running"
				case row.Running > 0:
					row.State = fmt.Sprintf("⚠ %d/%d", row.Running, row.Total)
				default:
					row.State = "○ stopped"
				}
				for containerName, version := range manifest[name] {
					row.Detail.Current[containerName] = version.Current
					row.Detail.Previous[containerName] = version.Previous
				}
			} else {
				row.State = "— not installed"
				if s, err := suiteGetFn(name); err == nil && len(s.Containers) > 0 {
					row.Image = s.Containers[0].Image
					row.Detail.Suite = s
				}
			}
			rows = append(rows, row)
		}
		return suitesLoadedMsg{rows: rows}
	}
}

func (m *SuitesModel) startOperation(kind string) tea.Cmd {
	if len(m.rows) == 0 || m.opCh != nil {
		return nil
	}
	ch := make(chan suiteOperationMsg, 32)
	m.opCh = ch
	m.opLines = nil
	m.note = ""
	m.lastErr = nil
	name := m.currentRow().Name
	go func() {
		defer close(ch)
		runSuiteOperation(kind, name, ch)
	}()
	return waitForSuiteOperation(ch)
}

func waitForSuiteOperation(ch <-chan suiteOperationMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		return suiteOperationEnvelope{msg: msg, ch: ch, ok: ok}
	}
}

func runSuiteOperation(kind, name string, ch chan<- suiteOperationMsg) {
	send := func(line string) { ch <- suiteOperationMsg{line: line} }
	done := func(note string) { ch <- suiteOperationMsg{done: true, note: note} }
	fail := func(err error) { ch <- suiteOperationMsg{done: true, err: err} }

	switch kind {
	case "install":
		failOrDone(performInstall(name, send), name+" installed", fail, done)
	case "update":
		failOrDone(performUpdate(name, send), name+" updated", fail, done)
	case "rollback":
		failOrDone(performRollback(name, send), name+" rolled back", fail, done)
	case "start":
		failOrDone(performStart(name), name+" started", fail, done)
	case "stop":
		failOrDone(performStop(name), name+" stopped", fail, done)
	case "restart":
		failOrDone(performRestart(name), name+" restarted", fail, done)
	case "remove":
		failOrDone(performRemove(name), name+" removed", fail, done)
	case "lab":
		url, err := openLabURL(name)
		if err != nil {
			fail(err)
			return
		}
		if err := systemOpenURLFn(url); err != nil {
			fail(err)
			return
		}
		ch <- suiteOperationMsg{done: true, note: "opened " + url}
	}
}

func failOrDone(err error, note string, fail func(error), done func(string)) {
	if err != nil {
		fail(err)
		return
	}
	done(note)
}

func performInstall(name string, send func(string)) error {
	s, err := suiteGetFn(name)
	if err != nil {
		return err
	}
	installed, err := isInstalledFn(name)
	if err != nil {
		return err
	}
	if installed {
		send("already installed")
		return nil
	}
	if s.GPURequired {
		state, err := gpuDetectFn()
		if err == nil && state != gpu.GPUStateNVIDIA {
			send("warning: NVIDIA GPU not detected")
		}
	}

	state, err := loadStateFn()
	if err != nil {
		return err
	}
	conflicts, err := system.CheckConflicts(s, state.Installed)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("port %d already used by %s", conflicts[0].Port, conflicts[0].ExistingSuite)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for _, container := range s.Containers {
		send("pulling " + container.Image)
		if err := dockerTagPreviousFn(container.Image); err != nil {
			return err
		}
		if err := dockerPullStreamFn(ctx, container.Image, func(line string) { send(line) }); err != nil {
			return err
		}
	}

	if name == "forge" {
		data, err := suiteBuildForgeFn(suite.ForgeSelection{Containers: s.Containers, Ports: s.Ports, Volumes: s.Volumes})
		if err != nil {
			return err
		}
		if _, err := dockerWriteRawFn(name, data); err != nil {
			return err
		}
	} else if _, err := dockerWriteComposeFn(name); err != nil {
		return err
	}

	manifest, err := loadManifestFn()
	if err != nil {
		return err
	}
	manifest = recordInstallFn(manifest, s)
	if err := saveManifestFn(manifest); err != nil {
		return err
	}
	return addSuiteFn(name)
}

func performUpdate(name string, send func(string)) error {
	installed, err := isInstalledFn(name)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("suite %s is not installed", name)
	}
	s, err := currentSuiteDefinition(name)
	if err != nil {
		return err
	}
	manifest, err := loadManifestFn()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for _, container := range s.Containers {
		send("pulling " + container.Image)
		if err := dockerTagPreviousFn(container.Image); err != nil {
			return err
		}
		if err := dockerPullStreamFn(ctx, container.Image, func(line string) { send(line) }); err != nil {
			return err
		}
		manifest = recordUpdateFn(manifest, name, container.Name, container.Image)
	}
	if err := saveManifestFn(manifest); err != nil {
		return err
	}
	if _, err := writeComposeForSuite(name); err != nil {
		return err
	}
	return dockerComposeUpFn(ctx, dockerComposePathFn(name), true)
}

func performRollback(name string, send func(string)) error {
	installed, err := isInstalledFn(name)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("suite %s is not installed", name)
	}
	manifest, err := loadManifestFn()
	if err != nil {
		return err
	}
	manifest, err = swapRollbackFn(manifest, name)
	if err != nil {
		return err
	}
	if err := saveManifestFn(manifest); err != nil {
		return err
	}
	send("restoring previous image tags")
	if _, err := writeComposeForSuite(name); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := dockerComposeDownFn(ctx, dockerComposePathFn(name)); err != nil {
		return err
	}
	return dockerComposeUpFn(ctx, dockerComposePathFn(name), true)
}

func performStart(name string) error {
	s, err := currentSuiteDefinition(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := dockerComposeUpFn(ctx, dockerComposePathFn(name), true); err != nil {
		return err
	}
	return systemRegisterFn(s)
}

func performStop(name string) error {
	s, err := currentSuiteDefinition(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := dockerComposeDownFn(ctx, dockerComposePathFn(name)); err != nil {
		return err
	}
	return systemDeregisterFn(s)
}

func performRestart(name string) error {
	if err := performStop(name); err != nil {
		return err
	}
	return performStart(name)
}

func performRemove(name string) error {
	installed, err := isInstalledFn(name)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("suite %s is not installed", name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := runComposeRemoveFn(ctx, dockerComposePathFn(name)); err != nil {
		return err
	}
	_ = os.Remove(dockerComposePathFn(name))
	if err := removeSuiteFn(name); err != nil {
		return err
	}
	manifest, err := loadManifestFn()
	if err != nil {
		return err
	}
	delete(manifest, name)
	if err := saveManifestFn(manifest); err != nil {
		return err
	}
	s, err := suiteGetFn(name)
	if err != nil {
		return err
	}
	return systemDeregisterFn(s)
}

func (m SuitesModel) openShell() tea.Cmd {
	row := m.currentRow()
	if !row.Installed {
		return func() tea.Msg {
			return suiteOperationMsg{done: true, err: fmt.Errorf("suite %s is not installed", row.Name)}
		}
	}
	container := suitePrimaryFn(row.Detail.Suite)
	cmd := exec.Command("docker", "exec", "-it", container, "sh")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return suiteOperationMsg{done: true, err: err, note: "shell exited"}
	})
}

func (m SuitesModel) execSuiteCommand(input string) tea.Cmd {
	row := m.currentRow()
	container := suitePrimaryFn(row.Detail.Suite)
	args := strings.Fields(input)
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command("docker", append([]string{"exec", "-it", container}, args...)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return suiteOperationMsg{done: true, err: err, note: "exec finished"}
	})
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	if limit <= 1 {
		return text[:limit]
	}
	return text[:limit-1] + "…"
}
