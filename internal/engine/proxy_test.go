package engine

import (
	"testing"
)

func TestProxyPoolRotation(t *testing.T) {
	proxies := []string{
		"http://proxy1:8080",
		"http://proxy2:8080",
		"http://proxy3:8080",
	}
	pool := NewProxyPool(proxies)
	u1, err := pool.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if u1.Host == "" {
		t.Error("expected a proxy URL")
	}
	hosts := make(map[string]int)
	for i := 0; i < 6; i++ {
		u, _ := pool.Next()
		hosts[u.Host]++
	}
	if len(hosts) != 3 {
		t.Errorf("expected 3 distinct hosts, got %d", len(hosts))
	}
}

func TestProxyPoolEmptyConfig(t *testing.T) {
	pool := NewProxyPool(nil)
	u, err := pool.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if u != nil {
		t.Error("expected nil on empty pool")
	}
}

func TestProxyPoolMarkBad(t *testing.T) {
	proxies := []string{"http://proxy1:8080", "http://proxy2:8080"}
	pool := NewProxyPool(proxies)
	u, _ := pool.Next()
	pool.MarkBad(u)
	for i := 0; i < 4; i++ {
		u2, _ := pool.Next()
		if u2.Host == u.Host {
			t.Error("marked-bad proxy should not appear again")
		}
	}
}
