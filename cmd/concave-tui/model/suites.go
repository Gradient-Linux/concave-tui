package model

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

type suiteRow struct {
	Name      string
	Installed bool
	State     string
	Image     string
	Running   int
	Total     int
	Summary    apiclient.SuiteSummary
	Detail    suiteDetail
	Problem   string
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
	line          string
	done          bool
	err           error
	note          string
	url           string
	progressLabel string
	progressStep  int
	progressTotal int
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
	role          tuiauth.Role
	loading       bool
	selected      int
	forgePrompt   bool
	forgeCursor   int
	forgeOptions  []suite.ForgeOption
	forgeSelected map[string]bool
	confirmRemove bool
	confirmUpdate bool
	execPrompt    bool
	execInput     textinput.Model
	rows          []suiteRow
	lastErr       error
	note          string
	opLines       []string
	opProgress    int
	opTotal       int
	opLabel       string
	updatePreview []string
	opCh          <-chan suiteOperationMsg
}

func NewSuitesModel() SuitesModel {
	input := textinput.New()
	input.Placeholder = "command"
	input.Prompt = "exec> "
	return SuitesModel{loading: true, execInput: input}
}

func (m *SuitesModel) SetRole(role tuiauth.Role) {
	m.role = role
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
	if m.forgePrompt {
		switch typed := msg.(type) {
		case tea.KeyMsg:
			switch typed.String() {
			case "esc", "n":
				m.forgePrompt = false
				m.lastErr = nil
				return m, nil
			case "up", "k":
				if m.forgeCursor > 0 {
					m.forgeCursor--
				}
				return m, nil
			case "down", "j":
				if m.forgeCursor < len(m.forgeOptions)-1 {
					m.forgeCursor++
				}
				return m, nil
			case " ":
				if len(m.forgeOptions) > 0 {
					key := m.forgeOptions[m.forgeCursor].Key
					if m.forgeSelected[key] {
						delete(m.forgeSelected, key)
					} else {
						m.forgeSelected[key] = true
					}
				}
				return m, nil
			case "enter", "y":
				keys, err := m.selectedForgeKeys()
				if err != nil {
					m.lastErr = err
					return m, nil
				}
				m.forgePrompt = false
				m.lastErr = nil
				return m, m.startOperation("install", keys)
			}
		}
		return m, nil
	}

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
		if msg.msg.progressTotal > 0 {
			m.opProgress = msg.msg.progressStep
			m.opTotal = msg.msg.progressTotal
			m.opLabel = msg.msg.progressLabel
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
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.rows)-1 {
				m.selected++
			}
		case "g":
			m.selected = 0
		case "G":
			if len(m.rows) > 0 {
				m.selected = len(m.rows) - 1
			}
		case "esc", "n":
			m.confirmRemove = false
			m.confirmUpdate = false
			m.note = ""
		case "y":
			if m.confirmRemove {
				m.confirmRemove = false
				return m, m.startOperation("remove")
			}
			if m.confirmUpdate {
				m.confirmUpdate = false
				return m, m.startOperation("update")
			}
		case "i":
			if !tuiauth.Can(m.role, tuiauth.ActionInstall) {
				return m, nil
			}
			if row := m.currentRow(); row.Name == "forge" && !row.Installed {
				m.forgePrompt = true
				m.forgeCursor = 0
				m.forgeOptions = suiteForgeOptionsFn()
				if m.forgeSelected == nil {
					m.forgeSelected = make(map[string]bool)
				}
				clear(m.forgeSelected)
				m.lastErr = nil
				return m, nil
			}
			return m, m.startOperation("install")
		case "u":
			if tuiauth.Can(m.role, tuiauth.ActionUpdate) && m.currentRow().Installed {
				m.confirmUpdate = true
				m.updatePreview = m.updateDiffLines(m.currentRow())
			}
		case "R":
			if !tuiauth.Can(m.role, tuiauth.ActionRollback) {
				return m, nil
			}
			return m, m.startOperation("rollback")
		case "s":
			if !tuiauth.Can(m.role, tuiauth.ActionStart) {
				return m, nil
			}
			return m, m.startOperation("start")
		case "x":
			if !tuiauth.Can(m.role, tuiauth.ActionStop) {
				return m, nil
			}
			return m, m.startOperation("stop")
		case "S":
			if !tuiauth.Can(m.role, tuiauth.ActionStop) || !tuiauth.Can(m.role, tuiauth.ActionStart) {
				return m, nil
			}
			return m, m.startOperation("restart")
		case "l":
			if !tuiauth.Can(m.role, tuiauth.ActionOpenLab) {
				return m, nil
			}
			return m, m.startOperation("lab")
		case "b":
			if !tuiauth.Can(m.role, tuiauth.ActionShell) {
				return m, nil
			}
			return m, m.openShell()
		case "e":
			if !tuiauth.Can(m.role, tuiauth.ActionExec) {
				return m, nil
			}
			m.execPrompt = true
			m.execInput.Focus()
			return m, nil
		case "r":
			if tuiauth.Can(m.role, tuiauth.ActionRemove) && m.currentRow().Installed {
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

	leftWidth := max(24, (m.width*30)/100)
	rightWidth := max(36, m.width-leftWidth-3)
	leftLines := []string{"Suites"}
	for idx, row := range m.rows {
		prefix := " "
		style := lipgloss.NewStyle()
		if idx == m.selected {
			prefix = "►"
			style = style.Foreground(lipgloss.Color(ColorGold)).Bold(true)
		}
		leftLines = append(leftLines, style.Render(fmt.Sprintf("%s %-10s %-12s", prefix, row.Name, row.State)))
	}
	rightLines := m.detailView(m.currentRow())
	if m.forgePrompt {
		rightLines = m.renderForgePicker()
	}
	if m.confirmUpdate {
		rightLines = append(rightLines, "", "Update "+m.currentRow().Name+"?", "")
		rightLines = append(rightLines, m.updatePreview...)
		rightLines = append(rightLines, "", "[y] confirm  [n] cancel")
	}
	if m.confirmRemove {
		rightLines = append(rightLines, "", "Remove "+m.currentRow().Name+"? Your data in ~/gradient/ will not be touched.", "y confirm · n cancel")
	}
	if m.opTotal > 0 {
		rightLines = append(rightLines, "", m.renderOperationProgress(rightWidth))
	}
	if len(m.opLines) > 0 {
		rightLines = append(rightLines, "", "Progress:")
		for _, line := range m.opLines {
			rightLines = append(rightLines, "  "+line)
		}
	}
	if m.execPrompt {
		rightLines = append(rightLines, "", m.execInput.View())
	}
	if m.note != "" {
		rightLines = append(rightLines, "", successText(m.note))
	}
	if m.lastErr != nil {
		rightLines = append(rightLines, "", errorText(m.lastErr.Error()))
	}
	left := lipgloss.NewStyle().Width(leftWidth).Render(strings.Join(leftLines, "\n"))
	right := lipgloss.NewStyle().Width(rightWidth).Render(strings.Join(rightLines, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m SuitesModel) HelpView() string {
	return "Suites\nj/k move"
}

func (m SuitesModel) currentRow() suiteRow {
	if len(m.rows) == 0 {
		return suiteRow{}
	}
	return m.rows[m.selected]
}

func (m SuitesModel) detailView(row suiteRow) []string {
	lines := []string{fmt.Sprintf("%s   %s", row.Name, row.State)}
	if row.Problem != "" {
		lines = append(lines, "", row.Problem)
		if row.Installed && tuiauth.Can(m.role, tuiauth.ActionRemove) {
			lines = append(lines, "", "[r]remove stale install")
		}
		lines = append(lines, mutedText("Clear the stale suite state in the TUI, then retry the install."))
		return lines
	}
	lines = append(lines, "", "Container              Status    Image")
	for _, container := range row.Summary.Containers {
		lines = append(lines, fmt.Sprintf("%-22s %-9s %s", container.Name, container.Status, truncate(firstNonEmpty(container.Current, container.Image), 24)))
		previous := container.Previous
		if previous != "" {
			lines = append(lines, fmt.Sprintf("  Previous %-14s %s", "", previous))
		}
	}
	if len(row.Summary.Ports) > 0 {
		lines = append(lines, "", "Ports")
		for _, mapping := range row.Summary.Ports {
			port := mapping.Port
			if port == 0 {
				port = mapping.Host
			}
			service := mapping.Service
			if service == "" {
				service = mapping.Description
			}
			lines = append(lines, fmt.Sprintf("  :%d · %s", port, service))
		}
	}
	if row.Name == "forge" && !row.Installed {
		if tuiauth.Can(m.role, tuiauth.ActionInstall) {
			lines = append(lines, "", "[i]install custom selection")
		}
		return lines
	}
	actions := []string{}
	if tuiauth.Can(m.role, tuiauth.ActionInstall) {
		actions = append(actions, "[i]install")
	}
	if tuiauth.Can(m.role, tuiauth.ActionRemove) {
		actions = append(actions, "[r]remove")
	}
	if tuiauth.Can(m.role, tuiauth.ActionUpdate) {
		actions = append(actions, "[u]update")
	}
	if tuiauth.Can(m.role, tuiauth.ActionRollback) {
		actions = append(actions, "[R]rollback")
	}
	if len(actions) > 0 {
		lines = append(lines, "", strings.Join(actions, " "))
	}
	actions = nil
	if tuiauth.Can(m.role, tuiauth.ActionStart) {
		actions = append(actions, "[s]start")
	}
	if tuiauth.Can(m.role, tuiauth.ActionStop) {
		actions = append(actions, "[x]stop")
	}
	if tuiauth.Can(m.role, tuiauth.ActionShell) {
		actions = append(actions, "[b]shell")
	}
	if tuiauth.Can(m.role, tuiauth.ActionExec) {
		actions = append(actions, "[e]exec")
	}
	if tuiauth.Can(m.role, tuiauth.ActionOpenLab) {
		actions = append(actions, "[l]lab")
	}
	if len(actions) > 0 {
		lines = append(lines, strings.Join(actions, "  "))
	}
	return lines
}

func (m SuitesModel) updateDiffLines(row suiteRow) []string {
	lines := make([]string, 0, len(row.Summary.Containers))
	for _, container := range row.Summary.Containers {
		current := firstNonEmpty(container.Current, container.Image)
		lines = append(lines, fmt.Sprintf("%-24s %s  →  %s", container.Name, current, container.Image))
	}
	return lines
}

func loadSuitesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		suites, err := apiSuitesFn(ctx)
		if err != nil {
			return suitesLoadedMsg{err: err}
		}
		rows := make([]suiteRow, 0, len(suites))
		for _, summary := range suites {
			row := suiteRow{
				Name:      summary.Name,
				Installed: summary.Installed,
				Summary:   summary,
			}
			if len(summary.Containers) > 0 {
				row.Image = summary.Containers[0].Image
			}
			for _, container := range summary.Containers {
				if container.Status == "running" {
					row.Running++
				}
			}
			row.Total = len(summary.Containers)
			switch summary.State {
			case "running":
				row.State = "● running"
			case "degraded":
				row.State = fmt.Sprintf("⚠ %d/%d", row.Running, row.Total)
			case "unconfigured":
				row.State = "⚠ unconfigured"
				row.Problem = summary.Error
			case "not-installed":
				row.State = "— not installed"
			default:
				row.State = "○ stopped"
			}
			rows = append(rows, row)
		}
		return suitesLoadedMsg{rows: rows}
	}
}

func (m *SuitesModel) startOperation(kind string, forgeKeys ...[]string) tea.Cmd {
	if len(m.rows) == 0 || m.opCh != nil {
		return nil
	}
	ch := make(chan suiteOperationMsg, 32)
	m.opCh = ch
	m.opLines = nil
	m.note = ""
	m.lastErr = nil
	m.opProgress = 0
	m.opTotal = 0
	m.opLabel = ""
	name := m.currentRow().Name
	var keys []string
	if len(forgeKeys) > 0 {
		keys = forgeKeys[0]
	}
	go func() {
		defer close(ch)
		runSuiteOperation(kind, name, keys, ch)
	}()
	return waitForSuiteOperation(ch)
}

func waitForSuiteOperation(ch <-chan suiteOperationMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		return suiteOperationEnvelope{msg: msg, ch: ch, ok: ok}
	}
}

