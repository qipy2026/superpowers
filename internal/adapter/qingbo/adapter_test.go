package qingbo

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestQingboCollect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`<html><body>
          <div class="rank-item"><span class="title">舆情指数1</span><span class="index">98.5</span></div>
          <div class="rank-item"><span class="title">舆情指数2</span><span class="index">92.3</span></div>
        </body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil, nil, nil, nil)
    task := &adapter.Task{ID: "t", Adapter: "qingbo"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil { t.Fatalf("Collect failed: %v", err) }
    if len(rows) == 0 { t.Fatal("expected data") }
    t.Logf("collected %d items", len(rows))
}
