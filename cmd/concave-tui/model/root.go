package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiconfig "github.com/Gradient-Linux/concave-tui/cmd/concave-tui/config"
	cfgstore "github.com/Gradient-Linux/concave-tui/internal/config"
	"github.com/Gradient-Linux/concave-tui/internal/docker"
	"github.com/Gradient-Linux/concave-tui/internal/gpu"
	"github.com/Gradient-Linux/concave-tui/internal/suite"
	"github.com/Gradient-Linux/concave-tui/internal/system"
	"github.com/Gradient-Linux/concave-tui/internal/workspace"
)

const (
	ColorDeep    = "#401079"
	ColorMid     = "#7c3aed"
	ColorGold    = "#F9D44E"
	ColorSuccess = "#22c55e"
	ColorWarn    = "#f59e0b"
	ColorError   = "#ef4444"
	ColorMuted   = "#6b7280"
	ColorBg      = "#0f0f1a"
	minWidth     = 80
)

var (
	loadStateFn          = cfgstore.LoadState
	saveStateFn          = cfgstore.SaveState
	addSuiteFn           = cfgstore.AddSuite
	removeSuiteFn        = cfgstore.RemoveSuite
	isInstalledFn        = cfgstore.IsInstalled
	loadManifestFn       = cfgstore.LoadManifest
	saveManifestFn       = cfgstore.SaveManifest
	recordInstallFn      = cfgstore.RecordInstall
	recordUpdateFn       = cfgstore.RecordUpdate
	swapRollbackFn       = cfgstore.SwapForRollback
	saveTUIConfigFn      = tuiconfig.Save
	suiteGetFn           = suite.Get
	suiteAllFn           = suite.All
	suiteNamesFn         = suite.Names
	suitePrimaryFn       = suite.PrimaryContainer
	suiteJupyterFn       = suite.JupyterContainer
	suiteSelectionFn     = suite.SelectionFromContainerNames
	suiteBuildForgeFn    = suite.BuildForgeCompose
	suitePickForgeFn     = suite.PickComponents
	dockerPullFn         = docker.PullWithRollbackSafety
	dockerPullStreamFn   = docker.Pull
	dockerTagPreviousFn  = docker.TagAsPrevious
	dockerRevertPrevFn   = docker.RevertToPrevious
	dockerComposeUpFn    = docker.ComposeUp
	dockerComposeDownFn  = docker.ComposeDown
	dockerComposePathFn  = docker.ComposePath
	dockerWriteComposeFn = docker.WriteCompose
	dockerWriteRawFn     = docker.WriteRawCompose
	dockerStatusFn       = docker.ContainerStatus
	dockerOutputFn       = runDockerOutput
	dockerLogStreamFn    = startDockerLogStream
	systemRegisterFn     = system.Register
	systemDeregisterFn   = system.Deregister
	systemDockerFn       = system.DockerRunning
	systemGroupFn        = system.UserInDockerGroup
	systemInternetFn     = system.InternetReachable
	systemOpenURLFn      = system.OpenURL
	workspaceRootFn      = workspace.Root
	workspaceEnsureFn    = workspace.EnsureLayout
	workspaceStatusFn    = workspace.Status
	workspaceBackupFn    = workspace.Backup
	workspaceCleanFn     = workspace.CleanOutputs
	gpuDetectFn          = gpu.Detect
	gpuRecommendedFn     = gpu.RecommendedDriverBranch
	gpuToolkitFn         = gpu.ToolkitConfigured
)

var viewOrder = []string{"boosting", "neural", "flow", "forge"}

var errForgeSelectionMissing = errors.New("forge has no recorded component selection")

type View int

const (
	ViewWorkspace View = iota
	ViewSuites
	ViewLogs
	ViewDoctor
)

type SidebarState int

const (
	SidebarExpanded SidebarState = iota
	SidebarCollapsed
)

type rootConfigSavedMsg struct {
	err error
}

type RootModel struct {
	activeView   View
	sidebar      SidebarState
	suites       SuitesModel
	logs         LogsModel
	workspace    WorkspaceModel
	doctor       DoctorModel
	settings     SettingsModel
	width        int
	height       int
	showHelp     bool
	showSettings bool
	err          error
	version      string
	cfg          tuiconfig.Config
	lastKey      string
}

