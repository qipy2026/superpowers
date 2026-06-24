package engine

import (
	"testing"
	"time"
)

func TestRodPoolBorrowReturn(t *testing.T) {
	pool := NewRodPool(2)
	if err := pool.Start(); err != nil {
		t.Skipf("browser not available: %v", err)
	}
	defer pool.Close()

	b1, err := pool.Borrow()
	if err != nil {
		t.Fatalf("Borrow failed: %v", err)
	}
	if b1 == nil {
		t.Fatal("expected non-nil browser")
	}
	pool.Return(b1)

	b2, err := pool.Borrow()
	if err != nil {
		t.Fatalf("second Borrow failed: %v", err)
	}
	pool.Return(b2)
}

func TestRodPoolExhausted(t *testing.T) {
	pool := NewRodPool(1)
	if err := pool.Start(); err != nil {
		t.Skipf("browser not available: %v", err)
	}
	defer pool.Close()

	b1, _ := pool.Borrow()
	b2, err := pool.BorrowTimeout(100 * time.Millisecond)
	if err == nil {
		pool.Return(b2)
		t.Error("expected timeout error when pool exhausted")
	}
	pool.Return(b1)
}
