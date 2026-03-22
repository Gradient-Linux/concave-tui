package model

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

func TestLogsContainerSwitchCancelsPreviousStream(t *testing.T) {
	restoreModelDeps(t)

	cancelCount := 0
	apiLogsDialFn = func(ctx context.Context, suiteName, container string) (logStream, error) {
		ch := make(chan dockerLogEvent)
		return logStream{
			cancel: func() { cancelCount++ },
			ch:     ch,
		}, nil
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

func TestLogsActivateLoadAndView(t *testing.T) {
	restoreModelDeps(t)

	apiSuitesFn = func(ctx context.Context) ([]apiclient.SuiteSummary, error) {
		return []apiclient.SuiteSummary{
			{
				Name:      "boosting",
				Installed: true,
				State:     "running",
				Containers: []apiclient.ContainerInfo{
					{Name: "gradient-boost-core"},
				},
			},
		}, nil
	}
	apiLogsDialFn = func(ctx context.Context, suiteName, container string) (logStream, error) {
		ch := make(chan dockerLogEvent)
		return logStream{cancel: func() {}, ch: ch}, nil
	}

	m := NewLogsModel()
	if cmd := m.Activate(); cmd == nil {
		t.Fatal("expected activate cmd")
	}
	msg := loadLogsTargetsCmd()().(logsLoadedMsg)
	if msg.err != nil || len(msg.targets) != 1 {
		t.Fatalf("loadLogsTargetsCmd() = %#v", msg)
	}
	m.loading = false
	m.targets = msg.targets
	m.SetSize(120, 30)
	if view := m.View(); view == "" {
		t.Fatal("expected logs view output")
	}
}

func TestLogsUpdateCoversKeyPaths(t *testing.T) {
	restoreModelDeps(t)

	streams := 0
	apiLogsDialFn = func(ctx context.Context, suiteName, container string) (logStream, error) {
		streams++
		ch := make(chan dockerLogEvent, 2)
		return logStream{cancel: func() {}, ch: ch}, nil
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

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if streams == 0 {
		t.Fatal("expected stream restarts")
	}
}