func init() {
	suite.SetConflictChecker(func(s suite.Suite, installed []string) ([]suite.PortConflict, error) {
		conflicts, err := system.CheckConflicts(s, installed)
		if err != nil {
			return nil, err
		}
		mapped := make([]suite.PortConflict, 0, len(conflicts))
		for _, conflict := range conflicts {
			mapped = append(mapped, suite.PortConflict{
				Port:          conflict.Port,
				ExistingSuite: conflict.ExistingSuite,
				NewSuite:      conflict.NewSuite,
				Service:       conflict.Service,
			})
		}
		return mapped, nil
	})
}

func NewRootModel(version string, cfgs ...tuiconfig.Config) *RootModel {
	cfg := tuiconfig.DefaultConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	m := &RootModel{
		activeView: ViewWorkspace,
		sidebar:    sidebarStateFromConfig(cfg),
		suites:     NewSuitesModel(),
		logs:       NewLogsModel(),
		workspace:  NewWorkspaceModel(),
		doctor:     NewDoctorModel(),
		settings:   NewSettingsModel(cfg),
		version:    version,
		cfg:        cfg,
	}
	m.applyConfig(cfg)
	return m
}

func (m *RootModel) Init() tea.Cmd {
	return m.activateView(m.activeView)
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil
	case rootConfigSavedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil
	case settingsSavedMsg:
		m.cfg = msg.Config
		m.showSettings = false
		m.settings.SetConfig(m.cfg)
		m.workspace.SetConfig(m.cfg)
		m.applyLayout()
		return m, saveTUIConfigCmd(m.cfg)
	case settingsDiscardedMsg:
		m.showSettings = false
		m.settings.SetConfig(m.cfg)
		m.workspace.SetConfig(m.cfg)
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if handled, cmd := m.handleGlobalKeys(keyMsg); handled {
			return m, cmd
		}
	}

	if m.showSettings {
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		m.workspace.SetConfig(m.settings.Current())
		return m, cmd
	}

	var cmd tea.Cmd
	switch m.activeView {
	case ViewWorkspace:
		m.workspace, cmd = m.workspace.Update(msg)
	case ViewSuites:
		m.suites, cmd = m.suites.Update(msg)
	case ViewLogs:
		m.logs, cmd = m.logs.Update(msg)
	case ViewDoctor:
		m.doctor, cmd = m.doctor.Update(msg)
	}
	return m, cmd
}

func (m *RootModel) View() string {
	if m.width > 0 && m.width < minWidth {
		return "Terminal too narrow — resize to at least 80 columns"
	}

	frame := m.renderFrame()
	if m.err != nil {
		frame += "\n" + errorText(m.err.Error())
	}
	if m.showSettings {
		return m.renderModal(frame, m.settings.View())
	}
	if m.showHelp {
		return m.renderModal(frame, m.helpOverlay())
	}
	return frame
}

func (m *RootModel) handleGlobalKeys(msg tea.KeyMsg) (bool, tea.Cmd) {
	if msg.String() != "g" {
		m.lastKey = ""
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return true, tea.Quit
	case "?", "f1":
		if m.showSettings {
			return false, nil
		}
		m.showHelp = !m.showHelp
		return true, nil
	case ",":
		if m.showHelp {
			m.showHelp = false
		}
		m.showSettings = true
		m.settings.SetConfig(m.cfg)
		m.workspace.SetConfig(m.settings.Current())
		m.applyLayout()
		return true, nil
	case "1":
		return true, m.switchView(ViewWorkspace)
	case "2":
		return true, m.switchView(ViewSuites)
	case "3":
		return true, m.switchView(ViewLogs)
	case "4":
		return true, m.switchView(ViewDoctor)
	case "tab":
		return true, m.switchView(nextVisibleView(m.activeView, 1))
	case "shift+tab":
		return true, m.switchView(nextVisibleView(m.activeView, -1))
	case "ctrl+b":
		if m.showSettings {
			return false, nil
		}
		m.pageUpFull()
		return true, nil
	case "b":
		if m.showSettings {
			return false, nil
		}
		if m.activeView != ViewSuites && m.activeView != ViewWorkspace {
			m.toggleSidebar()
			return true, nil
		}
	case "esc":
		if m.showHelp {
			m.showHelp = false
			return true, nil
		}
	case "g":
		if m.showSettings {
			return false, nil
		}
		if m.lastKey == "g" {
			m.lastKey = ""
			m.jumpTop()
			return true, nil
		}
		m.lastKey = "g"
		return true, nil
	case "G":
		if m.showSettings {
			return false, nil
		}
		m.jumpBottom()
		return true, nil
	case "ctrl+d":
		if m.showSettings {
			return false, nil
		}
		m.pageDownHalf()
		return true, nil
	case "ctrl+u":
		if m.showSettings {
			return false, nil
		}
		m.pageUpHalf()
		return true, nil
	case "ctrl+f":
		if m.showSettings {
			return false, nil
		}
		m.pageDownFull()
		return true, nil
	case "n":
		if m.showSettings {
			return false, nil
		}
		m.nextSearchMatch()
		return true, nil
	case "N":
		if m.showSettings {
			return false, nil
		}
		m.prevSearchMatch()
		return true, nil
	}
	return false, nil
}

