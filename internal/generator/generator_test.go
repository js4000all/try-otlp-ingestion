package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleConfig(t *testing.T) Config {
	t.Helper()
	config, err := Load("../../config")
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func TestConfigAndSeries(t *testing.T) {
	config := sampleConfig(t)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	batch := New(config, 1, start).At(start)

	if got := len(batch.ResourceMetrics); got != 2 {
		t.Fatalf("resources=%d, want 2 PLC Readers", got)
	}

	byReader := make(map[string]ResourceMetrics)
	sitePoints, opsPoints := 0, 0
	areas := make(map[string]bool)
	for _, resource := range batch.ResourceMetrics {
		attrs := resource.Resource.Attributes
		if attrs["service.namespace"] != "industrial-monitoring" || attrs["service.name"] != "plc-reader" {
			t.Fatalf("missing service identity: %v", attrs)
		}
		readerID := attrs["service.instance.id"]
		if readerID == "" || byReader[readerID].Resource.Attributes != nil {
			t.Fatalf("missing or duplicate reader %q", readerID)
		}
		byReader[readerID] = resource
		if len(resource.ScopeMetrics) != 1 || resource.ScopeMetrics[0].Scope.Name != "plc-reader" {
			t.Fatalf("unexpected scope for %s", readerID)
		}

		names := make(map[string]bool)
		for _, scope := range resource.ScopeMetrics {
			for _, metric := range scope.Metrics {
				if names[metric.Name] {
					t.Fatalf("duplicated metric %q for reader %s", metric.Name, readerID)
				}
				names[metric.Name] = true
				for _, point := range metric.DataPoints {
					if !point.Timestamp.Equal(start) {
						t.Fatal("timestamp mismatch")
					}
					switch point.Attributes["monitoring.domain"] {
					case "site_iot":
						sitePoints++
						if point.Attributes["sensor_id"] == "" || point.Attributes["target_id"] == "" || point.Attributes["target_type"] == "" || point.Attributes["area"] == "" || point.Attributes["source_id"] == "" {
							t.Fatalf("missing target dimensions: %v", point.Attributes)
						}
						areas[attrs["site_id"]+"/"+point.Attributes["area"]] = true
					case "ops":
						opsPoints++
						if !metric.Monotonic || metric.Temporality != "cumulative" || point.StartTimestamp == nil {
							t.Fatal("invalid ops sum")
						}
						if point.Attributes["source_id"] == "" {
							t.Fatal("missing PLC id")
						}
					default:
						t.Fatalf("missing monitoring.domain: %v", point.Attributes)
					}
				}
			}
		}
	}
	if sitePoints != 16 || opsPoints != 3 {
		t.Fatalf("points site=%d ops=%d, want 16 and 3", sitePoints, opsPoints)
	}
	if len(areas) != 3 {
		t.Fatalf("site/area combinations=%v, want three", areas)
	}
	if got := byReader["reader_01"].Resource.Attributes["site_id"]; got != "S001" {
		t.Fatalf("reader_01 site=%q", got)
	}
	if got := byReader["reader_02"].Resource.Attributes["site_id"]; got != "S002" {
		t.Fatalf("reader_02 site=%q", got)
	}
}

func TestReproducibility(t *testing.T) {
	config := sampleConfig(t)
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	first, err := json.Marshal(New(config, 42, at).At(at))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(New(config, 42, at).At(at))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same seed returned different data")
	}
}

func TestHistoricalRun(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	g := New(sampleConfig(t), 1, start)
	var out bytes.Buffer
	if err := g.Run(context.Background(), &out, 1, 12*time.Hour); err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Split(strings.TrimSpace(out.String()), "\n")); got != 2 {
		t.Fatalf("lines=%d, want 2", got)
	}
}

func TestFutureRunCancellation(t *testing.T) {
	start := time.Now().Add(time.Hour)
	g := New(sampleConfig(t), 1, start)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := g.Run(ctx, &out, 1, time.Hour); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("future data emitted early")
	}
}

func TestInvalidConfig(t *testing.T) {
	config := sampleConfig(t)
	config.Sites[0].Areas[0].Targets[0].Sensors[0].Metric = "missing"
	if err := config.Validate(); err == nil {
		t.Fatal("expected validation failure")
	}
}

func TestRunCountSingleImmediate(t *testing.T) {
	start := time.Now().Add(-time.Second)
	g := New(sampleConfig(t), 1, start)
	var out bytes.Buffer
	if err := g.RunCount(context.Background(), &out, 1, time.Hour); err != nil {
		t.Fatal(err)
	}

	var batch Batch
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.ResourceMetrics) != 2 {
		t.Fatalf("resources=%d want 2", len(batch.ResourceMetrics))
	}
	for _, resource := range batch.ResourceMetrics {
		for _, scope := range resource.ScopeMetrics {
			for _, metric := range scope.Metrics {
				for _, point := range metric.DataPoints {
					if !point.Timestamp.Equal(start) {
						t.Fatalf("timestamp=%v want %v", point.Timestamp, start)
					}
				}
			}
		}
	}
}

func TestRunCountMultipleScheduled(t *testing.T) {
	start := time.Now().Add(-time.Second)
	interval := 15 * time.Millisecond
	g := New(sampleConfig(t), 1, start)
	var out bytes.Buffer
	if err := g.RunCount(context.Background(), &out, 4, interval); err != nil {
		t.Fatal(err)
	}

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 4 {
		t.Fatalf("lines=%d want 4", len(lines))
	}
	for i, line := range lines {
		var batch Batch
		if err := json.Unmarshal(line, &batch); err != nil {
			t.Fatal(err)
		}
		got := batch.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].DataPoints[0].Timestamp
		want := start.Add(time.Duration(i) * interval)
		if !got.Equal(want) {
			t.Fatalf("sample %d timestamp=%v want %v", i, got, want)
		}
	}
}

func TestRunCountFutureCancellation(t *testing.T) {
	start := time.Now().Add(time.Hour)
	g := New(sampleConfig(t), 1, start)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	if err := g.RunCount(ctx, &out, 3, time.Minute); err != context.Canceled {
		t.Fatalf("got %v want context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Fatal("future data emitted early")
	}
}

func TestRunCountInvalid(t *testing.T) {
	g := New(sampleConfig(t), 1, time.Now())
	tests := []struct {
		count    int
		interval time.Duration
	}{
		{0, time.Second},
		{-1, time.Second},
		{1, 0},
		{1, -time.Second},
	}
	for _, tt := range tests {
		if err := g.RunCount(context.Background(), &bytes.Buffer{}, tt.count, tt.interval); err == nil {
			t.Fatalf("accepted count=%d interval=%v", tt.count, tt.interval)
		}
	}
}

func TestWaitUntilFutureCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitUntil(ctx, time.Now().Add(time.Hour)); err != context.Canceled {
		t.Fatalf("got %v want context.Canceled", err)
	}
}
