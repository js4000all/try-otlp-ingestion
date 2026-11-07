// Package otlphttp converts generated samples into the official OTLP/HTTP JSON
// message structure and sends them to an OTLP Metrics receiver.
package otlphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/js4000all/try-otlp-ingestion/internal/generator"
)

type Client struct {
	endpoint string
	http     *http.Client
}

// New requires a full OTLP metrics URL (including /v1/metrics for a Collector).
func New(endpoint string) (*Client, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("invalid OTLP metrics endpoint %q: expected http(s) URL with a path", endpoint)
	}
	return &Client{endpoint: endpoint, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

// Send posts one timestamp's metrics as an OTLP ExportMetricsServiceRequest.
func (c *Client) Send(ctx context.Context, batch generator.Batch) error {
	payload, err := Marshal(batch)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create OTLP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post OTLP metrics: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("OTLP endpoint returned HTTP %d: %s", response.StatusCode, body)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

// The following types implement the OTLP JSON mapping (not the generator's
// human-readable JSONL format). uint64 timestamps are encoded as strings.
type exportRequest struct {
	ResourceMetrics []resourceMetrics `json:"resourceMetrics"`
}

type resourceMetrics struct {
	Resource     otlpResource   `json:"resource"`
	ScopeMetrics []scopeMetrics `json:"scopeMetrics"`
}

type otlpResource struct {
	Attributes []keyValue `json:"attributes"`
}

type scopeMetrics struct {
	Scope   scope    `json:"scope"`
	Metrics []metric `json:"metrics"`
}

type scope struct {
	Name string `json:"name"`
}

type keyValue struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}

type anyValue struct {
	StringValue string `json:"stringValue"`
}

type metric struct {
	Name  string `json:"name"`
	Unit  string `json:"unit"`
	Gauge *gauge `json:"gauge,omitempty"`
	Sum   *sum   `json:"sum,omitempty"`
}

type gauge struct {
	DataPoints []numberDataPoint `json:"dataPoints"`
}

type sum struct {
	DataPoints             []numberDataPoint `json:"dataPoints"`
	AggregationTemporality int               `json:"aggregationTemporality"`
	IsMonotonic            bool              `json:"isMonotonic"`
}

type numberDataPoint struct {
	Attributes        []keyValue `json:"attributes"`
	StartTimeUnixNano string     `json:"startTimeUnixNano,omitempty"`
	TimeUnixNano      string     `json:"timeUnixNano"`
	AsDouble          float64    `json:"asDouble"`
}

// Marshal translates Resource, Scope and DataPoint attributes without
// flattening them. Only Gauge and Cumulative/Delta Sum are supported.
func Marshal(batch generator.Batch) ([]byte, error) {
	request := exportRequest{ResourceMetrics: make([]resourceMetrics, 0, len(batch.ResourceMetrics))}
	for _, resource := range batch.ResourceMetrics {
		rm := resourceMetrics{
			Resource:     otlpResource{Attributes: attributes(resource.Resource.Attributes)},
			ScopeMetrics: make([]scopeMetrics, 0, len(resource.ScopeMetrics)),
		}

		for _, scoped := range resource.ScopeMetrics {
			sm := scopeMetrics{
				Scope:   scope{Name: scoped.Scope.Name},
				Metrics: make([]metric, 0, len(scoped.Metrics)),
			}
			for _, source := range scoped.Metrics {
				points := make([]numberDataPoint, 0, len(source.DataPoints))
				for _, point := range source.DataPoints {
					dp := numberDataPoint{
						Attributes:   attributes(point.Attributes),
						TimeUnixNano: fmt.Sprint(point.Timestamp.UnixNano()),
						AsDouble:     point.Value,
					}
					if point.StartTimestamp != nil {
						dp.StartTimeUnixNano = fmt.Sprint(point.StartTimestamp.UnixNano())
					}
					points = append(points, dp)
				}

				m := metric{Name: source.Name, Unit: source.Unit}
				switch source.Type {
				case "gauge":
					m.Gauge = &gauge{DataPoints: points}
				case "sum":
					temporality := 0
					switch source.Temporality {
					case "cumulative":
						temporality = 2
					case "delta":
						temporality = 1
					default:
						return nil, fmt.Errorf("unsupported temporality %q for %s", source.Temporality, source.Name)
					}
					m.Sum = &sum{
						DataPoints:             points,
						AggregationTemporality: temporality,
						IsMonotonic:            source.Monotonic,
					}
				default:
					return nil, fmt.Errorf("unsupported metric type %q for %s", source.Type, source.Name)
				}
				sm.Metrics = append(sm.Metrics, m)
			}
			rm.ScopeMetrics = append(rm.ScopeMetrics, sm)
		}
		request.ResourceMetrics = append(request.ResourceMetrics, rm)
	}
	return json.Marshal(request)
}

func attributes(input map[string]string) []keyValue {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([]keyValue, 0, len(keys))
	for _, key := range keys {
		values = append(values, keyValue{
			Key: key, Value: anyValue{StringValue: input[key]},
		})
	}
	return values
}