func (m *RootModel) switchView(next View) tea.Cmd {
	if next == m.activeView {
		return nil
	}
	m.deactivateView(m.activeView)
	m.activeView = next
	return m.activateView(next)
}

func (m *RootModel) deactivateView(view View) {
	switch view {
	case ViewWorkspace:
		m.workspace.Deactivate()
	case ViewSuites:
		m.suites.Deactivate()
	case ViewLogs:
		m.logs.Deactivate()
	case ViewDoctor:
		m.doctor.Deactivate()
	}
}

func (m *RootModel) activateView(view View) tea.Cmd {
	switch view {
	case ViewWorkspace:
		return m.workspace.Activate()
	case ViewSuites:
		return m.suites.Activate()
	case ViewLogs:
		return m.logs.Activate()
	case ViewDoctor:
		return m.doctor.Activate()
	default:
		return nil
	}
}

func (m *RootModel) applyConfig(cfg tuiconfig.Config) {
	m.cfg = cfg
	m.sidebar = sidebarStateFromConfig(cfg)
	m.settings.SetConfig(cfg)
	m.workspace.SetConfig(cfg)
}

func (m *RootModel) applyLayout() {
	contentWidth := m.contentWidth()
	contentHeight := m.contentHeight()
	m.suites.SetSize(contentWidth, contentHeight)
	m.logs.SetSize(contentWidth, contentHeight)
	m.workspace.SetSize(contentWidth, contentHeight)
	m.doctor.SetSize(contentWidth, contentHeight)
	m.settings.SetSize(max(56, min(contentWidth, 78)), max(18, min(contentHeight, 22)))
}

func (m *RootModel) toggleSidebar() {
	if m.sidebar == SidebarExpanded {
		m.sidebar = SidebarCollapsed
	} else {
		m.sidebar = SidebarExpanded
	}
	m.applyLayout()
}

func (m *RootModel) sidebarWidth() int {
	if m.sidebar == SidebarCollapsed {
		return 4
	}
	return 22
}

func (m *RootModel) contentWidth() int {
	width := m.width - m.sidebarWidth() - 7
	if width < minWidth-10 {
		return minWidth - 10
	}
	return width
}

func (m *RootModel) contentHeight() int {
	height := m.height - 8
	if height < 16 {
		return 16
	}
	return height
}

func (m *RootModel) contentView() string {
	style := lipgloss.NewStyle().
		Width(m.contentWidth()).
		Height(m.contentHeight()).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(0, 1)
	return style.Render(padToHeight(m.activeContent(), m.contentHeight()))
}

func (m *RootModel) sidebarView() string {
	lines := make([]string, 0, 7)
	for _, view := range visibleViews() {
		active := view == m.activeView
		icon := sidebarIcon(view)
		label := sidebarLabel(view)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
		if active {
			style = style.Foreground(lipgloss.Color(ColorGold)).Bold(true).Background(lipgloss.Color(ColorDeep))
		}
		if m.sidebar == SidebarCollapsed {
			lines = append(lines, style.Width(2).Render(icon))
			continue
		}
		lines = append(lines, style.Width(m.sidebarWidth()-4).Render(" "+icon+"  "+label))
	}
	return lipgloss.NewStyle().
		Width(m.sidebarWidth()).
		Height(m.contentHeight()).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func (m *RootModel) activeContent() string {
	switch m.activeView {
	case ViewWorkspace:
		return m.workspace.View()
	case ViewSuites:
		return m.suites.View()
	case ViewLogs:
		return m.logs.View()
	case ViewDoctor:
		return m.doctor.View()
	default:
		return ""
	}
}

func (m *RootModel) helpOverlay() string {
	box := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGold)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(1, 2)
	return box.Render(m.activeHelp())
}

