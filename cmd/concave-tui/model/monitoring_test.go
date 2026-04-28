package model

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	"github.com/Gradient-Linux/concave-tui/internal/monitoring"
)

type stubMonitoringProber struct {
	reachability monitoring.Reachability
	samples      map[string]monitoring.Sample
}

func (s stubMonitoringProber) Probe(ctx context.Context) monitoring.Reachability {
	return s.reachability
}

func (s stubMonitoringProber) Query(ctx context.Context, expr string) monitoring.Sample {
	if sample, ok := s.samples[expr]; ok {
		return sample
	}
	return monitoring.Sample{Query: expr, Error: "no stub"}
}

func TestMonitoringModel_NoURLsConfigured(t *testing.T) {
	restoreModelDeps(t)
	m := NewMonitoringModel()
	m.SetRole(tuiauth.RoleViewer)
	if view := m.View(); !strings.Contains(view, "No monitoring URLs configured") {
		t.Fatalf("View() = %q", view)
	}
}

func TestMonitoringModel_RendersReachabilityAndSamples(t *testing.T) {
	restoreModelDeps(t)

	reach := monitoring.Reachability{
		Prometheus: monitoring.ServiceStatus{Configured: true, Reachable: true, Version: "2.51.0"},
		Grafana:    monitoring.ServiceStatus{Configured: true, Reachable: true, Version: "10.4.0"},
	}
	samples := map[string]monitoring.Sample{}
	for _, metric := range monitoringMetrics {
		samples[metric.query] = monitoring.Sample{Query: metric.query, Value: 42}
	}
	newMonitoringProberFn = func(_ monitoringSettings) monitoringProber {
		return stubMonitoringProber{reachability: reach, samples: samples}
	}

	m := NewMonitoringModel()
	m.SetRole(tuiauth.RoleViewer)
	m.SetConfig("http://prom", "http://graf")
	cmd := m.Activate()
	if cmd == nil {
		t.Fatal("Activate() returned nil cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Activate cmd returned %T, want BatchMsg", msg)
	}
	var loaded monitoringLoadedMsg
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		if l, ok := sub().(monitoringLoadedMsg); ok {
			loaded = l
			break
		}
	}
	if loaded.probe.Samples == nil {
		t.Fatal("no loaded probe returned from activate")
	}
	model, _ := m.Update(loaded)
	view := model.View()
	for _, expected := range []string{"Prometheus:", "Grafana:", "reachable", "Scrape targets up", "CPU busy", "GPU utilisation"} {
		if !strings.Contains(view, expected) {
			t.Errorf("View() missing %q:\n%s", expected, view)
		}
	}
}

func TestMonitoringModel_ShowsUnreachable(t *testing.T) {
	restoreModelDeps(t)

	newMonitoringProberFn = func(_ monitoringSettings) monitoringProber {
		return stubMonitoringProber{
			reachability: monitoring.Reachability{
				Prometheus: monitoring.ServiceStatus{Configured: true, Reachable: false, Error: "connection refused"},
				Grafana:    monitoring.ServiceStatus{Configured: true, Reachable: false, Error: "connection refused"},
			},
		}
	}

	m := NewMonitoringModel()
	m.SetRole(tuiauth.RoleViewer)
	m.SetConfig("http://prom", "http://graf")
	cmd := m.Activate()
	msg := cmd().(tea.BatchMsg)
	var loaded monitoringLoadedMsg
	for _, sub := range msg {
		if l, ok := sub().(monitoringLoadedMsg); ok {
			loaded = l
		}
	}
	model, _ := m.Update(loaded)
	view := model.View()
	if !strings.Contains(view, "unreachable") || !strings.Contains(view, "connection refused") {
		t.Fatalf("expected unreachable error in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Prometheus unreachable") {
		t.Fatalf("expected per-metric error, got:\n%s", view)
	}
}
