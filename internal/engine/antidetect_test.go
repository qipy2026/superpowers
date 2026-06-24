package engine

import (
    "net/http"
    "testing"
    "time"
)

func TestAntiDetectRandomUA(t *testing.T) {
    ad := NewAntiDetect(nil)
    ua1 := ad.RandomUA()
    if ua1 == "" {
        t.Error("UA should not be empty")
    }
    found := false
    for i := 0; i < 10; i++ {
        if ad.RandomUA() != ua1 {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected UA rotation")
    }
}

func TestAntiDetectRandomDelay(t *testing.T) {
    ad := NewAntiDetect(nil)
    start := time.Now()
    ad.RandomDelay(50*time.Millisecond, 100*time.Millisecond)
    elapsed := time.Since(start)
    if elapsed < 50*time.Millisecond {
        t.Error("delay too short")
    }
}

func TestAntiDetectSetHeaders(t *testing.T) {
    ad := NewAntiDetect(nil)
    req, _ := http.NewRequest("GET", "https://example.com", nil)
    ad.SetHeaders(req, "https://www.google.com")
    if req.Header.Get("User-Agent") == "" {
        t.Error("User-Agent not set")
    }
    if req.Header.Get("Referer") == "" {
        t.Error("Referer not set")
    }
    if req.Header.Get("Accept-Language") == "" {
        t.Error("Accept-Language not set")
    }
}
