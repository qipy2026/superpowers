package iresearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestIResearchCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="report-list">
    <div class="report-item">
      <h3 class="title">2024年中国移动互联网报告</h3>
      <span class="date">2024-06-15</span>
      <span class="category">移动互联网</span>
    </div>
    <div class="report-item">
      <h3 class="title">2024年电商行业洞察</h3>
      <span class="date">2024-05-20</span>
      <span class="category">电子商务</span>
    </div>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL, nil) // nil RodPool -> Colly only
	task := &adapter.Task{ID: "t1", Adapter: "iresearch"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected report data")
	}
	t.Logf("collected %d reports", len(rows))
}
