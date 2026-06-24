package penguin_intelligence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestPenguinIntelligenceCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="post-item">
    <a href="/report/123"><h3>2024数字中国报告</h3></a>
    <span class="date">2024-06-10</span>
  </div>
  <div class="post-item">
    <a href="/report/456"><h3>AI时代的教育变革</h3></a>
    <span class="date">2024-05-28</span>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL)
	task := &adapter.Task{ID: "t5", Adapter: "penguin_intelligence"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected reports")
	}
	t.Logf("collected %d reports", len(rows))
}
