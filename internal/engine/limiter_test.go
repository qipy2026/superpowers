package engine

import (
    "testing"
    "time"
)

func TestTokenBucketAllowsWithinLimit(t *testing.T) {
    lim := NewRateLimiter(10) // 10 tokens/sec
    allowed := 0
    for i := 0; i < 10; i++ {
        if lim.Allow() {
            allowed++
        }
    }
    if allowed != 10 {
        t.Errorf("expected 10 allowed, got %d", allowed)
    }
}

func TestTokenBucketBlocksOverLimit(t *testing.T) {
    lim := NewRateLimiter(10)
    for i := 0; i < 10; i++ {
        lim.Allow()
    }
    if lim.Allow() {
        t.Error("expected false after exhausting tokens")
    }
}

func TestTokenBucketRefills(t *testing.T) {
    lim := NewRateLimiter(100)
    for i := 0; i < 100; i++ {
        lim.Allow()
    }
    if lim.Allow() {
        t.Error("should be empty")
    }
    time.Sleep(50 * time.Millisecond)
    if !lim.Allow() {
        t.Error("should have refilled at least 1 token after 50ms")
    }
}

func TestMultiLimiter(t *testing.T) {
    ml := NewMultiLimiter(10) // global 10/sec
    ml.SetAdapterLimit("a", 5)
    // adapter 'a' can't exceed 5
    allowed := 0
    for i := 0; i < 10; i++ {
        if ml.Allow("a") {
            allowed++
        }
    }
    if allowed > 5 {
        t.Errorf("adapter limit breached: %d > 5", allowed)
    }
}
