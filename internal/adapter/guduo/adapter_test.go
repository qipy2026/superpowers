package guduo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
)

func TestGuduoCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<html><body>
  <div class="drama-list">
    <div class="drama-card">
      <span class="drama-name">庆余年2</span>
      <span class="heat-index">9876</span>
      <span class="platform">腾讯视频</span>
    </div>
    <div class="drama-card">
      <span class="drama-name">狐妖小红娘</span>
      <span class="heat-index">8765</span>
      <span class="platform">爱奇艺</span>
    </div>
  </div>
</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL, nil)
	task := &adapter.Task{ID: "t2", Adapter: "guduo"}
	rows, err := a.Collect(context.Background(), task)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected drama data")
	}
	t.Logf("collected %d dramas", len(rows))
}