func (m *RootModel) activeHelp() string {
	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Keybindings"),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Navigation"),
		"j / k          move up / down",
		"h / l          move left / right",
		"g g            jump to top",
		"G              jump to bottom",
		"ctrl+d / u     scroll half page",
		"ctrl+f / b     scroll full page",
		"1-4            switch view",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Actions (" + sidebarLabel(m.activeView) + " view)"),
	}
	lines = append(lines, m.activeHelpActions()...)
	lines = append(lines,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("Global"),
		"b              toggle sidebar",
		",              settings",
		"q              quit",
		"",
		"esc / ? / F1   close this overlay",
	)
	return strings.Join(lines, "\n")
}

func (m *RootModel) activeHelpActions() []string {
	switch m.activeView {
	case ViewWorkspace:
		return []string{
			"b              backup notebooks + models",
			"x              clean outputs",
			"r              refresh workspace",
		}
	case ViewSuites:
		return []string{
			"i              install suite",
			"r              remove suite",
			"u              update suite",
			"R              rollback suite",
			"s / x          start / stop suite",
			"b              shell into suite",
			"e              exec command in suite",
			"l              open JupyterLab",
		}
	case ViewLogs:
		return []string{
			"/              search logs",
			"f              resume follow",
			"n / N          next / previous match",
		}
	case ViewDoctor:
		return []string{
			"r              rerun checks",
		}
	default:
		return nil
	}
}

func (m *RootModel) headerView() string {
	wordmark := gradientText("concave tui")
	version := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("concave-tui " + m.version)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(max(24, m.width-28)).Render(wordmark),
		version,
	)
}

func (m *RootModel) footerView() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Render(fmt.Sprintf("%s │ tab next · shift+tab prev · b sidebar · , settings · F1 help · q quit", m.currentMode()))
}

func (m *RootModel) currentMode() string {
	if m.showSettings && m.settings.IsInsertMode() {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("INSERT")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("NORMAL")
}

func (m *RootModel) renderFrame() string {
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.sidebarView(), m.contentView())
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Render(strings.Join([]string{
			m.headerView(),
			body,
			m.footerView(),
		}, "\n"))
}

func (m *RootModel) renderModal(background, panel string) string {
	width := max(minWidth, m.width)
	height := max(24, m.height)
	dimmed := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3a3a4a")).
		Render(stripANSI(background))
	placed := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#3a3a4a")),
	)
	return mergeOverlayRows(dimmed, placed, width, height)
}

func (m *RootModel) jumpTop() {
	switch m.activeView {
	case ViewSuites:
		m.suites.selected = 0
	case ViewLogs:
		m.logs.follow = false
		m.logs.viewport.GotoTop()
	}
}

func (m *RootModel) jumpBottom() {
	switch m.activeView {
	case ViewSuites:
		if len(m.suites.rows) > 0 {
			m.suites.selected = len(m.suites.rows) - 1
		}
	case ViewLogs:
		m.logs.follow = true
		m.logs.syncViewport()
	}
}

func (m *RootModel) pageDownHalf() {
	if m.activeView == ViewLogs {
		m.logs.follow = false
		m.logs.viewport.HalfViewDown()
	}
}

func (m *RootModel) pageUpHalf() {
	if m.activeView == ViewLogs {
		m.logs.follow = false
		m.logs.viewport.HalfViewUp()
	}
}

func (m *RootModel) pageDownFull() {
	if m.activeView == ViewLogs {
		m.logs.follow = false
		m.logs.viewport.ViewDown()
	}
}

func (m *RootModel) pageUpFull() {
	if m.activeView == ViewLogs {
		m.logs.follow = false
		m.logs.viewport.ViewUp()
	}
}

func (m *RootModel) nextSearchMatch() {
	if m.activeView == ViewLogs {
		m.logs.jumpToMatch(true)
	}
}

func (m *RootModel) prevSearchMatch() {
	if m.activeView == ViewLogs {
		m.logs.jumpToMatch(false)
	}
}

func saveTUIConfigCmd(cfg tuiconfig.Config) tea.Cmd {
	return func() tea.Msg {
		return rootConfigSavedMsg{err: saveTUIConfigFn(cfg)}
	}
}

func sidebarStateFromConfig(cfg tuiconfig.Config) SidebarState {
	if strings.EqualFold(cfg.Display.SidebarDefault, "collapsed") {
		return SidebarCollapsed
	}
	return SidebarExpanded
}

