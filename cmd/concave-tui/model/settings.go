package model

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
)

const (
	settingsFieldRefresh = iota
	settingsFieldSidebar
	settingsFieldCount
)

type settingsSavedMsg struct {
	Config tuiconfig.Config
}

type settingsDiscardedMsg struct{}

type RadioField struct {
	Label    string
	Options  []string
	Selected int
}

func (f *RadioField) SetValue(value string) {
	for idx, option := range f.Options {
		if strings.EqualFold(option, value) {
			f.Selected = idx
			return
		}
	}
	f.Selected = 0
}

func (f RadioField) Value() string {
	if len(f.Options) == 0 {
		return ""
	}
	if f.Selected < 0 || f.Selected >= len(f.Options) {
		return f.Options[0]
	}
	return f.Options[f.Selected]
}

func (f *RadioField) Move(delta int) {
	if len(f.Options) == 0 {
		return
	}
	next := (f.Selected + delta) % len(f.Options)
	if next < 0 {
		next += len(f.Options)
	}
	f.Selected = next
}

func (f RadioField) View(focused bool) string {
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(f.Label)
	if focused {
		label = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render(f.Label)
	}

	parts := make([]string, 0, len(f.Options))
	for idx, opt := range f.Options {
		if idx == f.Selected {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("● "+opt))
			continue
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("○ "+opt))
	}
	return label + "   " + strings.Join(parts, "  ")
}

type SettingsModel struct {
	width        int
	height       int
	current      tuiconfig.Config
	original     tuiconfig.Config
	sidebarRadio RadioField
	refreshInput textinput.Model
	focusedField int
	insertMode   bool
}

func NewSettingsModel(cfg tuiconfig.Config) SettingsModel {
	m := SettingsModel{
		sidebarRadio: RadioField{
			Label:   "Sidebar default",
			Options: []string{"expanded", "collapsed"},
		},
		refreshInput: newNumericInput(),
	}
	m.SetConfig(cfg)
	return m
}

func newNumericInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 6
	input.Width = 8
	input.Validate = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		_, err := strconv.Atoi(value)
		return err
	}
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold))
	input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold))
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	return input
}

func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) SetConfig(cfg tuiconfig.Config) {
	m.current = cfg
	m.original = cfg
	m.sidebarRadio.SetValue(cfg.Display.SidebarDefault)
	m.refreshInput.SetValue(strconv.Itoa(cfg.Display.RefreshIntervalMs))
	m.focusedField = settingsFieldRefresh
	m.insertMode = true
	m.applyFocus()
}

func (m SettingsModel) Current() tuiconfig.Config {
	return m.current
}

func (m SettingsModel) IsInsertMode() bool {
	return m.insertMode
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		if m.insertMode {
			switch typed.String() {
			case "esc":
				m.insertMode = false
				m.blurInputs()
				return m, nil
			case "tab", "j", "down":
				return m.moveFocus(1)
			case "shift+tab", "k", "up":
				return m.moveFocus(-1)
			}
			if typed.Type == tea.KeyRunes {
				for _, r := range typed.Runes {
					if r < '0' || r > '9' {
						return m, nil
					}
				}
			}
			return m.updateFocusedInput(msg)
		}

		switch typed.String() {
		case "esc":
			return m, func() tea.Msg { return settingsDiscardedMsg{} }
		case "s":
			m.syncCurrent()
			saved := m.current
			return m, func() tea.Msg { return settingsSavedMsg{Config: saved} }
		case "tab", "j", "down":
			return m.moveFocus(1)
		case "shift+tab", "k", "up":
			return m.moveFocus(-1)
		case "h", "left":
			m.shiftFocusedSelection(-1)
			return m, nil
		case "l", "right":
			m.shiftFocusedSelection(1)
			return m, nil
		case "enter":
			if m.isNumericField(m.focusedField) {
				m.insertMode = true
				return m, m.applyFocus()
			}
		}
	}
	return m, nil
}

func (m *SettingsModel) syncCurrent() {
	m.current.Display.SidebarDefault = m.sidebarRadio.Value()
	m.current.Display.RefreshIntervalMs = m.numericValue(m.refreshInput.Value(), m.current.Display.RefreshIntervalMs)
}

func (m SettingsModel) numericValue(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func (m SettingsModel) moveFocus(delta int) (SettingsModel, tea.Cmd) {
	m.focusedField = (m.focusedField + delta + settingsFieldCount) % settingsFieldCount
	m.insertMode = m.isNumericField(m.focusedField)
	return m, m.applyFocus()
}

func (m *SettingsModel) blurInputs() {
	m.refreshInput.Blur()
}

func (m *SettingsModel) applyFocus() tea.Cmd {
	m.blurInputs()
	if m.focusedField == settingsFieldRefresh {
		return m.refreshInput.Focus()
	}
	return nil
}

func (m SettingsModel) isNumericField(index int) bool {
	return index == settingsFieldRefresh
}

func (m SettingsModel) updateFocusedInput(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd
	if m.focusedField == settingsFieldRefresh {
		m.refreshInput, cmd = m.refreshInput.Update(msg)
	}
	m.syncCurrent()
	return m, cmd
}

func (m *SettingsModel) shiftFocusedSelection(delta int) {
	if m.focusedField == settingsFieldSidebar {
		m.sidebarRadio.Move(delta)
	}
	m.syncCurrent()
}

func (m SettingsModel) View() string {
	width := m.width
	if width <= 0 {
		width = 72
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Settings"),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Display"),
		m.renderNumericRow("Refresh interval", m.refreshInput, m.focusedField == settingsFieldRefresh),
		m.sidebarRadio.View(m.focusedField == settingsFieldSidebar),
		"",
		mutedText("[s] save and close        [esc] discard changes"),
	}

	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(min(width, 78)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(1, 2).
		Render(body)
}

func (m SettingsModel) renderNumericRow(label string, input textinput.Model, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
	}
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	if focused {
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold))
	}
	return labelStyle.Render(label) + "   " + valueStyle.Render(input.View()) + mutedText(" ms")
}
