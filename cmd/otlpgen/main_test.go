package main

import (
	"testing"
	"time"
)

func TestParseStart(t *testing.T) {
	date, err := parseStart("2026-11-06")
	if err != nil || !date.Equal(time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date=%v err=%v", date, err)
	}

	ts, err := parseStart("2026-11-06T12:30:00+09:00")
	if err != nil || ts.UTC().Hour() != 3 {
		t.Fatalf("timestamp=%v err=%v", ts, err)
	}

	if _, err := parseStart("2026/11/06"); err == nil {
		t.Fatal("invalid date accepted")
	}
}

func TestCountOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		ok        bool
		countMode bool
	}{
		{"single sample", []string{"-count", "1", "-interval", "10s"}, true, true},
		{"multiple samples", []string{"-count", "5", "-interval", "10s"}, true, true},
		{"zero count", []string{"-count", "0"}, false, false},
		{"negative count", []string{"-count", "-1"}, false, false},
		{"count and start", []string{"-count", "2", "-start", "2026-11-01"}, false, false},
		{"count and days", []string{"-count", "2", "-days", "1"}, false, false},
		{"count and default-valued start", []string{"-count", "2", "-start", "2026-11-07"}, false, false},
		{"bad interval", []string{"-count", "2", "-interval", "0s"}, false, false},
		{"period mode", []string{"-start", "2026-11-01", "-days", "2"}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := parseOptions(tt.args)
			if (err == nil) != tt.ok {
				t.Fatalf("parseOptions(%v) error = %v; want success %v", tt.args, err, tt.ok)
			}
			if tt.ok && opt.countMode != tt.countMode {
				t.Fatalf("countMode=%v want %v", opt.countMode, tt.countMode)
			}
		})
	}
}

func TestOutputOptions(t *testing.T) {
	for _, tt := range []struct {
		args  []string
		valid bool
	}{
		{[]string{"-output", "stdout"}, true},
		{[]string{"-output", "otlp", "-otlp-endpoint", "http://otel-collector:4318/v1/metrics"}, true},
		{[]string{"-output", "otlp", "-otlp-endpoint", "http://127.0.0.1:4318/v1/metrics"}, true},
		{[]string{"-output", "invalid"}, false},
		{[]string{"-output", "otlp", "-otlp-endpoint", "invalid"}, false},
	} {
		_, err := parseOptions(tt.args)
		if (err == nil) != tt.valid {
			t.Fatalf("parseOptions(%v): err=%v, want valid=%v", tt.args, err, tt.valid)
		}
	}
}

func TestSendIntervalOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
		ok   bool
	}{
		{"default", []string{"-output", "otlp"}, 0, true},
		{"pacing", []string{"-output", "otlp", "-send-interval", "100ms"}, 100 * time.Millisecond, true},
		{"disabled", []string{"-output", "otlp", "-send-interval", "0"}, 0, true},
		{"negative", []string{"-output", "otlp", "-send-interval", "-1s"}, 0, false},
		{"stdout rejected", []string{"-send-interval", "100ms"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt, err := parseOptions(tt.args)
			if (err == nil) != tt.ok {
				t.Fatalf("parseOptions(%v): err=%v want success=%v", tt.args, err, tt.ok)
			}
			if tt.ok && opt.sendInterval != tt.want {
				t.Fatalf("sendInterval=%v want %v", opt.sendInterval, tt.want)
			}
		})
	}
}
