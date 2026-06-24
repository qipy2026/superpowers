package stats_gov

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestValidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><table class="data"><tr><td>GDP</td><td>100</td></tr></table></body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL)
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestCollectReturnsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <table class="MsoNormalTable">
    <tr><td>指标名称</td><td>2023年</td><td>2024年</td></tr>
    <tr><td>年末总人口</td><td>140967</td><td>140828</td></tr>
    <tr><td>国内生产总值</td><td>1260582</td><td>1349084</td></tr>
  </table>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL)
	task := &adapter.Task{ID: "t1", Adapter: "stats_gov"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected some data rows")
	}
	t.Logf("collected %d rows", len(rows))
	for _, row := range rows {
		if row.SourceURL == "" {
			t.Error("SourceURL should not be empty")
		}
		if len(row.Data) == 0 {
			t.Error("Data should not be empty")
		}
	}
}
