package hangye_paihang

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestCollectIncremental(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="ranking-list">
    <div class="rank-item">
      <span class="rank">1</span>
      <span class="name">品牌A</span>
      <span class="score">98.5</span>
    </div>
    <div class="rank-item">
      <span class="rank">2</span>
      <span class="name">品牌B</span>
      <span class="score">95.2</span>
    </div>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL)
	task := &adapter.Task{ID: "t2", Adapter: "hangye_paihang"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected ranking data")
	}
	t.Logf("collected %d ranking items", len(rows))
}