func visibleViews() []View {
	return []View{ViewWorkspace, ViewSuites, ViewLogs, ViewDoctor}
}

func nextPresetName(cfg tuiconfig.Config) string {
	names := cfg.PresetNames()
	if len(names) == 0 {
		return "default"
	}
	current := cfg.ActivePreset
	for idx, name := range names {
		if name == current {
			return names[(idx+1)%len(names)]
		}
	}
	return names[0]
}

func nextVisibleView(current View, delta int) View {
	views := visibleViews()
	if len(views) == 0 {
		return current
	}
	index := 0
	for idx, view := range views {
		if view == current {
			index = idx
			break
		}
	}
	next := (index + delta) % len(views)
	if next < 0 {
		next += len(views)
	}
	return views[next]
}

func sidebarIcon(view View) string {
	switch view {
	case ViewWorkspace:
		return "󰉋"
	case ViewSuites:
		return "󰣘"
	case ViewLogs:
		return "󰈙"
	case ViewDoctor:
		return "󰓙"
	default:
		return "•"
	}
}

func sidebarLabel(view View) string {
	switch view {
	case ViewWorkspace:
		return "Workspace"
	case ViewSuites:
		return "Suites"
	case ViewLogs:
		return "Logs"
	case ViewDoctor:
		return "Doctor"
	default:
		return ""
	}
}

func gradientText(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(runes))
	for idx, r := range runes {
		color := interpolateHex(ColorDeep, ColorGold, ratio(idx, len(runes)))
		parts = append(parts, lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Bold(true).
			Render(string(r)))
	}
	return strings.Join(parts, "")
}

func gradientBar(width int, fillRatio float64, styleThreshold bool) string {
	if width <= 0 {
		return ""
	}
	if fillRatio < 0 {
		fillRatio = 0
	}
	if fillRatio > 1 {
		fillRatio = 1
	}
	filled := int(math.Round(fillRatio * float64(width)))
	if filled > width {
		filled = width
	}
	var parts []string
	for idx := 0; idx < filled; idx++ {
		color := interpolateHex(ColorDeep, ColorGold, ratio(idx, max(1, filled)))
		if styleThreshold {
			switch {
			case fillRatio >= 0.95:
				color = ColorError
			case fillRatio >= 0.80:
				color = ColorWarn
			}
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("█"))
	}
	if width-filled > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(strings.Repeat("░", width-filled)))
	}
	return strings.Join(parts, "")
}

func utilizationBar(width int, fillRatio float64, styleThreshold bool) string {
	if width <= 0 {
		return ""
	}
	if fillRatio < 0 {
		fillRatio = 0
	}
	if fillRatio > 1 {
		fillRatio = 1
	}
	filled := int(math.Round(fillRatio * float64(width)))
	if filled > width {
		filled = width
	}

	color := interpolateHex(ColorDeep, ColorGold, fillRatio)
	if styleThreshold {
		switch {
		case fillRatio >= 0.95:
			color = ColorError
		case fillRatio >= 0.80:
			color = ColorWarn
		}
	}

	parts := make([]string, 0, 2)
	if filled > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(strings.Repeat("█", filled)))
	}
	if width-filled > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(strings.Repeat("░", width-filled)))
	}
	return strings.Join(parts, "")
}

func interpolateHex(startHex, endHex string, amount float64) string {
	sr, sg, sb := hexColor(startHex)
	er, eg, eb := hexColor(endHex)
	return fmt.Sprintf(
		"#%02x%02x%02x",
		int(clamp(float64(sr)+(float64(er-sr)*amount), 0, 255)),
		int(clamp(float64(sg)+(float64(eg-sg)*amount), 0, 255)),
		int(clamp(float64(sb)+(float64(eb-sb)*amount), 0, 255)),
	)
}

func hexColor(value string) (int, int, int) {
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseInt(value[0:2], 16, 64)
	g, _ := strconv.ParseInt(value[2:4], 16, 64)
	b, _ := strconv.ParseInt(value[4:6], 16, 64)
	return int(r), int(g), int(b)
}

