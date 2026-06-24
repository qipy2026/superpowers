package tencent_research

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestTencentResearchCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="article-item">
    <h3><a href="/article/789">量子计算前沿报告</a></h3>
    <span class="time">2024-06-01</span>
  </div>
  <div class="article-item">
    <h3><a href="/article/012">数据安全白皮书</a></h3>
    <span class="time">2024-05-15</span>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL)
	task := &adapter.Task{ID: "t6", Adapter: "tencent_research"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected reports")
	}
	t.Logf("collected %d reports", len(rows))
}
