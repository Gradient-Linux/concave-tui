package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
)

type settingsSavedMsg struct {
	Config tuiconfig.Config
}

type settingsDiscardedMsg struct{}

type SettingsModel struct {
	width      int
	height     int
	cursor     int
	editing    bool
	input      textinput.Model
	current    tuiconfig.Config
	original   tuiconfig.Config
	lastSaved  string
	fieldOrder []string
}

func NewSettingsModel(cfg tuiconfig.Config) SettingsModel {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 6
	input.Width = 8
	input.Validate = func(value string) error {
		if value == "" {
			return nil
		}
		for _, r := range value {
			if r < '0' || r > '9' {
				return fmt.Errorf("numeric only")
			}
		}
		return nil
	}
	return SettingsModel{
		current:    cfg,
		original:   cfg,
		input:      input,
		fieldOrder: []string{"graph_style", "width", "height", "refresh", "sidebar", "preset"},
	}
}

func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) SetConfig(cfg tuiconfig.Config) {
	m.current = cfg
	m.original = cfg
	m.editing = false
	m.input.Blur()
	m.input.SetValue("")
}

func (m SettingsModel) Current() tuiconfig.Config {
	return m.current
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.editing {
			return m.updateEditing(keyMsg)
		}
		return m.updateNavigation(keyMsg)
	}
	return m, nil
}

func (m SettingsModel) updateEditing(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.input.Blur()
		m.input.SetValue("")
		return m, nil
	case "enter":
		if m.applyInput() {
			m.editing = false
			m.input.Blur()
			m.input.SetValue("")
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m SettingsModel) updateNavigation(msg tea.KeyMsg) (SettingsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.current = m.original
		return m, func() tea.Msg { return settingsDiscardedMsg{} }
	case "s":
		m.original = m.current
		return m, func() tea.Msg { return settingsSavedMsg{Config: m.current} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.fieldOrder)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = len(m.fieldOrder) - 1
	case "left", "h":
		m.adjustCurrent(-1)
	case "right":
		m.adjustCurrent(1)
	case "l":
		if m.fieldOrder[m.cursor] == "preset" {
			m.adjustCurrent(1)
			break
		}
		if m.fieldOrder[m.cursor] == "graph_style" || m.fieldOrder[m.cursor] == "sidebar" {
			m.adjustCurrent(1)
		}
	case "enter":
		if m.fieldOrder[m.cursor] == "width" || m.fieldOrder[m.cursor] == "height" || m.fieldOrder[m.cursor] == "refresh" {
			m.editing = true
			m.input.Focus()
			m.input.SetValue(m.numericValue())
		}
	}
	return m, nil
}

func (m *SettingsModel) adjustCurrent(step int) {
	switch m.fieldOrder[m.cursor] {
	case "graph_style":
		options := []string{"line", "bar", "auto"}
		m.current.Display.GraphStyle = cycleString(options, m.current.Display.GraphStyle, step)
	case "sidebar":
		options := []string{"expanded", "collapsed"}
		m.current.Display.SidebarDefault = cycleString(options, m.current.Display.SidebarDefault, step)
	case "preset":
		names := m.current.PresetNames()
		if len(names) == 0 {
			return
		}
		m.current.ActivePreset = cycleString(names, m.current.ActivePreset, step)
	}
}

func (m SettingsModel) numericValue() string {
	switch m.fieldOrder[m.cursor] {
	case "width":
		return strconv.Itoa(m.current.Display.GraphAutoWidthThreshold)
	case "height":
		return strconv.Itoa(m.current.Display.GraphAutoHeightThreshold)
	case "refresh":
		return strconv.Itoa(m.current.Display.RefreshIntervalMs)
	default:
		return ""
	}
}

func (m *SettingsModel) applyInput() bool {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return false
	}
	num, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	switch m.fieldOrder[m.cursor] {
	case "width":
		m.current.Display.GraphAutoWidthThreshold = num
	case "height":
		m.current.Display.GraphAutoHeightThreshold = num
	case "refresh":
		m.current.Display.RefreshIntervalMs = num
	default:
		return false
	}
	return true
}

func (m SettingsModel) View() string {
	style := lipgloss.NewStyle().
		Width(max(56, m.width)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorGold)).
		Padding(0, 1)

	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Settings"),
		"",
		"Display",
		m.renderRow(0, "Graph style", fmt.Sprintf("○ line  ○ bar  ○ auto   [%s]", m.current.Display.GraphStyle)),
		m.renderRow(1, "Width threshold", m.renderNumericOrValue(strconv.Itoa(m.current.Display.GraphAutoWidthThreshold)+" cols")),
		m.renderRow(2, "Height threshold", m.renderNumericOrValue(strconv.Itoa(m.current.Display.GraphAutoHeightThreshold)+" rows")),
		m.renderRow(3, "Refresh interval", m.renderNumericOrValue(strconv.Itoa(m.current.Display.RefreshIntervalMs)+" ms")),
		m.renderRow(4, "Sidebar default", fmt.Sprintf("○ expanded  ○ collapsed   [%s]", m.current.Display.SidebarDefault)),
		"",
		"Dashboard Preset",
	}
	for _, preset := range m.current.Presets {
		marker := "○"
		if preset.Name == m.current.ActivePreset {
			marker = "●"
		}
		lines = append(lines, fmt.Sprintf("  %s %-10s %s", marker, preset.Name, preset.Description))
	}
	lines = append(lines, "", "[s] save and close        [esc] discard changes")
	return style.Render(strings.Join(lines, "\n"))
}

func (m SettingsModel) renderRow(index int, label, value string) string {
	row := fmt.Sprintf("  %-18s %s", label, value)
	if index == m.cursor {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGold)).
			Background(lipgloss.Color(ColorDeep)).
			Render(row)
	}
	return row
}

func (m SettingsModel) renderNumericOrValue(value string) string {
	if m.editing {
		switch m.fieldOrder[m.cursor] {
		case "width", "height", "refresh":
			return m.input.View()
		}
	}
	return value
}

func cycleString(values []string, current string, step int) string {
	if len(values) == 0 {
		return current
	}
	index := 0
	for idx, value := range values {
		if value == current {
			index = idx
			break
		}
	}
	index = (index + step + len(values)) % len(values)
	return values[index]
}
