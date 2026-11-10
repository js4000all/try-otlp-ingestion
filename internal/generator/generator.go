package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"time"
)

// These structures intentionally resemble OTLP's hierarchy; this is not OTLP wire JSON.
type Batch struct {
	ResourceMetrics []ResourceMetrics `json:"resourceMetrics"`
}

type ResourceMetrics struct {
	Resource     Resource       `json:"resource"`
	ScopeMetrics []ScopeMetrics `json:"scopeMetrics"`
}

type Resource struct {
	Attributes map[string]string `json:"attributes"`
}

type ScopeMetrics struct {
	Scope   Scope    `json:"scope"`
	Metrics []Metric `json:"metrics"`
}

type Scope struct {
	Name string `json:"name"`
}

type Metric struct {
	Name        string      `json:"name"`
	Unit        string      `json:"unit"`
	Type        string      `json:"type"`
	Monotonic   bool        `json:"monotonic,omitempty"`
	Temporality string      `json:"temporality,omitempty"`
	DataPoints  []DataPoint `json:"dataPoints"`
}

type DataPoint struct {
	Attributes     map[string]string `json:"attributes"`
	Timestamp      time.Time         `json:"timestamp"`
	StartTimestamp *time.Time        `json:"startTimestamp,omitempty"`
	Value          float64           `json:"value"`
}

type Generator struct {
	config Config
	states map[string]float64
	rng    *rand.Rand
	start  time.Time
}

func New(c Config, seed int64, start time.Time) *Generator {
	g := &Generator{
		config: c,
		states: make(map[string]float64),
		rng:    rand.New(rand.NewSource(seed)),
		start:  start,
	}

	return g
}

func (g *Generator) sample(key string, pattern Pattern, at time.Time) float64 {
	elapsed := at.Sub(g.start).Seconds()

	switch pattern.Pattern {
	case "sine":
		value := pattern.Baseline
		if pattern.Period > 0 {
			value += pattern.Amplitude * math.Sin(2*math.Pi*elapsed/pattern.Period)
		}
		return value + (g.rng.Float64()*2-1)*pattern.Noise

	case "random_walk":
		value, ok := g.states[key]
		if !ok {
			value = pattern.Baseline
		}
		value += (g.rng.Float64()*2 - 1) * pattern.Step
		value = math.Max(pattern.Min, math.Min(pattern.Max, value))
		g.states[key] = value
		return value

	case "spike":
		value := pattern.Baseline + (g.rng.Float64()*2-1)*pattern.Noise
		if g.rng.Float64() < pattern.SpikeProbability {
			value += pattern.SpikeAmplitude
		}
		return value

	case "toggle":
		if pattern.Interval <= 0 {
			return pattern.Initial
		}
		if int64(math.Floor(elapsed/pattern.Interval))%2 == 0 {
			return pattern.Initial
		}
		if pattern.Initial == 0 {
			return 1
		}
		return 0

	case "counter":
		value, ok := g.states[key]
		if !ok {
			value = pattern.Initial
		}
		if g.rng.Float64() < pattern.IncrementProbability {
			value += pattern.Increment
		}
		g.states[key] = value
		return value

	default:
		return 0
	}
}

// At groups all metrics by their collecting PLC Reader. Every Resource has
// one stable service identity, so backends can associate it with target_info.
func (g *Generator) At(at time.Time) Batch {
	type locatedTarget struct {
		site   Site
		area   Area
		target Target
	}
	targets := make(map[string]locatedTarget)
	for _, site := range g.config.Sites {
		for _, area := range site.Areas {
			for _, target := range area.Targets {
				targets[site.ID+"/"+target.ID] = locatedTarget{site: site, area: area, target: target}
			}
		}
	}

	result := Batch{ResourceMetrics: make([]ResourceMetrics, 0, len(g.config.Readers))}
	for _, reader := range g.config.Readers {
		metrics := make([]Metric, 0)
		index := make(map[string]int)

		// Merge data points of the same metric into a single metric per
		// Resource/Scope, avoiding duplicate metric definitions.
		appendMetrics := func(items []Metric) {
			for _, metric := range items {
				i, exists := index[metric.Name]
				if !exists {
					index[metric.Name] = len(metrics)
					metrics = append(metrics, metric)
					continue
				}
				metrics[i].DataPoints = append(metrics[i].DataPoints, metric.DataPoints...)
			}
		}

		for _, plc := range reader.PLCs {
			for _, targetID := range plc.Targets {
				target := targets[reader.SiteID+"/"+targetID]
				appendMetrics(g.siteMetrics(target.site, target.area, target.target, plc.ID, at))
			}
			appendMetrics(g.opsMetrics(reader, plc, at))
		}

		result.ResourceMetrics = append(result.ResourceMetrics, ResourceMetrics{
			Resource: Resource{Attributes: map[string]string{
				"service.namespace":   "industrial-monitoring",
				"service.name":        "plc-reader",
				"service.instance.id": reader.ID,
				"site_id":             reader.SiteID,
				"host.id":             reader.HostID,
			}},
			ScopeMetrics: []ScopeMetrics{{
				Scope:   Scope{Name: "plc-reader"},
				Metrics: metrics,
			}},
		})
	}
	return result
}

