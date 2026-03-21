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
	ViewDashboard View = iota
	ViewSuites
	ViewLogs
	ViewWorkspace
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
	dashboard    DashboardModel
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
		activeView: ViewDashboard,
		sidebar:    sidebarStateFromConfig(cfg),
		dashboard:  NewDashboardModel(),
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
		m.dashboard.SetConfig(m.cfg)
		m.applyLayout()
		return m, saveTUIConfigCmd(m.cfg)
	case settingsDiscardedMsg:
		m.showSettings = false
		m.settings.SetConfig(m.cfg)
		m.dashboard.SetConfig(m.cfg)
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
		m.dashboard.SetConfig(m.settings.Current())
		return m, cmd
	}

	var cmd tea.Cmd
	switch m.activeView {
	case ViewDashboard:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case ViewSuites:
		m.suites, cmd = m.suites.Update(msg)
	case ViewLogs:
		m.logs, cmd = m.logs.Update(msg)
	case ViewWorkspace:
		m.workspace, cmd = m.workspace.Update(msg)
	case ViewDoctor:
		m.doctor, cmd = m.doctor.Update(msg)
	}
	return m, cmd
}

func (m *RootModel) View() string {
	if m.width > 0 && m.width < minWidth {
		return "Terminal too narrow — resize to at least 80 columns"
	}

	innerWidth := max(78, m.width-2)
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.sidebarView(), m.contentView())
	if m.showHelp {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", centeredOverlay(innerWidth, m.helpOverlay()))
	}
	if m.showSettings {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", centeredOverlay(innerWidth, m.settings.View()))
	}
	if m.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", errorText(m.err.Error()))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Render(strings.Join([]string{
			m.headerView(),
			body,
			m.footerView(),
		}, "\n"))
}

func (m *RootModel) handleGlobalKeys(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return true, tea.Quit
	case "?", "f1":
		m.showHelp = !m.showHelp
		return true, nil
	case ",":
		m.showSettings = true
		m.showHelp = false
		m.settings.SetConfig(m.cfg)
		m.dashboard.SetConfig(m.settings.Current())
		m.applyLayout()
		return true, nil
	case "1":
		return true, m.switchView(ViewDashboard)
	case "2":
		return true, m.switchView(ViewSuites)
	case "3":
		return true, m.switchView(ViewLogs)
	case "4":
		return true, m.switchView(ViewWorkspace)
	case "5":
		return true, m.switchView(ViewDoctor)
	case "tab":
		return true, m.switchView((m.activeView + 1) % 5)
	case "shift+tab":
		next := m.activeView - 1
		if next < 0 {
			next = ViewDoctor
		}
		return true, m.switchView(next)
	case "p":
		m.cfg.ActivePreset = nextPresetName(m.cfg)
		m.dashboard.SetConfig(m.cfg)
		return true, nil
	case "ctrl+b":
		m.toggleSidebar()
		return true, nil
	case "b":
		if m.activeView != ViewSuites && m.activeView != ViewWorkspace {
			m.toggleSidebar()
			return true, nil
		}
	case "esc":
		if m.showHelp {
			m.showHelp = false
			return true, nil
		}
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
	case ViewDashboard:
		m.dashboard.Deactivate()
	case ViewSuites:
		m.suites.Deactivate()
	case ViewLogs:
		m.logs.Deactivate()
	case ViewWorkspace:
		m.workspace.Deactivate()
	case ViewDoctor:
		m.doctor.Deactivate()
	}
}

func (m *RootModel) activateView(view View) tea.Cmd {
	switch view {
	case ViewDashboard:
		return m.dashboard.Activate()
	case ViewSuites:
		return m.suites.Activate()
	case ViewLogs:
		return m.logs.Activate()
	case ViewWorkspace:
		return m.workspace.Activate()
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
	m.dashboard.SetConfig(cfg)
}

func (m *RootModel) applyLayout() {
	contentWidth := m.contentWidth()
	contentHeight := m.contentHeight()
	m.dashboard.SetSize(contentWidth, contentHeight)
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
	return style.Render(m.activeContent())
}

func (m *RootModel) sidebarView() string {
	lines := make([]string, 0, 7)
	if m.sidebar == SidebarExpanded {
		lines = append(lines, gradientText("gradient"), "")
	}
	for _, view := range []View{ViewDashboard, ViewSuites, ViewLogs, ViewWorkspace, ViewDoctor} {
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
		lines = append(lines, style.Width(m.sidebarWidth() - 4).Render(" "+icon+"  "+label))
	}
	if m.sidebar == SidebarExpanded {
		lines = append(lines, "", mutedText(" Settings"))
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
	case ViewDashboard:
		return m.dashboard.View()
	case ViewSuites:
		return m.suites.View()
	case ViewLogs:
		return m.logs.View()
	case ViewWorkspace:
		return m.workspace.View()
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
		BorderForeground(lipgloss.Color(ColorGold)).
		Padding(0, 1)
	return box.Render(m.activeHelp())
}

func (m *RootModel) activeHelp() string {
	switch m.activeView {
	case ViewDashboard:
		return m.dashboard.HelpView()
	case ViewSuites:
		return m.suites.HelpView()
	case ViewLogs:
		return m.logs.HelpView()
	case ViewWorkspace:
		return m.workspace.HelpView()
	case ViewDoctor:
		return m.doctor.HelpView()
	default:
		return ""
	}
}

func (m *RootModel) headerView() string {
	wordmark := gradientText("gradient linux")
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
		Render("tab next · shift+tab prev · b sidebar · , settings · p presets · F1 help · q quit")
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

func sidebarIcon(view View) string {
	switch view {
	case ViewDashboard:
		return "󰕮"
	case ViewSuites:
		return "󰣘"
	case ViewLogs:
		return "󰈙"
	case ViewWorkspace:
		return "󰉋"
	case ViewDoctor:
		return "󰓙"
	default:
		return "•"
	}
}

func sidebarLabel(view View) string {
	switch view {
	case ViewDashboard:
		return "Dashboard"
	case ViewSuites:
		return "Suites"
	case ViewLogs:
		return "Logs"
	case ViewWorkspace:
		return "Workspace"
	case ViewDoctor:
		return "Doctor"
	default:
		return ""
	}
}

func centeredOverlay(width int, body string) string {
	return lipgloss.PlaceHorizontal(max(40, width), lipgloss.Center, body)
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
