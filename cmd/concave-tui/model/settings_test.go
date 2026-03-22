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
	if !strings.Contains(view, "● auto") {
		t.Fatalf("expected auto to be selected, got %q", view)
	}
	if strings.Contains(view, "Dashboard Preset") || strings.Contains(view, "Balance view") {
		t.Fatalf("unexpected dashboard preset controls in view, got %q", view)
	}
	if strings.Contains(view, "Width threshold    ╭") || strings.Contains(view, "Height threshold   ╭") || strings.Contains(view, "Refresh interval   ╭") {
		t.Fatalf("unexpected boxed numeric row in view, got %q", view)
	}

	updated, _ := m.Update(keyRunes("l"))
	if updated.graphStyle.Value() != "line" {
		t.Fatalf("graph style = %q, want line", updated.graphStyle.Value())
	}
	if !strings.Contains(updated.View(), "● line") {
		t.Fatalf("expected selected radio to move, got %q", updated.View())
	}
}

func TestSettingsNumericFieldsFocusIndependently(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.focusedField != settingsFieldWidth || !updated.insertMode {
		t.Fatalf("focus = %d insert=%v, want width input in insert mode", updated.focusedField, updated.insertMode)
	}

	updated, _ = updated.Update(keyRunes("9"))
	if updated.widthInput.Value() != "1209" {
		t.Fatalf("widthInput = %q", updated.widthInput.Value())
	}
	if updated.heightInput.Value() != "40" || updated.refreshInput.Value() != "1000" {
		t.Fatalf("other inputs changed: height=%q refresh=%q", updated.heightInput.Value(), updated.refreshInput.Value())
	}

	updated, _ = updated.Update(keyRunes("j"))
	if updated.focusedField != settingsFieldHeight {
		t.Fatalf("focus = %d, want height input", updated.focusedField)
	}
	updated, _ = updated.Update(keyRunes("7"))
	if updated.heightInput.Value() != "407" {
		t.Fatalf("heightInput = %q", updated.heightInput.Value())
	}

	updated, _ = updated.Update(keyRunes("x"))
	if updated.heightInput.Value() != "407" {
		t.Fatalf("non-numeric input should be ignored, got %q", updated.heightInput.Value())
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
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
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
