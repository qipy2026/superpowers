package baidu_index

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"crawler/internal/adapter"
	"crawler/internal/engine"
)

type mockSolver struct{}

func (m *mockSolver) Solve(ctx context.Context, img []byte, typ string) (*engine.CaptchaResult, error) {
	return &engine.CaptchaResult{ID: "mock1", Code: "ABCD"}, nil
}
func (m *mockSolver) ReportError(ctx context.Context, id string) error { return nil }

func TestName(t *testing.T) {
	a := New("https://index.baidu.com", nil, nil, nil, nil, &mockSolver{})
	if a.Name() != "baidu_index" {
		t.Fatalf("expected baidu_index, got %s", a.Name())
	}
}

func TestValidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body>test</body></html>`))
	}))
	defer srv.Close()
	a := New(srv.URL, nil, nil, nil, nil, &mockSolver{})
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestCollectRequiresRod(t *testing.T) {
	a := New("https://index.baidu.com", nil, nil, nil, nil, &mockSolver{})
	task := &adapter.Task{ID: "t", Adapter: "baidu_index"}
	_, err := a.Collect(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when rodPool is nil")
	}
	t.Logf("correctly rejected nil rodPool: %v", err)
}
