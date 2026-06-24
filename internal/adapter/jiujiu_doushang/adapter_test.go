package jiujiu_doushang

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestCollect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`<html><body>
          <div class="data-card"><span class="title">数据项1</span><span class="value">1234</span></div>
          <div class="data-card"><span class="title">数据项2</span><span class="value">5678</span></div>
        </body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil, nil, nil, nil)
    task := &adapter.Task{ID: "t", Adapter: "jiujiu_doushang"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil { t.Fatalf("Collect failed: %v", err) }
    if len(rows) == 0 { t.Fatal("expected data") }
    t.Logf("collected %d items", len(rows))
}