func ratio(idx, count int) float64 {
	if count <= 1 {
		return 0
	}
	return float64(idx) / float64(count-1)
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func orderedInstalledSuites(installed []string) []string {
	set := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		set[name] = struct{}{}
	}
	ordered := make([]string, 0, len(installed))
	for _, name := range viewOrder {
		if _, ok := set[name]; ok {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

func currentSuiteDefinition(name string) (suite.Suite, error) {
	s, err := suiteGetFn(name)
	if err != nil {
		return suite.Suite{}, err
	}
	if name != "forge" {
		return s, nil
	}

	manifest, err := loadManifestFn()
	if err != nil {
		return suite.Suite{}, err
	}
	containers, ok := manifest[name]
	if !ok || len(containers) == 0 {
		return suite.Suite{}, errForgeSelectionMissing
	}

	names := make([]string, 0, len(containers))
	overrides := make(map[string]string, len(containers))
	for containerName, version := range containers {
		names = append(names, containerName)
		overrides[containerName] = version.Current
	}
	sort.Strings(names)

	selection, err := suiteSelectionFn(names, overrides)
	if err != nil {
		return suite.Suite{}, err
	}
	s.Containers = selection.Containers
	s.Ports = selection.Ports
	s.Volumes = selection.Volumes
	return s, nil
}

func isMissingForgeSelection(err error) bool {
	return errors.Is(err, errForgeSelectionMissing)
}

func writeComposeForSuite(name string) (string, error) {
	if name != "forge" {
		return dockerWriteComposeFn(name)
	}

	s, err := currentSuiteDefinition(name)
	if err != nil {
		return "", err
	}
	data, err := suiteBuildForgeFn(suite.ForgeSelection{
		Containers: s.Containers,
		Ports:      s.Ports,
		Volumes:    s.Volumes,
	})
	if err != nil {
		return "", err
	}
	return dockerWriteRawFn(name, data)
}

func currentImageForFirstContainer(name string, manifest cfgstore.VersionManifest) string {
	s, err := suiteGetFn(name)
	if err != nil || len(s.Containers) == 0 {
		return ""
	}
	if containers, ok := manifest[name]; ok {
		if version, ok := containers[s.Containers[0].Name]; ok && version.Current != "" {
			return version.Current
		}
	}
	return s.Containers[0].Image
}

func runDockerOutput(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
}

func openLabURL(name string) (string, error) {
	s, err := currentSuiteDefinition(name)
	if err != nil {
		if isMissingForgeSelection(err) && name == "forge" {
			return "", fmt.Errorf("forge is marked installed but has no recorded component selection; run concave remove forge, then reinstall it")
		}
		return "", err
	}
	container, ok := suiteJupyterFn(s)
	if !ok {
		return "", fmt.Errorf("suite %s has no JupyterLab service", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	status, err := dockerStatusFn(ctx, container)
	if err != nil {
		return "", err
	}
	if status != "running" {
		return "", fmt.Errorf("JupyterLab is not running. Start it with: concave start %s", name)
	}

	out, err := dockerOutputFn(ctx, "exec", container, "jupyter", "server", "list", "--json")
	if err != nil {
		return "", fmt.Errorf("resolve Jupyter token for %s: %w", container, err)
	}
	return extractLabURL(string(out))
}

func extractLabURL(raw string) (string, error) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var server struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(line), &server); err == nil && server.Token != "" {
			return "http://localhost:8888/lab?token=" + server.Token, nil
		}
	}
	re := regexp.MustCompile(`https?://[^\s]+/\??[^\s]*token=[A-Za-z0-9]+`)
	match := re.FindString(raw)
	if match == "" {
		return "", fmt.Errorf("unable to find tokenized Jupyter URL")
	}
	match = strings.Replace(match, "0.0.0.0", "127.0.0.1", 1)
	match = strings.Replace(match, "localhost", "127.0.0.1", 1)
	match = strings.Replace(match, "/?token=", "/lab?token=", 1)
	return match, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func padToHeight(content string, height int) string {
	if height <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func stripANSI(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}

func mergeOverlayRows(background, overlay string, _ int, height int) string {
	bgLines := strings.Split(padToHeight(background, height), "\n")
	ovLines := strings.Split(padToHeight(overlay, height), "\n")
	merged := make([]string, 0, height)
	for idx := 0; idx < height && idx < len(bgLines) && idx < len(ovLines); idx++ {
		if strings.TrimSpace(stripANSI(ovLines[idx])) == "" {
			merged = append(merged, bgLines[idx])
			continue
		}
		merged = append(merged, ovLines[idx])
	}
	return strings.Join(merged, "\n")
}
