package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	"github.com/gorilla/websocket"
)

const defaultBaseURL = "http://127.0.0.1:7777"

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type APIError struct {
	Status  int
	Message string
}

type SessionResponse struct {
	Token     string       `json:"token"`
	Username  string       `json:"username"`
	Role      tuiauth.Role `json:"role"`
	ExpiresAt time.Time    `json:"expires_at"`
}

type ContainerInfo struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Current  string `json:"current,omitempty"`
	Previous string `json:"previous,omitempty"`
}

type PortMapping struct {
	Host        int    `json:"host"`
	Container   int    `json:"container"`
	Port        int    `json:"port"`
	Service     string `json:"service"`
	Description string `json:"description,omitempty"`
}

type SuiteSummary struct {
	Name          string          `json:"name"`
	Installed     bool            `json:"installed"`
	State         string          `json:"state"`
	Current       string          `json:"current,omitempty"`
	Previous      string          `json:"previous,omitempty"`
	Ports         []PortMapping   `json:"ports"`
	Containers    []ContainerInfo `json:"containers"`
	GPURequired   bool            `json:"gpu_required"`
	Error         string          `json:"error,omitempty"`
	ComposeExists bool            `json:"compose_exists"`
}

type SuitesResponse struct {
	Suites []SuiteSummary `json:"suites"`
}

type WorkspacePayload struct {
	Root   string            `json:"root"`
	Total  uint64            `json:"total"`
	Free   uint64            `json:"free"`
	Used   uint64            `json:"used"`
	Usages map[string]uint64 `json:"usages"`
}

type DoctorCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Recovery string `json:"recovery,omitempty"`
}

type DoctorResponse struct {
	Checks []DoctorCheck `json:"checks"`
}

type SystemService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	User   string `json:"user,omitempty"`
}

type SystemInfo struct {
	Hostname string          `json:"hostname"`
	Uptime   string          `json:"uptime"`
	Kernel   string          `json:"kernel"`
	OS       string          `json:"os"`
	Concave  string          `json:"concave"`
	Docker   string          `json:"docker"`
	Services []SystemService `json:"services"`
}