func (g *Generator) siteMetrics(site Site, area Area, target Target, plcID string, at time.Time) []Metric {
	targetType := g.config.TargetTypes[target.Type]
	metrics := make([]Metric, 0, len(targetType.Metrics))

	for _, def := range targetType.Metrics {
		metric := Metric{
			Name:       def.Name,
			Unit:       def.Unit,
			Type:       def.Type,
			DataPoints: []DataPoint{},
		}

		for _, sensor := range target.Sensors {
			if sensor.Metric != def.Name {
				continue
			}
			for _, channel := range def.Channels {
				if channel.Position != sensor.Position {
					continue
				}

				attributes := map[string]string{
					"monitoring.domain": "site_iot",
					"area":              area.ID,
					"target_id":         target.ID,
					"target_type":       target.Type,
					"sensor_id":         sensor.ID,
					"position":          sensor.Position,
					"source_type":       "plc",
					"source_id":         plcID,
				}
				if def.SignalType != "" {
					attributes["signal_type"] = def.SignalType
				}

				value := g.sample(site.ID+"/"+sensor.ID, channel.Generator, at)
				metric.DataPoints = append(metric.DataPoints, DataPoint{
					Attributes: attributes,
					Timestamp:  at,
					Value:      value,
				})
			}
		}
		if len(metric.DataPoints) > 0 {
			metrics = append(metrics, metric)
		}
	}

	return metrics
}

func (g *Generator) opsMetrics(reader Reader, plc PLC, at time.Time) []Metric {
	metrics := make([]Metric, 0, len(g.config.OpsMetrics["plc-reader"]))

	for _, def := range g.config.OpsMetrics["plc-reader"] {
		start := g.start
		value := g.sample(reader.SiteID+"/"+reader.ID+"/"+plc.ID+"/"+def.Name, def.Generator, at)
		metrics = append(metrics, Metric{
			Name:        def.Name,
			Unit:        def.Unit,
			Type:        def.Type,
			Monotonic:   def.Monotonic,
			Temporality: def.Temporality,
			DataPoints: []DataPoint{{
				Attributes:     map[string]string{"monitoring.domain": "ops", "source_id": plc.ID},
				StartTimestamp: &start,
				Timestamp:      at,
				Value:          value,
			}},
		})
	}

	return metrics
}

// Run writes JSON Lines to out for each timestamp in [start, start+days).
func (g *Generator) Run(ctx context.Context, out io.Writer, days int, interval time.Duration) error {
	encoder := json.NewEncoder(out)
	return g.RunTo(ctx, days, interval, func(_ context.Context, batch Batch) error {
		return encoder.Encode(batch)
	})
}

// RunTo emits each batch to a caller-provided destination, preserving timestamps.
func (g *Generator) RunTo(ctx context.Context, days int, interval time.Duration, emit func(context.Context, Batch) error) error {
	if days <= 0 || interval <= 0 {
		return fmt.Errorf("days and interval must be positive")
	}

	end := g.start.AddDate(0, 0, days)
	for at := g.start; at.Before(end); at = at.Add(interval) {
		if err := g.emitAt(ctx, at, emit); err != nil {
			return err
		}
	}
	return nil
}

// RunCount writes exactly count batches as JSON Lines, starting at g.start.
func (g *Generator) RunCount(ctx context.Context, out io.Writer, count int, interval time.Duration) error {
	encoder := json.NewEncoder(out)
	return g.RunCountTo(ctx, count, interval, func(_ context.Context, batch Batch) error {
		return encoder.Encode(batch)
	})
}

// RunCountTo sends count batches using start+i*interval (count-1 waits).
func (g *Generator) RunCountTo(ctx context.Context, count int, interval time.Duration, emit func(context.Context, Batch) error) error {
	if count <= 0 || interval <= 0 {
		return fmt.Errorf("count and interval must be positive")
	}

	for i := 0; i < count; i++ {
		at := g.start.Add(time.Duration(i) * interval)
		if err := g.emitAt(ctx, at, emit); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) emitAt(ctx context.Context, at time.Time, emit func(context.Context, Batch) error) error {
	if err := waitUntil(ctx, at); err != nil {
		return err
	}
	if err := emit(ctx, g.At(at)); err != nil {
		return fmt.Errorf("send timestamp %s: %w", at.Format(time.RFC3339Nano), err)
	}
	return nil
}

// waitUntil returns immediately for past timestamps and can be interrupted while waiting.
func waitUntil(ctx context.Context, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay := time.Until(at); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ctx.Err()
}
