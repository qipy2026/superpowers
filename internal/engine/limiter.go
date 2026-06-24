package engine

import (
    "sync"
    "time"
)

type RateLimiter struct {
    rate       float64    // tokens per second
    tokens     float64
    lastRefill time.Time
    mu         sync.Mutex
}

func NewRateLimiter(ratePerSec float64) *RateLimiter {
    return &RateLimiter{
        rate:       ratePerSec,
        tokens:     ratePerSec,
        lastRefill: time.Now(),
    }
}

func (r *RateLimiter) Allow() bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    now := time.Now()
    elapsed := now.Sub(r.lastRefill).Seconds()
    r.tokens += elapsed * r.rate
    if r.tokens > r.rate {
        r.tokens = r.rate
    }
    r.lastRefill = now
    if r.tokens >= 1 {
        r.tokens--
        return true
    }
    return false
}

type MultiLimiter struct {
    global     *RateLimiter
    perAdapter map[string]*RateLimiter
    mu         sync.RWMutex
}

func NewMultiLimiter(globalRate float64) *MultiLimiter {
    return &MultiLimiter{
        global:     NewRateLimiter(globalRate),
        perAdapter: make(map[string]*RateLimiter),
    }
}

func (m *MultiLimiter) SetAdapterLimit(name string, rate float64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.perAdapter[name] = NewRateLimiter(rate)
}

func (m *MultiLimiter) Allow(adapter string) bool {
    if !m.global.Allow() {
        return false
    }
    m.mu.RLock()
    lim, ok := m.perAdapter[adapter]
    m.mu.RUnlock()
    if !ok {
        return true
    }
    return lim.Allow()
}
