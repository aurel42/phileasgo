package probe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// CheckFunc is a function that performs a health check.
// It returns nil if the check passes, or an error if it fails.
type CheckFunc func(ctx context.Context) error

// Probe represents a single startup check.
type Probe struct {
	Name     string
	Check    CheckFunc
	Critical bool // If true, a failure here should prevent application startup.
}

// Result holds the outcome of a single probe.
type Result struct {
	Probe    Probe
	Error    error
	Duration time.Duration
}

// Run executes a list of probes and returns their results.
// It enforces a timeout for each check if the context doesn't already have one.
func Run(ctx context.Context, probes []Probe) []Result {
	results := make([]Result, len(probes))

	for i, p := range probes {
		start := time.Now()

		// Create a child context to ensure individual probes don't hang indefinitely
		// even if the parent context is long-lived.
		paramsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

		err := p.Check(paramsCtx)
		cancel()

		results[i] = Result{
			Probe:    p,
			Error:    err,
			Duration: time.Since(start),
		}
	}

	return results
}

// AnalyzeResults aggregates the results and returns a combined error if critical probes failed.
// It also logs the results using the provided logger or default slog.
func AnalyzeResults(results []Result) error {
	var criticalErrors []error

	slog.Info("Startup Checks Summary")

	for _, r := range results {
		status := "PASS"
		if r.Error != nil {
			status = "FAIL"
		}

		msg := fmt.Sprintf("[%s] %-20s (%v)", status, r.Probe.Name, r.Duration.Round(time.Millisecond))

		if r.Error != nil {
			slog.Error(msg, "error", r.Error)
			if r.Probe.Critical {
				criticalErrors = append(criticalErrors, fmt.Errorf("%s: %w", r.Probe.Name, r.Error))
			}
		} else {
			slog.Info(msg)
		}
	}

	if len(criticalErrors) > 0 {
		return errors.Join(criticalErrors...)
	}

	return nil
}
