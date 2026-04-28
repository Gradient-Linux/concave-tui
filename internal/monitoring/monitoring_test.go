package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProberShortCircuitsWhenURLIsEmpty(t *testing.T) {
	p := NewProber("", "")
	r := p.Probe(context.Background())
	if r.Prometheus.Configured || r.Grafana.Configured {
		t.Fatalf("Probe() configured = %+v, want both false", r)
	}
	if r.Prometheus.Reachable || r.Grafana.Reachable {
		t.Fatalf("Probe() reachable = %+v, want both false", r)
	}
}

func TestProberReachabilityAndVersions(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/-/healthy":
			_, _ = w.Write([]byte("Prometheus is healthy.\n"))
		case "/api/v1/status/buildinfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"version":"2.51.0"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer prom.Close()

	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"database":"ok","version":"10.4.0"}`)
	}))
	defer graf.Close()

	p := NewProber(prom.URL, graf.URL)
	r := p.Probe(context.Background())
	if !r.Prometheus.Reachable || r.Prometheus.Version != "2.51.0" {
		t.Fatalf("Prometheus = %+v, want reachable 2.51.0", r.Prometheus)
	}
	if !r.Grafana.Reachable || r.Grafana.Version != "10.4.0" {
		t.Fatalf("Grafana = %+v, want reachable 10.4.0", r.Grafana)
	}
}

func TestProberQueryReturnsFirstScalar(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("query") != "up" {
			t.Errorf("query = %q, want up", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1711376000,"3"]}]}}`)
	}))
	defer prom.Close()

	sample := NewProber(prom.URL, "").Query(context.Background(), "up")
	if sample.Error != "" {
		t.Fatalf("Query() error = %q", sample.Error)
	}
	if sample.Value != 3 {
		t.Fatalf("Query() value = %v, want 3", sample.Value)
	}
}

func TestProberQueryReportsPrometheusErrors(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"boom"}`)
	}))
	defer prom.Close()

	sample := NewProber(prom.URL, "").Query(context.Background(), "broken")
	if sample.Error != "boom" {
		t.Fatalf("Query() error = %q, want boom", sample.Error)
	}
}

func TestProberQueryShortCircuitsWhenURLMissing(t *testing.T) {
	sample := NewProber("", "").Query(context.Background(), "up")
	if sample.Error == "" {
		t.Fatal("Query() expected error, got none")
	}
}
