package model

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
)

func TestSettingsDiscardAndSave(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())
	m.cursor = 5
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if updated.current.ActivePreset == m.current.ActivePreset {
		t.Fatal("expected preset preview change")
	}

	updated, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected save message command")
	}
	if _, ok := cmd().(settingsSavedMsg); !ok {
		t.Fatal("expected settingsSavedMsg")
	}

	updated.SetConfig(tuiconfig.DefaultConfig())
	updated.cursor = 5
	updated.current.ActivePreset = "training"
	_, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected discard message command")
	}
	if _, ok := cmd().(settingsDiscardedMsg); !ok {
		t.Fatal("expected settingsDiscardedMsg")
	}
}

func TestSettingsNumericEditingIgnoresNonNumeric(t *testing.T) {
	m := NewSettingsModel(tuiconfig.DefaultConfig())
	m.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.editing {
		t.Fatal("expected numeric edit mode")
	}
	updated, _ = updated.Update(keyRunes("x"))
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.current.Display.GraphAutoWidthThreshold != tuiconfig.DefaultConfig().Display.GraphAutoWidthThreshold {
		t.Fatal("expected non-numeric input to be ignored")
	}
}
