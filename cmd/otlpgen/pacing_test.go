package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/js4000all/try-otlp-ingestion/internal/generator"
)

func TestPaceSenderSpacing(t *testing.T) {
	const interval = 25 * time.Millisecond
	var starts []time.Time
	send := paceSender(func(_ context.Context, _ generator.Batch) error {
		starts = append(starts, time.Now())
		return nil
	}, interval)
	for i := 0; i < 3; i++ {
		if err := send(context.Background(), generator.Batch{}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < interval-2*time.Millisecond {
			t.Fatalf("request gap %v is shorter than %v", gap, interval)
		}
	}
}

func TestPaceSenderCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var sends int
	send := paceSender(func(_ context.Context, _ generator.Batch) error {
		sends++
		return nil
	}, time.Hour)
	if err := send(ctx, generator.Batch{}); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := send(ctx, generator.Batch{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
	if sends != 1 {
		t.Fatalf("sends=%d, want 1", sends)
	}
}

func TestPaceSenderError(t *testing.T) {
	want := errors.New("send failed")
	send := paceSender(func(_ context.Context, _ generator.Batch) error {
		return want
	}, time.Second)
	if err := send(context.Background(), generator.Batch{}); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
