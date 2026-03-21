package model

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Gradient-Linux/concave-tui/internal/config"
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
	loadStateFn          = config.LoadState
	saveStateFn          = config.SaveState
	addSuiteFn           = config.AddSuite
	removeSuiteFn        = config.RemoveSuite
	isInstalledFn        = config.IsInstalled
	loadManifestFn       = config.LoadManifest
	saveManifestFn       = config.SaveManifest
	recordInstallFn      = config.RecordInstall
	recordUpdateFn       = config.RecordUpdate
	swapRollbackFn       = config.SwapForRollback
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

type View int

const (
	ViewDashboard View = iota
	ViewSuites
	ViewLogs
	ViewWorkspace
	ViewDoctor
)

type RootModel struct {
	activeView View
	dashboard  DashboardModel
	suites     SuitesModel
	logs       LogsModel
	workspace  WorkspaceModel
	doctor     DoctorModel
	width      int
	height     int
	showHelp   bool
	err        error
	version    string
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

func NewRootModel(version string) *RootModel {
	return &RootModel{
		activeView: ViewDashboard,
		dashboard:  NewDashboardModel(),
		suites:     NewSuitesModel(),
		logs:       NewLogsModel(),
		workspace:  NewWorkspaceModel(),
		doctor:     NewDoctorModel(),
		version:    version,
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.activateView(m.activeView)
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard.SetSize(m.contentWidth(), m.contentHeight())
		m.suites.SetSize(m.contentWidth(), m.contentHeight())
		m.logs.SetSize(m.contentWidth(), m.contentHeight())
		m.workspace.SetSize(m.contentWidth(), m.contentHeight())
		m.doctor.SetSize(m.contentWidth(), m.contentHeight())
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "1":
			return m.switchView(ViewDashboard)
		case "2":
			return m.switchView(ViewSuites)
		case "3":
			return m.switchView(ViewLogs)
		case "4":
			return m.switchView(ViewWorkspace)
		case "5":
			return m.switchView(ViewDoctor)
		case "tab":
			return m.switchView((m.activeView + 1) % 5)
		case "shift+tab":
			next := m.activeView - 1
			if next < 0 {
				next = ViewDoctor
			}
			return m.switchView(next)
		}
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

	content := m.activeContent()
	if m.showHelp {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", m.helpOverlay())
	}

	bodyStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorMid)).
		Padding(0, 1)

	return bodyStyle.Render(strings.Join([]string{
		m.headerView(),
		content,
		m.footerView(),
	}, "\n"))
}

func (m *RootModel) switchView(next View) (tea.Model, tea.Cmd) {
	if next == m.activeView {
		return m, nil
	}
	m.deactivateView(m.activeView)
	m.activeView = next
	return m, m.activateView(next)
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
	title := gradientText("gradient")
	tabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
	tabs := []string{
		"1 Dashboard",
		"2 Suites",
		"3 Logs",
		"4 Workspace",
		"5 Doctor",
	}
	for idx, tab := range tabs {
		if View(idx) == m.activeView {
			tabs[idx] = activeStyle.Render(tab)
		} else {
			tabs[idx] = tabStyle.Render(tab)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("── "+strings.Join(tabs, " · ")+" ──"),
	)
}

func (m *RootModel) footerView() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Render("tab next · shift+tab prev · ? help · q quit")
}

func gradientText(text string) string {
	colors := []string{ColorDeep, ColorMid, ColorGold}
	var parts []string
	for idx, r := range text {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors[idx%len(colors)])).
			Bold(true).
			Render(string(r)))
	}
	return strings.Join(parts, "")
}

func (m *RootModel) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width - 4
}

func (m *RootModel) contentHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height - 8
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
		return suite.Suite{}, fmt.Errorf("forge has no recorded component selection")
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

func currentImageForFirstContainer(name string, manifest config.VersionManifest) string {
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
