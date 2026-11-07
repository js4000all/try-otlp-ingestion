package otlphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/js4000all/try-otlp-ingestion/internal/generator"
)

func testBatch() generator.Batch {
	at := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	return generator.Batch{ResourceMetrics: []generator.ResourceMetrics{{
		Resource: generator.Resource{Attributes: map[string]string{
			"monitoring.domain": "site_iot",
			"site_id":           "S001",
		}},
		ScopeMetrics: []generator.ScopeMetrics{{
			Scope: generator.Scope{Name: "plc-reader"},
			Metrics: []generator.Metric{{
				Name: "temperature", Unit: "Cel", Type: "gauge",
				DataPoints: []generator.DataPoint{{
					Timestamp: at, Value: 65.2, Attributes: map[string]string{"sensor_id": "T0001"},
				}},
			}, {
				Name: "plc_reader.read_errors", Unit: "{error}", Type: "sum",
				Monotonic: true, Temporality: "cumulative",
				DataPoints: []generator.DataPoint{{
					Timestamp: at, StartTimestamp: &at, Value: 12,
				}},
			}},
		}},
	}}}
}

func TestMarshalOTLP(t *testing.T) {
	data, err := Marshal(testBatch())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Metrics []struct {
					Gauge *struct {
						DataPoints []struct {
							AsDouble     float64 `json:"asDouble"`
							TimeUnixNano string  `json:"timeUnixNano"`
						} `json:"dataPoints"`
					} `json:"gauge"`
					Sum *struct {
						AggregationTemporality int  `json:"aggregationTemporality"`
						IsMonotonic            bool `json:"isMonotonic"`
					} `json:"sum"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	rm := payload.ResourceMetrics[0]
	if rm.Resource.Attributes[0].Key != "monitoring.domain" {
		t.Fatalf("attributes not preserved: %v", rm.Resource.Attributes)
	}
	metrics := rm.ScopeMetrics[0].Metrics
	if metrics[0].Gauge == nil || metrics[0].Gauge.DataPoints[0].AsDouble != 65.2 {
		t.Fatal("gauge not converted")
	}
	if metrics[0].Gauge.DataPoints[0].TimeUnixNano == "" {
		t.Fatal("timestamp missing")
	}
	if metrics[1].Sum == nil || metrics[1].Sum.AggregationTemporality != 2 || !metrics[1].Sum.IsMonotonic {
		t.Fatal("cumulative monotonic sum not converted")
	}
}

func TestSend(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/metrics" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("invalid JSON: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(server.URL + "/v1/metrics")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}
}

func TestSendHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(server.URL + "/v1/metrics")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), testBatch()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected HTTP 503, got %v", err)
	}
}

func TestNewInvalidEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "localhost:4318", "ftp://host/v1/metrics", "http://host", "http://host/v1/metrics?x=1"} {
		if _, err := New(endpoint); err == nil {
			t.Errorf("accepted endpoint %q", endpoint)
		}
	}
}
