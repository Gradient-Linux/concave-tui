// Package monitoring provides lightweight probes of the Prometheus and Grafana
// instances shipped with the Flow and Forge suites. It is consumed by the
// Monitoring view in the Bubble Tea TUI. The TUI does not own monitoring
// business logic — these helpers only read from upstream HTTP endpoints.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 4 * time.Second

// ServiceStatus represents the reachability of a single monitoring service.
type ServiceStatus struct {
	Configured bool
	Reachable  bool
	Version    string
	Error      string
}

// Reachability bundles the health of both Prometheus and Grafana.
type Reachability struct {
	Prometheus ServiceStatus
	Grafana    ServiceStatus
}

// Sample is a single PromQL instant-vector result reduced to a scalar.
type Sample struct {
	Query string
	Value float64
	Raw   string
	Error string
}

// Prober issues reachability and PromQL probes against configured URLs.
type Prober struct {
	client        *http.Client
	prometheusURL string
	grafanaURL    string
}

// NewProber builds a prober with sensible defaults. Either URL may be empty,
// in which case the corresponding probes short-circuit with Configured=false.
func NewProber(prometheusURL, grafanaURL string) *Prober {
	return &Prober{
		client:        &http.Client{Timeout: defaultTimeout},
		prometheusURL: strings.TrimRight(strings.TrimSpace(prometheusURL), "/"),
		grafanaURL:    strings.TrimRight(strings.TrimSpace(grafanaURL), "/"),
	}
}

// PrometheusURL returns the configured Prometheus base URL (without trailing slash).
func (p *Prober) PrometheusURL() string { return p.prometheusURL }

// GrafanaURL returns the configured Grafana base URL (without trailing slash).
func (p *Prober) GrafanaURL() string { return p.grafanaURL }

// Probe performs reachability checks against Prometheus and Grafana in
// parallel. A nil ctx defaults to context.Background().
func (p *Prober) Probe(ctx context.Context) Reachability {
	if ctx == nil {
		ctx = context.Background()
	}
	promCh := make(chan ServiceStatus, 1)
	grafCh := make(chan ServiceStatus, 1)
	go func() { promCh <- p.probePrometheus(ctx) }()
	go func() { grafCh <- p.probeGrafana(ctx) }()
	return Reachability{Prometheus: <-promCh, Grafana: <-grafCh}
}

func (p *Prober) probePrometheus(ctx context.Context) ServiceStatus {
	if p.prometheusURL == "" {
		return ServiceStatus{Configured: false}
	}
	status := ServiceStatus{Configured: true}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.prometheusURL+"/-/healthy", nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	resp, err := p.client.Do(req)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		status.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return status
	}
	status.Reachable = true

	buildReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.prometheusURL+"/api/v1/status/buildinfo", nil)
	if err == nil {
		if buildResp, err := p.client.Do(buildReq); err == nil {
			defer buildResp.Body.Close()
			if buildResp.StatusCode < 400 {
				var payload struct {
					Data struct {
						Version string `json:"version"`
					} `json:"data"`
				}
				_ = json.NewDecoder(buildResp.Body).Decode(&payload)
				status.Version = payload.Data.Version
			}
		}
	}
	return status
}

func (p *Prober) probeGrafana(ctx context.Context) ServiceStatus {
	if p.grafanaURL == "" {
		return ServiceStatus{Configured: false}
	}
	status := ServiceStatus{Configured: true}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.grafanaURL+"/api/health", nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	resp, err := p.client.Do(req)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		status.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return status
	}
	var payload struct {
		Database string `json:"database"`
		Version  string `json:"version"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	status.Reachable = true
	status.Version = payload.Version
	return status
}

// Query issues a Prometheus instant query and returns the first scalar-valued
// sample in the vector result. Errors are reported on the Sample.Error field
// so call sites can render them inline.
func (p *Prober) Query(ctx context.Context, expr string) Sample {
	sample := Sample{Query: expr}
	if p.prometheusURL == "" {
		sample.Error = "Prometheus not configured"
		return sample
	}
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := p.prometheusURL + "/api/v1/query?query=" + url.QueryEscape(expr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		sample.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return sample
	}
	var payload struct {
		Status    string `json:"status"`
		Error     string `json:"error"`
		ErrorType string `json:"errorType"`
		Data      struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value [2]any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		sample.Error = "decode: " + err.Error()
		return sample
	}
	if payload.Status == "error" {
		if payload.Error != "" {
			sample.Error = payload.Error
		} else {
			sample.Error = payload.ErrorType
		}
		return sample
	}
	if len(payload.Data.Result) == 0 {
		sample.Error = "no samples"
		return sample
	}
	raw, ok := payload.Data.Result[0].Value[1].(string)
	if !ok {
		sample.Error = "unexpected value shape"
		return sample
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		sample.Error = "parse: " + err.Error()
		return sample
	}
	sample.Value = value
	sample.Raw = raw
	return sample
}
