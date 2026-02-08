package util

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// ExponentialBackoff executes fn with exponential backoff retry
func ExponentialBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	// Check if context has deadline
	deadline, hasDeadline := ctx.Deadline()

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(config.BaseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			// Check if delay would exceed context deadline
			if hasDeadline {
				timeUntilDeadline := time.Until(deadline)
				if delay >= timeUntilDeadline {
					return fmt.Errorf("cannot retry: delay would exceed context deadline: %w", lastErr)
				}
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			if !IsRetryable(err) {
				return err
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("max retry attempts reached: %w", lastErr)
}

// IsRetryable determines if error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	retryableErrors := []string{
		"RequestTimeout",
		"ServiceUnavailable",
		"Throttling",
		"ThrottlingException",
		"TooManyRequests",
		"connection reset",
		"connection refused",
	}

	for _, retryable := range retryableErrors {
		if contains(errMsg, retryable) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)*2))
}
