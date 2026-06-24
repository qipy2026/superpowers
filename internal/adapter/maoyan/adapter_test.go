package maoyan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestMaoyanCollectNilRodPool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL, nil)
	task := &adapter.Task{ID: "t3", Adapter: "maoyan"}
	_, err := a.Collect(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when RodPool is nil")
	}
}

func TestMaoyanCollectStaticFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="movie-box">
    <span class="movie-name">热辣滚烫</span>
    <span class="box-office">34.6亿</span>
  </div>
  <div class="movie-box">
    <span class="movie-name">飞驰人生2</span>
    <span class="box-office">33.9亿</span>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL, nil)
	task := &adapter.Task{ID: "t4", Adapter: "maoyan"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected movie data from static fallback")
	}
	t.Logf("collected %d movies", len(rows))
}