type ActivityContainer struct {
	Name       string  `json:"name"`
	Suite      string  `json:"suite"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemoryMiB  float64 `json:"memory_mib,omitempty"`
}

type UserActivity struct {
	Username     string              `json:"username"`
	Role         tuiauth.Role        `json:"role"`
	Containers   []ActivityContainer `json:"containers"`
	GPUMemoryMiB int                 `json:"gpu_memory_mib"`
	LastActive   time.Time           `json:"last_active,omitempty"`
}

type UsersActivityResponse struct {
	Users []UserActivity `json:"users"`
}

type DriftTier int

type PackageDiff struct {
	Name     string    `json:"name"`
	Baseline string    `json:"baseline"`
	Current  string    `json:"current"`
	Tier     DriftTier `json:"tier"`
	Reason   string    `json:"reason"`
}

type DriftReport struct {
	Group     string        `json:"group"`
	User      string        `json:"user"`
	Timestamp time.Time     `json:"timestamp"`
	Diffs     []PackageDiff `json:"diffs"`
	Clean     bool          `json:"clean"`
}

type ResolverStatus struct {
	Available     *bool         `json:"available,omitempty"`
	Message       string        `json:"message,omitempty"`
	Running       bool          `json:"running,omitempty"`
	LastScan      time.Time     `json:"last_scan,omitempty"`
	GroupReports  []DriftReport `json:"group_reports,omitempty"`
	SnapshotCount int           `json:"snapshot_count,omitempty"`
	SocketPath    string        `json:"socket_path,omitempty"`
}

type FleetNode struct {
	Hostname        string    `json:"hostname"`
	MachineID       string    `json:"machine_id"`
	GradientVersion string    `json:"gradient_version"`
	Visibility      string    `json:"visibility"`
	InstalledSuites []string  `json:"installed_suites"`
	ResolverRunning bool      `json:"resolver_running"`
	LastSeen        time.Time `json:"last_seen"`
	Address         string    `json:"address"`
}

type FleetResponse struct {
	Available *bool       `json:"available,omitempty"`
	Message   string      `json:"message,omitempty"`
	Count     int         `json:"count,omitempty"`
	Peers     []FleetNode `json:"peers,omitempty"`
}

type TeamQuota struct {
	CPUCores    float64 `json:"cpu_cores"`
	MemoryGB    float64 `json:"memory_gb"`
	GPUFraction float64 `json:"gpu_fraction"`
	GPUMemoryGB float64 `json:"gpu_memory_gb"`
	IOWeightPct int     `json:"io_weight_pct"`
}

type TeamSummary struct {
	Name        string    `json:"name"`
	Preset      string    `json:"preset"`
	Users       []string  `json:"users"`
	Quota       TeamQuota `json:"quota"`
	GPUStrategy string    `json:"gpu_strategy"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type TeamsResponse struct {
	Available *bool         `json:"available,omitempty"`
	Message   string        `json:"message,omitempty"`
	Teams     []TeamSummary `json:"teams,omitempty"`
}

type JobSnapshot struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Lines       []string       `json:"lines"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
}

type suiteActionResponse struct {
	JobID string `json:"job_id"`
}

type labURLResponse struct {
	URL string `json:"url"`
}

type changelogEntry struct {
	Container string `json:"container"`
	From      string `json:"from"`
	To        string `json:"to"`
}

type ChangelogResponse struct {
	Suite   string           `json:"suite"`
	Changes []changelogEntry `json:"changes"`
}

type WebsocketEvent struct {
	Type string `json:"type"`
	Line string `json:"line,omitempty"`
	Data string `json:"data,omitempty"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func DefaultBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("CONCAVE_API_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return defaultBaseURL
}

func New(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL()
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) Login(ctx context.Context, username, password string) (tuiauth.Session, error) {
	payload := map[string]string{"username": username, "password": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return tuiauth.Session{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/api/v1/auth/login"), bytes.NewReader(body))
	if err != nil {
		return tuiauth.Session{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Concave-Client", "tui")
	var response SessionResponse
	if err := c.do(req, &response); err != nil {
		return tuiauth.Session{}, err
	}
	session := tuiauth.Session{
		Token:     response.Token,
		Username:  response.Username,
		Role:      response.Role,
		ExpiresAt: response.ExpiresAt,
	}
	c.SetToken(session.Token)
	return session, nil
}

func (c *Client) Me(ctx context.Context) (tuiauth.Session, error) {
	var response SessionResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/auth/me", nil)
	if err != nil {
		return tuiauth.Session{}, err
	}
	if err := c.do(req, &response); err != nil {
		return tuiauth.Session{}, err
	}
	return tuiauth.Session{
		Token:     c.token,
		Username:  response.Username,
		Role:      response.Role,
		ExpiresAt: response.ExpiresAt,
	}, nil
}

func (c *Client) Refresh(ctx context.Context) (tuiauth.Session, error) {
	var response SessionResponse
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/auth/refresh", nil)
	if err != nil {
		return tuiauth.Session{}, err
	}
	req.Header.Set("X-Concave-Client", "tui")
	if err := c.do(req, &response); err != nil {
		return tuiauth.Session{}, err
	}
	session := tuiauth.Session{
		Token:     response.Token,
		Username:  response.Username,
		Role:      response.Role,
		ExpiresAt: response.ExpiresAt,
	}
	c.SetToken(session.Token)
	return session, nil
}

func (c *Client) Logout(ctx context.Context) error {
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/auth/logout", nil)
	if err != nil {
		return err
	}
	c.SetToken("")
	return c.do(req, nil)
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/api/v1/health"), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) ListSuites(ctx context.Context) ([]SuiteSummary, error) {
	var response SuitesResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/suites", nil)
	if err != nil {
		return nil, err
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}
	return response.Suites, nil
}

func (c *Client) GetSuite(ctx context.Context, name string) (SuiteSummary, error) {
	var response SuiteSummary
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/suites/"+url.PathEscape(name), nil)
	if err != nil {
		return SuiteSummary{}, err
	}
	if err := c.do(req, &response); err != nil {
		return SuiteSummary{}, err
	}
	return response, nil
}

func (c *Client) Workspace(ctx context.Context) (WorkspacePayload, error) {
	var payload WorkspacePayload
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/workspace", nil)
	if err != nil {
		return WorkspacePayload{}, err
	}
	if err := c.do(req, &payload); err != nil {
		return WorkspacePayload{}, err
	}
	return payload, nil
}

func (c *Client) WorkspaceBackup(ctx context.Context) (string, error) {
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/workspace/backup", nil)
	if err != nil {
		return "", err
	}
	var response suiteActionResponse
	if err := c.do(req, &response); err != nil {
		return "", err
	}
	return response.JobID, nil
}

func (c *Client) WorkspaceClean(ctx context.Context) (string, error) {
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/workspace/clean", nil)
	if err != nil {
		return "", err
	}
	var response suiteActionResponse
	if err := c.do(req, &response); err != nil {
		return "", err
	}
	return response.JobID, nil
}

func (c *Client) Doctor(ctx context.Context) ([]DoctorCheck, error) {
	var response DoctorResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/doctor", nil)
	if err != nil {
		return nil, err
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}
	return response.Checks, nil
}

func (c *Client) SystemInfo(ctx context.Context) (SystemInfo, error) {
	var response SystemInfo
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/system/info", nil)
	if err != nil {
		return SystemInfo{}, err
	}
	if err := c.do(req, &response); err != nil {
		return SystemInfo{}, err
	}
	return response, nil
}

func (c *Client) UsersActivity(ctx context.Context) ([]UserActivity, error) {
	var response UsersActivityResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/users/activity", nil)
	if err != nil {
		return nil, err
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}
	return response.Users, nil
}

func (c *Client) ResolverStatus(ctx context.Context) (ResolverStatus, error) {
	var response ResolverStatus
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/env/status", nil)
	if err != nil {
		return ResolverStatus{}, err
	}
	if err := c.do(req, &response); err != nil {
		return ResolverStatus{}, err
	}
	return response, nil
}

func (c *Client) FleetStatus(ctx context.Context) (FleetResponse, error) {
	var response FleetResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/fleet/status", nil)
	if err != nil {
		return FleetResponse{}, err
	}
	if err := c.do(req, &response); err != nil {
		return FleetResponse{}, err
	}
	return response, nil
}

func (c *Client) NodeStatus(ctx context.Context) (FleetNode, error) {
	var response FleetNode
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/node/status", nil)
	if err != nil {
		return FleetNode{}, err
	}
	if err := c.do(req, &response); err != nil {
		return FleetNode{}, err
	}
	return response, nil
}

func (c *Client) Teams(ctx context.Context) (TeamsResponse, error) {
	var response TeamsResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/teams", nil)
	if err != nil {
		return TeamsResponse{}, err
	}
	if err := c.do(req, &response); err != nil {
		return TeamsResponse{}, err
	}
	return response, nil
}

func (c *Client) SystemAction(ctx context.Context, action string) error {
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/system/"+action, map[string]bool{"confirm": true})
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) StartSuiteAction(ctx context.Context, name, action string, body any) (string, error) {
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/api/v1/suites/"+url.PathEscape(name)+"/"+action, body)
	if err != nil {
		return "", err
	}
	var response suiteActionResponse
	if err := c.do(req, &response); err != nil {
		return "", err
	}
	return response.JobID, nil
}

func (c *Client) Job(ctx context.Context, id string) (JobSnapshot, error) {
	var response JobSnapshot
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/jobs/"+url.PathEscape(id), nil)
	if err != nil {
		return JobSnapshot{}, err
	}
	if err := c.do(req, &response); err != nil {
		return JobSnapshot{}, err
	}
	return response, nil
}

func (c *Client) LabURL(ctx context.Context, name string) (string, error) {
	var response labURLResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/suites/"+url.PathEscape(name)+"/lab", nil)
	if err != nil {
		return "", err
	}
	if err := c.do(req, &response); err != nil {
		return "", err
	}
	return response.URL, nil
}

func (c *Client) Changelog(ctx context.Context, name string) (ChangelogResponse, error) {
	var response ChangelogResponse
	req, err := c.newJSONRequest(ctx, http.MethodGet, "/api/v1/suites/"+url.PathEscape(name)+"/changelog", nil)
	if err != nil {
		return ChangelogResponse{}, err
	}
	if err := c.do(req, &response); err != nil {
		return ChangelogResponse{}, err
	}
	return response, nil
}

func (c *Client) DialLogs(ctx context.Context, suiteName, container string) (*websocket.Conn, error) {
	wsURL, err := url.Parse(strings.Replace(c.baseURL, "http://", "ws://", 1))
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(c.baseURL, "https://") {
		wsURL, err = url.Parse(strings.Replace(c.baseURL, "https://", "wss://", 1))
		if err != nil {
			return nil, err
		}
	}
	wsURL.Path = "/api/v1/suites/" + url.PathEscape(suiteName) + "/logs"
	query := wsURL.Query()
	if container != "" {
		query.Set("container", container)
	}
	wsURL.RawQuery = query.Encode()
	headers := http.Header{}
	if c.token != "" {
		headers.Set("Authorization", "Bearer "+c.token)
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL.String(), headers)
	return conn, err
}

func (c *Client) url(path string) string {
	return c.baseURL + path
}

func (c *Client) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && strings.TrimSpace(payload["error"]) != "" {
			return &APIError{Status: resp.StatusCode, Message: payload["error"]}
		}
		return &APIError{Status: resp.StatusCode, Message: resp.Status}
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Status == http.StatusUnauthorized
}
