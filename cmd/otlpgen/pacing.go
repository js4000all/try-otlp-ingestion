package main

import (
	"context"
	"time"

	"github.com/js4000all/try-otlp-ingestion/internal/generator"
)

// paceSender enforces a minimum interval between request start times.
// Sends are sequential; slow requests do not accumulate additional delays.
func paceSender(send func(context.Context, generator.Batch) error, interval time.Duration) func(context.Context, generator.Batch) error {
	var next time.Time

	return func(ctx context.Context, batch generator.Batch) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !next.IsZero() {
			delay := time.Until(next)
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		next = time.Now().Add(interval)
		return send(ctx, batch)
	}
}
