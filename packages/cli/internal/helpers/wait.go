package helpers

import (
	"context"
	"fmt"
	"time"
)

// WaitConfig configures polling behavior for wait operations.
type WaitConfig struct {
	// Timeout is the maximum duration to wait before giving up.
	Timeout time.Duration

	// Interval is the duration between status checks.
	Interval time.Duration

	// ProgressChar is printed on each poll iteration (e.g., ".").
	// Set to empty string to disable progress output.
	ProgressChar string
}

// DefaultWaitConfig returns sensible defaults for waiting operations.
func DefaultWaitConfig() WaitConfig {
	return WaitConfig{
		Timeout:      5 * time.Minute,
		Interval:     2 * time.Second,
		ProgressChar: ".",
	}
}

// BuildWaitConfig returns defaults optimized for build operations (longer timeout).
func BuildWaitConfig() WaitConfig {
	return WaitConfig{
		Timeout:      10 * time.Minute,
		Interval:     5 * time.Second,
		ProgressChar: ".",
	}
}

// WaitResult represents the outcome of a single status check.
type WaitResult int

const (
	// WaitContinue indicates the operation should continue polling.
	WaitContinue WaitResult = iota

	// WaitSuccess indicates the operation completed successfully.
	WaitSuccess

	// WaitFailure indicates the operation failed (stop polling with error).
	WaitFailure
)

// StatusFunc is called on each polling iteration.
// It should return:
//   - WaitSuccess when the desired state is reached
//   - WaitFailure when an unrecoverable error occurs
//   - WaitContinue to keep polling
//   - An error if the status check itself failed (will be ignored, polling continues)
type StatusFunc func(ctx context.Context) (WaitResult, error)

// WaitFor polls the given status function until success, failure, or timeout.
// This consolidates the duplicate waitForBuild/waitForDeployment patterns.
//
// Example:
//
//	err := WaitFor(ctx, "deployment", DefaultWaitConfig(), func(ctx context.Context) (WaitResult, error) {
//	    status, err := client.GetDeploymentStatus(ctx, deploymentID)
//	    if err != nil {
//	        return WaitContinue, err // Ignore transient errors
//	    }
//	    switch status {
//	    case "ready":
//	        return WaitSuccess, nil
//	    case "failed":
//	        return WaitFailure, fmt.Errorf("deployment failed")
//	    default:
//	        return WaitContinue, nil
//	    }
//	})
func WaitFor(ctx context.Context, operation string, config WaitConfig, statusFn StatusFunc) error {
	timeout := time.After(config.Timeout)
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return NewTimeoutError(operation, formatDuration(config.Timeout))

		case <-ticker.C:
			result, err := statusFn(ctx)
			switch result {
			case WaitSuccess:
				return nil
			case WaitFailure:
				if err != nil {
					return err
				}
				return fmt.Errorf("%s failed", operation)
			case WaitContinue:
				if config.ProgressChar != "" {
					fmt.Print(config.ProgressChar)
				}
				continue
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForWithProgress is like WaitFor but prints a custom progress indicator
// based on the status check result.
//
// Example:
//
//	err := WaitForWithProgress(ctx, "deployment", DefaultWaitConfig(), func(ctx context.Context) (WaitResult, string, error) {
//	    status, err := client.GetDeploymentStatus(ctx, deploymentID)
//	    if err != nil {
//	        return WaitContinue, "?", err
//	    }
//	    switch status {
//	    case "ready":
//	        return WaitSuccess, "✓", nil
//	    case "failed":
//	        return WaitFailure, "✗", fmt.Errorf("deployment failed")
//	    case "unhealthy":
//	        return WaitContinue, "⚠", nil
//	    default:
//	        return WaitContinue, ".", nil
//	    }
//	})
func WaitForWithProgress(ctx context.Context, operation string, config WaitConfig, statusFn func(ctx context.Context) (WaitResult, string, error)) error {
	timeout := time.After(config.Timeout)
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return NewTimeoutError(operation, formatDuration(config.Timeout))

		case <-ticker.C:
			result, progressChar, err := statusFn(ctx)
			switch result {
			case WaitSuccess:
				return nil
			case WaitFailure:
				if err != nil {
					return err
				}
				return fmt.Errorf("%s failed", operation)
			case WaitContinue:
				if progressChar != "" {
					fmt.Print(progressChar)
				}
				continue
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	if d >= time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%d hours %d minutes", hours, mins)
		}
		return fmt.Sprintf("%d hours", hours)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%d seconds", int(d.Seconds()))
}
