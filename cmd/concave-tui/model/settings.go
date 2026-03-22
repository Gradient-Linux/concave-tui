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
	settingsFieldGraphStyle = iota
	settingsFieldWidth
	settingsFieldHeight
	settingsFieldRefresh
	settingsFieldSidebar
	settingsFieldPreset
	settingsFieldCount
)

type settingsSavedMsg struct {
	Config tuiconfig.Config
}

type settingsDiscardedMsg struct{}

type PresetChangedMsg struct {
	PresetName string
}

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
	graphStyle   RadioField
	sidebarRadio RadioField
	presetRadio  RadioField
	widthInput   textinput.Model
	heightInput  textinput.Model
	refreshInput textinput.Model
	focusedField int
	insertMode   bool
}

func NewSettingsModel(cfg tuiconfig.Config) SettingsModel {
	m := SettingsModel{
		graphStyle: RadioField{
			Label:   "Graph style",
			Options: []string{"line", "bar", "auto"},
		},
		sidebarRadio: RadioField{
			Label:   "Sidebar default",
			Options: []string{"expanded", "collapsed"},
		},
		widthInput:   newNumericInput(),
		heightInput:  newNumericInput(),
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
	m.graphStyle.SetValue(cfg.Display.GraphStyle)
	m.sidebarRadio.SetValue(cfg.Display.SidebarDefault)
	m.presetRadio = RadioField{
		Label:   "Dashboard preset",
		Options: cfg.PresetNames(),
	}
	if len(m.presetRadio.Options) == 0 {
		m.presetRadio.Options = []string{"default"}
	}
	m.presetRadio.SetValue(cfg.ActivePreset)
	m.widthInput.SetValue(strconv.Itoa(cfg.Display.GraphAutoWidthThreshold))
	m.heightInput.SetValue(strconv.Itoa(cfg.Display.GraphAutoHeightThreshold))
	m.refreshInput.SetValue(strconv.Itoa(cfg.Display.RefreshIntervalMs))
	m.focusedField = settingsFieldGraphStyle
	m.insertMode = false
	m.blurInputs()
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
			return m, tea.Batch(
				func() tea.Msg { return settingsSavedMsg{Config: saved} },
				func() tea.Msg { return PresetChangedMsg{PresetName: saved.ActivePreset} },
			)
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
	m.current.Display.GraphStyle = m.graphStyle.Value()
	m.current.Display.SidebarDefault = m.sidebarRadio.Value()
	m.current.ActivePreset = m.presetRadio.Value()
	m.current.Display.GraphAutoWidthThreshold = m.numericValue(m.widthInput.Value(), m.current.Display.GraphAutoWidthThreshold)
	m.current.Display.GraphAutoHeightThreshold = m.numericValue(m.heightInput.Value(), m.current.Display.GraphAutoHeightThreshold)
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
	m.widthInput.Blur()
	m.heightInput.Blur()
	m.refreshInput.Blur()
}

func (m *SettingsModel) applyFocus() tea.Cmd {
	m.blurInputs()
	switch m.focusedField {
	case settingsFieldWidth:
		return m.widthInput.Focus()
	case settingsFieldHeight:
		return m.heightInput.Focus()
	case settingsFieldRefresh:
		return m.refreshInput.Focus()
	default:
		return nil
	}
}

func (m SettingsModel) isNumericField(index int) bool {
	switch index {
	case settingsFieldWidth, settingsFieldHeight, settingsFieldRefresh:
		return true
	default:
		return false
	}
}

func (m SettingsModel) updateFocusedInput(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focusedField {
	case settingsFieldWidth:
		m.widthInput, cmd = m.widthInput.Update(msg)
	case settingsFieldHeight:
		m.heightInput, cmd = m.heightInput.Update(msg)
	case settingsFieldRefresh:
		m.refreshInput, cmd = m.refreshInput.Update(msg)
	}
	m.syncCurrent()
	return m, cmd
}

func (m *SettingsModel) shiftFocusedSelection(delta int) {
	switch m.focusedField {
	case settingsFieldGraphStyle:
		m.graphStyle.Move(delta)
	case settingsFieldSidebar:
		m.sidebarRadio.Move(delta)
	case settingsFieldPreset:
		m.presetRadio.Move(delta)
	default:
		return
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
		m.graphStyle.View(m.focusedField == settingsFieldGraphStyle),
		m.renderNumericRow("Width threshold", m.widthInput, m.focusedField == settingsFieldWidth),
		m.renderNumericRow("Height threshold", m.heightInput, m.focusedField == settingsFieldHeight),
		m.renderNumericRow("Refresh interval", m.refreshInput, m.focusedField == settingsFieldRefresh),
		m.sidebarRadio.View(m.focusedField == settingsFieldSidebar),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Dashboard Preset"),
		m.renderPresetRow(),
	}

	lines = append(lines,
		"",
		mutedText("[s] save and close        [esc] discard changes"),
	)

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

	value := input.View()
	if !focused {
		value = mutedText(input.Value())
	} else {
		value = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(value)
	}
	return labelStyle.Width(18).Render(label) + " " + value
}

func (m SettingsModel) presetLabel(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "default":
		return "Balance view"
	case "training":
		return "Training view"
	case "mlops":
		return "MLOps view"
	case "inference":
		return "Inference view"
	default:
		return strings.TrimSpace(name)
	}
}

func (m SettingsModel) renderPresetRow() string {
	parts := make([]string, 0, len(m.presetRadio.Options))
	for idx, name := range m.presetRadio.Options {
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
		marker := mutedText("○")
		if idx == m.presetRadio.Selected {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("●")
		}
		if m.focusedField == settingsFieldPreset && idx == m.presetRadio.Selected {
			labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
		}
		parts = append(parts, marker+" "+labelStyle.Render(m.presetLabel(name)))
	}

	row := strings.Join(parts, "  ")
	if m.width <= 0 {
		return row
	}
	return lipgloss.NewStyle().Width(min(m.width-4, 72)).Render(row)
}
