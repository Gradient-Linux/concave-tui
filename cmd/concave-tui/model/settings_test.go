package model

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
)

func TestSettingsRadioSelectionRendersAndMoves(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())

	view := m.View()
	if !strings.Contains(view, "● expanded") {
		t.Fatalf("expected expanded to be selected, got %q", view)
	}

	updated, _ := m.Update(keyRunes("j"))
	updated, _ = updated.Update(keyRunes("l"))
	if updated.sidebarRadio.Value() != "collapsed" {
		t.Fatalf("sidebar = %q, want collapsed", updated.sidebarRadio.Value())
	}
	if !strings.Contains(updated.View(), "● collapsed") {
		t.Fatalf("expected selected radio to move, got %q", updated.View())
	}
}

func TestSettingsNumericFieldsFocusIndependently(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())

	if updated := m; updated.focusedField != settingsFieldRefresh || !updated.insertMode {
		t.Fatalf("focus = %d insert=%v, want refresh input in insert mode", updated.focusedField, updated.insertMode)
	}

	updated, _ := m.Update(keyRunes("9"))
	if updated.refreshInput.Value() != "10009" {
		t.Fatalf("refreshInput = %q", updated.refreshInput.Value())
	}
	updated, _ = updated.Update(keyRunes("j"))
	if updated.focusedField != settingsFieldSidebar {
		t.Fatalf("focus = %d, want sidebar radio", updated.focusedField)
	}
	updated, _ = updated.Update(keyRunes("x"))
	if updated.refreshInput.Value() != "10009" {
		t.Fatalf("non-numeric input should be ignored, got %q", updated.refreshInput.Value())
	}
}

func TestSettingsSaveKeepsDisplayConfig(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())
	m.focusedField = settingsFieldSidebar
	m.insertMode = false

	updated, _ := m.Update(keyRunes("l"))
	if updated.current.Display.SidebarDefault != "collapsed" {
		t.Fatalf("sidebar default = %q", updated.current.Display.SidebarDefault)
	}

	updated, cmd := updated.Update(keyRunes("s"))
	if cmd == nil {
		t.Fatal("expected save command")
	}
	got, ok := cmd().(settingsSavedMsg)
	if !ok {
		t.Fatalf("save command = %#v", cmd())
	}
	if got.Config.Display.SidebarDefault != "collapsed" {
		t.Fatalf("saved sidebar default = %q", got.Config.Display.SidebarDefault)
	}
}

func TestSettingsEscLeavesInsertModeAndDiscards(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())
	updated := m
	if !updated.insertMode {
		t.Fatal("expected insert mode")
	}
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.insertMode {
		t.Fatal("expected esc to leave insert mode")
	}

	updated, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected discard command")
	}
	if _, ok := cmd().(settingsDiscardedMsg); !ok {
		t.Fatalf("discard command = %#v", cmd())
	}
}