func runSuiteOperation(kind, name string, forgeKeys []string, ch chan<- suiteOperationMsg) {
	send := func(line string) { ch <- suiteOperationMsg{line: line} }
	progress := func(label string, step, total int) {
		ch <- suiteOperationMsg{progressLabel: label, progressStep: step, progressTotal: total}
	}
	done := func(note string) { ch <- suiteOperationMsg{done: true, note: note} }
	fail := func(err error) { ch <- suiteOperationMsg{done: true, err: err} }

	switch kind {
	case "install":
		failOrDone(performInstall(name, forgeKeys, send, progress), name+" installed", fail, done)
	case "update":
		failOrDone(performUpdate(name, send, progress), name+" updated", fail, done)
	case "rollback":
		failOrDone(performRollback(name, send, progress), name+" rolled back", fail, done)
	case "start":
		failOrDone(performStart(name, progress), name+" started", fail, done)
	case "stop":
		failOrDone(performStop(name, progress), name+" stopped", fail, done)
	case "restart":
		failOrDone(performRestart(name, progress), name+" restarted", fail, done)
	case "remove":
		failOrDone(performRemove(name, progress), name+" removed", fail, done)
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

func performInstall(name string, forgeKeys []string, send func(string), progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	body := any(nil)
	if name == "forge" && len(forgeKeys) > 0 {
		body = map[string][]string{"forge_components": forgeKeys}
	}
	return runServerSuiteJob(name, "install", body, send, progress)
}

func performUpdate(name string, send func(string), progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	return runServerSuiteJob(name, "update", nil, send, progress)
}

func performRollback(name string, send func(string), progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	return runServerSuiteJob(name, "rollback", nil, send, progress)
}

func performStart(name string, progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	return runServerSuiteJob(name, "start", nil, nil, progress)
}

func performStop(name string, progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	return runServerSuiteJob(name, "stop", nil, nil, progress)
}

func performRestart(name string, progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	if err := performStop(name, func(_ string, step, total int) {
		progress("Restarting", step, total*2)
	}); err != nil {
		return err
	}
	return performStart(name, func(_ string, step, total int) {
		progress("Restarting", total+step, total*2)
	})
}

func performRemove(name string, progressFns ...func(string, int, int)) error {
	progress := func(string, int, int) {}
	if len(progressFns) > 0 && progressFns[0] != nil {
		progress = progressFns[0]
	}
	return runServerSuiteJob(name, "remove", nil, nil, progress)
}

func installConflicts(_ suite.Suite, _ []string) ([]suite.PortConflict, error) { return nil, nil }

func cleanupSuiteDefinition(name string) (suite.Suite, error) {
	s, err := currentSuiteDefinition(name)
	if err == nil {
		return s, nil
	}
	if name == "forge" && isMissingForgeSelection(err) {
		return suiteGetFn(name)
	}
	return suite.Suite{}, err
}

func cleanupSuiteContainers(ctx context.Context, name string) error {
	s, err := cleanupSuiteDefinition(name)
	if err != nil {
		return err
	}
	if len(s.Containers) == 0 {
		return nil
	}
	args := make([]string, 0, len(s.Containers)+2)
	args = append(args, "rm", "-f")
	for _, container := range s.Containers {
		args = append(args, container.Name)
	}
	output, err := dockerOutputFn(ctx, args...)
	if err != nil {
		text := strings.ToLower(err.Error() + " " + string(output))
		if !strings.Contains(text, "no such") {
			return err
		}
	}
	return nil
}

func (m SuitesModel) renderForgePicker() []string {
	lines := []string{
		"forge   custom install",
		"",
		"Select the Forge components you want to install.",
		"",
	}
	for idx, option := range m.forgeOptions {
		cursor := " "
		if idx == m.forgeCursor {
			cursor = "►"
		}
		marker := "□"
		if m.forgeSelected[option.Key] {
			marker = "■"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s", cursor, marker, option.Label))
	}
	lines = append(lines, "", "[space] toggle  [enter] install  [esc] cancel")
	return lines
}

func (m SuitesModel) selectedForgeSelection() (suite.ForgeSelection, error) {
	keys := make([]string, 0, len(m.forgeSelected))
	for _, option := range m.forgeOptions {
		if m.forgeSelected[option.Key] {
			keys = append(keys, option.Key)
		}
	}
	return suiteForgeSelectFn(keys)
}

func (m SuitesModel) selectedForgeKeys() ([]string, error) {
	keys := make([]string, 0, len(m.forgeSelected))
	for _, option := range m.forgeOptions {
		if m.forgeSelected[option.Key] {
			keys = append(keys, option.Key)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no components selected")
	}
	return keys, nil
}

func (m SuitesModel) openShell() tea.Cmd {
	row := m.currentRow()
	if !row.Installed {
		return func() tea.Msg {
			return suiteOperationMsg{done: true, err: fmt.Errorf("suite %s is not installed", row.Name)}
		}
	}
	container := suitePrimaryFn(row.Detail.Suite)
	if container == "" && len(row.Summary.Containers) > 0 {
		container = row.Summary.Containers[0].Name
	}
	cmd := exec.Command("docker", "exec", "-it", container, "sh")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return suiteOperationMsg{done: true, err: err, note: "shell exited"}
	})
}

func (m SuitesModel) execSuiteCommand(input string) tea.Cmd {
	row := m.currentRow()
	container := ""
	if len(row.Summary.Containers) > 0 {
		container = row.Summary.Containers[0].Name
	}
	args := strings.Fields(input)
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command("docker", append([]string{"exec", "-it", container}, args...)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return suiteOperationMsg{done: true, err: err, note: "exec finished"}
	})
}

func runServerSuiteJob(name, action string, body any, send func(string), progress func(string, int, int)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	jobID, err := apiSuiteActionFn(ctx, name, action, body)
	if err != nil {
		return err
	}
	seen := 0
	step := 5
	progress(strings.Title(action), step, 100)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
		job, err := apiJobFn(reqCtx, jobID)
		reqCancel()
		if err != nil {
			return err
		}
		for _, line := range job.Lines[seen:] {
			if send != nil {
				send(line)
			}
			if step < 90 {
				step += 4
			}
		}
		seen = len(job.Lines)
		progress(strings.Title(action), step, 100)
		switch job.Status {
		case "completed":
			progress(strings.Title(action), 100, 100)
			return nil
		case "failed":
			if job.Error == "" {
				job.Error = "job failed"
			}
			return fmt.Errorf("%s", job.Error)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func (m SuitesModel) renderOperationProgress(width int) string {
	if m.opTotal <= 0 {
		return ""
	}
	ratio := float64(m.opProgress) / float64(max(1, m.opTotal))
	barWidth := max(16, min(40, width-18))
	return fmt.Sprintf("%s  %s  %3d%%", m.opLabel, gradientBar(barWidth, ratio, false), int(ratio*100))
}
