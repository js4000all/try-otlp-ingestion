package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/js4000all/try-otlp-ingestion/internal/generator"
	"github.com/js4000all/try-otlp-ingestion/internal/otlphttp"
)

func parseStart(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid -start %q: use YYYY-MM-DD (UTC) or RFC3339", s)
}

type options struct {
	start     time.Time
	days      int
	count     int
	countMode bool
	interval  time.Duration
	seed      int64
	configDir string
	output    string
	endpoint  string
}

func parseOptions(args []string) (options, error) {
	var opt options
	fs := flag.NewFlagSet("otlpgen", flag.ContinueOnError)
	startArg := fs.String("start", time.Now().UTC().Format("2006-01-02"), "YYYY-MM-DD (UTC) or RFC3339")
	days := fs.Int("days", 1, "number of calendar days (period mode)")
	count := fs.Int("count", 0, "number of samples starting now (count mode)")
	interval := fs.Duration("interval", time.Minute, "sample interval (e.g. 1m, 10s)")
	seed := fs.Int64("seed", 1, "random seed")
	configDir := fs.String("config", "config", "configuration directory")
	output := fs.String("output", "stdout", "output: stdout or otlp")
	endpoint := fs.String("otlp-endpoint", "http://otel-collector:4318/v1/metrics", "full OTLP/HTTP metrics URL")
	if err := fs.Parse(args); err != nil {
		return opt, err
	}
	if fs.NArg() != 0 {
		return opt, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	opt.countMode = seen["count"]
	if *interval <= 0 {
		return opt, fmt.Errorf("-interval must be positive")
	}
	if opt.countMode {
		if *count <= 0 {
			return opt, fmt.Errorf("-count must be positive")
		}
		if seen["start"] || seen["days"] {
			return opt, fmt.Errorf("-count cannot be combined with -start or -days")
		}
		// The actual start time is captured after configuration loading.
	} else {
		if *days <= 0 {
			return opt, fmt.Errorf("-days must be positive")
		}
		start, err := parseStart(*startArg)
		if err != nil {
			return opt, err
		}
		opt.start = start
	}
	opt.days = *days
	opt.count = *count
	opt.interval = *interval
	opt.seed = *seed
	if *output != "stdout" && *output != "otlp" {
		return opt, fmt.Errorf("invalid -output %q: use stdout or otlp", *output)
	}
	opt.output = *output
	opt.endpoint = *endpoint
	if opt.output == "otlp" {
		if _, err := otlphttp.New(opt.endpoint); err != nil {
			return opt, err
		}
	}
	opt.configDir = *configDir
	return opt, nil
}

func run(args []string) error {
	opt, err := parseOptions(args)
	if err != nil {
		return err
	}
	cfg, err := generator.Load(opt.configDir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var start time.Time
	if opt.countMode {
		start = time.Now()
	} else {
		start = opt.start
	}
	g := generator.New(cfg, opt.seed, start)

	if opt.output == "otlp" {
		client, err := otlphttp.New(opt.endpoint)
		if err != nil {
			return err
		}
		if opt.countMode {
			return g.RunCountTo(ctx, opt.count, opt.interval, client.Send)
		}
		return g.RunTo(ctx, opt.days, opt.interval, client.Send)
	}

	if opt.countMode {
		return g.RunCount(ctx, os.Stdout, opt.count, opt.interval)
	}
	return g.Run(ctx, os.Stdout, opt.days, opt.interval)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "otlpgen:", err)
		os.Exit(1)
	}
}
