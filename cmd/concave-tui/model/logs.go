package model

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const logBufferCap = 1000

type logTarget struct {
	Suite     string
	Container string
}

type dockerLogEvent struct {
	line string
	err  error
	done bool
}

type logsLoadedMsg struct {
	targets []logTarget
	err     error
}

type logEnvelope struct {
	event dockerLogEvent
	ch    <-chan dockerLogEvent
	ok    bool
}

func startDockerLogStream(container string) (func(), <-chan dockerLogEvent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", container)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, err
	}

	ch := make(chan dockerLogEvent, 128)
	go func() {
		defer close(ch)
		relay := func(reader io.Reader) {
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				ch <- dockerLogEvent{line: scanner.Text()}
			}
		}
		go relay(stdout)
		relay(stderr)
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			ch <- dockerLogEvent{err: err}
		}
		ch <- dockerLogEvent{done: true}
	}()
	return cancel, ch, nil
}

type LogsModel struct {
	width       int
	height      int
	active      bool
	loading     bool
	targets     []logTarget
	selected    int
	lines       []string
	viewport    viewport.Model
	searchInput textinput.Model
	searching   bool
	follow      bool
	lastErr     error
	streamCh    <-chan dockerLogEvent
	cancel      func()
}

func NewLogsModel() LogsModel {
	search := textinput.New()
	search.Prompt = "/"
	search.Placeholder = "search"
	vp := viewport.New(40, 10)
	return LogsModel{loading: true, follow: true, viewport: vp, searchInput: search}
}

func (m *LogsModel) Activate() tea.Cmd {
	m.active = true
	m.loading = true
	return loadLogsTargetsCmd()
}

func (m *LogsModel) Deactivate() {
	m.active = false
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m *LogsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	leftWidth := 22
	rightWidth := width - leftWidth - 4
	if rightWidth < 20 {
		rightWidth = 20
	}
	viewHeight := height - 4
	if viewHeight < 6 {
		viewHeight = 6
	}
	m.viewport.Width = rightWidth
	m.viewport.Height = viewHeight
}

func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	if m.searching {
		switch typed := msg.(type) {
		case tea.KeyMsg:
			switch typed.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				m.syncViewport()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case logsLoadedMsg:
		m.loading = false
		m.targets = msg.targets
		m.lastErr = msg.err
		if len(m.targets) > 0 {
			return m, m.startStreamForSelection()
		}
	case logEnvelope:
		if m.streamCh == nil || m.streamCh != msg.ch {
			return m, nil
		}
		if !msg.ok {
			m.streamCh = nil
			return m, nil
		}
		if msg.event.err != nil {
			m.lastErr = msg.event.err
		}
		if msg.event.line != "" {
			m.lines = append(m.lines, msg.event.line)
			if len(m.lines) > logBufferCap {
				m.lines = m.lines[len(m.lines)-logBufferCap:]
			}
			m.syncViewport()
		}
		if msg.event.done {
			m.streamCh = nil
			return m, nil
		}
		return m, waitForLogEvent(msg.ch)
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.follow {
				m.follow = false
			}
			if m.selected > 0 {
				m.selected--
				m.lines = nil
				return m, m.startStreamForSelection()
			}
		case "down":
			if m.selected < len(m.targets)-1 {
				m.selected++
				m.lines = nil
				return m, m.startStreamForSelection()
			}
		case "/":
			m.searching = true
			m.searchInput.Focus()
			return m, nil
		case "f":
			m.follow = true
			m.syncViewport()
		case "pgup":
			m.follow = false
			m.viewport.HalfViewUp()
		case "pgdown":
			m.viewport.HalfViewDown()
		}
	}
	return m, nil
}

func (m LogsModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("Loading logs…")
	}
	if m.lastErr != nil {
		return errorText(m.lastErr.Error())
	}
	if len(m.targets) == 0 {
		return mutedText("No installed suites with containers to stream")
	}

	leftWidth := 22
	if m.width > 0 && m.width < minWidth {
		leftWidth = 18
	}
	leftLines := make([]string, 0, len(m.targets))
	for idx, target := range m.targets {
		prefix := "·"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
		if idx == m.selected {
			prefix = "►"
			style = style.Foreground(lipgloss.Color(ColorGold)).Bold(true)
		}
		leftLines = append(leftLines, style.Render(fmt.Sprintf("%s %s", prefix, target.Container)))
	}
	left := lipgloss.NewStyle().Width(leftWidth).Render(strings.Join(leftLines, "\n"))
	rightLines := []string{
		fmt.Sprintf("%s              %s", m.targets[m.selected].Suite, m.targets[m.selected].Container),
		strings.Repeat("─", max(10, m.viewport.Width)),
		m.viewport.View(),
	}
	if m.searching {
		rightLines = append(rightLines, "", m.searchInput.View())
	}
	right := strings.Join(rightLines, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m LogsModel) HelpView() string {
	return "Logs\n↑/↓ container · / search · f follow · pgup/pgdown scroll"
}

func loadLogsTargetsCmd() tea.Cmd {
	return func() tea.Msg {
		state, err := loadStateFn()
		if err != nil {
			return logsLoadedMsg{err: err}
		}
		targets := make([]logTarget, 0)
		for _, name := range orderedInstalledSuites(state.Installed) {
			s, err := currentSuiteDefinition(name)
			if err != nil {
				return logsLoadedMsg{err: err}
			}
			for _, container := range s.Containers {
				targets = append(targets, logTarget{Suite: name, Container: container.Name})
			}
		}
		return logsLoadedMsg{targets: targets}
	}
}

func (m *LogsModel) startStreamForSelection() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if len(m.targets) == 0 {
		return nil
	}
	cancel, ch, err := dockerLogStreamFn(m.targets[m.selected].Container)
	if err != nil {
		return func() tea.Msg { return logsLoadedMsg{err: err} }
	}
	m.cancel = cancel
	m.streamCh = ch
	m.follow = true
	m.syncViewport()
	return waitForLogEvent(ch)
}

func waitForLogEvent(ch <-chan dockerLogEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		return logEnvelope{event: event, ch: ch, ok: ok}
	}
}

func (m *LogsModel) syncViewport() {
	lines := m.lines
	query := strings.TrimSpace(m.searchInput.Value())
	if query != "" {
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				filtered = append(filtered, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(line))
			} else {
				filtered = append(filtered, mutedText(line))
			}
		}
		lines = filtered
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
	if m.follow {
		m.viewport.GotoBottom()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
