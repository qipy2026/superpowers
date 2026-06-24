package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySucceedsOnRetryableError(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	rm := NewRetryManager(cfg)
	attempts := 0
	err := rm.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return &RetryableError{Err: errors.New("timeout")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryGivesUp(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	rm := NewRetryManager(cfg)
	attempts := 0
	err := rm.Do(context.Background(), func() error {
		attempts++
		return &RetryableError{Err: errors.New("timeout")}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}

func TestNoRetryOnNonRetryableError(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	rm := NewRetryManager(cfg)
	attempts := 0
	err := rm.Do(context.Background(), func() error {
		attempts++
		return errors.New("parse error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable, got %d", attempts)
	}
}

func TestRetryRespectsContext(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second}
	rm := NewRetryManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := rm.Do(ctx, func() error {
		return &RetryableError{Err: errors.New("timeout")}
	})
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}
