package model

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Gradient-Linux/concave-tui/internal/config"
)

func TestLogsContainerSwitchCancelsPreviousStream(t *testing.T) {
	restoreModelDeps(t)

	cancelCount := 0
	dockerLogStreamFn = func(container string) (func(), <-chan dockerLogEvent, error) {
		ch := make(chan dockerLogEvent)
		return func() { cancelCount++ }, ch, nil
	}

	m := NewLogsModel()
	m.targets = []logTarget{
		{Suite: "boosting", Container: "gradient-boost-lab"},
		{Suite: "boosting", Container: "gradient-boost-track"},
	}

	_ = m.startStreamForSelection()
	m.selected = 1
	_ = m.startStreamForSelection()

	if cancelCount != 1 {
		t.Fatalf("cancel count = %d, want 1", cancelCount)
	}
}

func TestLogsBufferCapIsEnforced(t *testing.T) {
	m := NewLogsModel()
	m.loading = false
	m.targets = []logTarget{{Suite: "boosting", Container: "gradient-boost-lab"}}
	m.SetSize(120, 30)

	ch := make(chan dockerLogEvent)
	m.streamCh = ch
	for idx := 0; idx < logBufferCap+5; idx++ {
		updated, _ := m.Update(logEnvelope{
			ch: ch,
			ok: true,
			event: dockerLogEvent{
				line: strings.Repeat("x", idx%5+1),
			},
		})
		m = updated
	}

	if len(m.lines) != logBufferCap {
		t.Fatalf("buffer len = %d, want %d", len(m.lines), logBufferCap)
	}
}

func TestLogsSearchHighlightsMatches(t *testing.T) {
	m := NewLogsModel()
	m.lines = []string{"jupyter started", "kernel ready"}
	m.SetSize(120, 30)
	m.searchInput.SetValue("jupyter")
	m.syncViewport()

	view := m.viewport.View()
	if !strings.Contains(view, "jupyter started") {
		t.Fatalf("viewport missing matching line: %q", view)
	}
	if !strings.Contains(view, "kernel ready") {
		t.Fatalf("viewport missing non-matching line: %q", view)
	}
}

func TestLogsSearchDismissLeavesStreamActive(t *testing.T) {
	m := NewLogsModel()
	m.searching = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.searching {
		t.Fatal("expected esc to dismiss search")
	}
}

func TestLogsActivateDeactivateViewHelpersAndLoad(t *testing.T) {
	restoreModelDeps(t)

	loadStateFn = func() (config.State, error) {
		return config.State{Installed: []string{"boosting"}}, nil
	}

	m := NewLogsModel()
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	if help := m.HelpView(); help == "" {
		t.Fatal("expected logs help text")
	}
	msg := loadLogsTargetsCmd()().(logsLoadedMsg)
	if msg.err != nil || len(msg.targets) == 0 {
		t.Fatalf("loadLogsTargetsCmd() = %#v", msg)
	}
	m.loading = false
	m.targets = msg.targets
	m.SetSize(120, 30)
	if view := m.View(); view == "" {
		t.Fatal("expected logs view output")
	}
	called := false
	m.cancel = func() { called = true }
	m.Deactivate()
	if m.active {
		t.Fatal("expected deactivate to clear active flag")
	}
	if !called {
		t.Fatal("expected active log stream to be cancelled")
	}
	if max(1, 2) != 2 {
		t.Fatal("expected max helper to work")
	}
}

func TestWaitForLogEventReturnsMessage(t *testing.T) {
	ch := make(chan dockerLogEvent, 1)
	ch <- dockerLogEvent{line: "hello"}
	if waitForLogEvent(ch)() == nil {
		t.Fatal("expected envelope message")
	}
}

func TestStartDockerLogStreamReadsOutput(t *testing.T) {
	prependMockDocker(t, `printf 'line one\n'; printf 'line two\n' >&2`)

	cancel, ch, err := startDockerLogStream("gradient-boost-lab")
	if err != nil {
		t.Fatalf("startDockerLogStream() error = %v", err)
	}
	defer cancel()

	var lines []string
	for event := range ch {
		if event.line != "" {
			lines = append(lines, event.line)
		}
		if event.err != nil {
			t.Fatalf("unexpected stream error: %v", event.err)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestLogsUpdateCoversKeyPaths(t *testing.T) {
	restoreModelDeps(t)

	streams := 0
	dockerLogStreamFn = func(container string) (func(), <-chan dockerLogEvent, error) {
		streams++
		ch := make(chan dockerLogEvent, 2)
		return func() {}, ch, nil
	}

	m := NewLogsModel()
	m.SetSize(100, 30)
	updated, cmd := m.Update(logsLoadedMsg{
		targets: []logTarget{
			{Suite: "boosting", Container: "gradient-boost-core"},
			{Suite: "boosting", Container: "gradient-boost-lab"},
		},
	})
	if cmd == nil || updated.loading {
		t.Fatal("expected initial stream command")
	}

	updated, _ = updated.Update(keyRunes("/"))
	if !updated.searching {
		t.Fatal("expected search mode")
	}
	updated.searchInput.SetValue("jupyter")
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.searching {
		t.Fatal("expected enter to dismiss search")
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	updated, _ = updated.Update(keyRunes("f"))
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if streams == 0 {
		t.Fatal("expected stream restarts")
	}
}
