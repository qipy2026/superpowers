package engine

import (
	"context"
	"fmt"
	"math"
	"time"
)

type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return fmt.Sprintf("retryable: %s", e.Err) }
func (e *RetryableError) Unwrap() error { return e.Err }

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

type RetryManager struct{ cfg RetryConfig }

func NewRetryManager(cfg RetryConfig) *RetryManager {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	return &RetryManager{cfg: cfg}
}

func (r *RetryManager) Do(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if _, ok := err.(*RetryableError); !ok {
			return err
		}
		if attempt == r.cfg.MaxRetries {
			break
		}
		delay := time.Duration(float64(r.cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > r.cfg.MaxDelay {
			delay = r.cfg.MaxDelay
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("exhausted %d retries: %w", r.cfg.MaxRetries, lastErr)
}
