package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChaojiyingSolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		if r.FormValue("user") != "testuser" {
			t.Error("expected testuser in form")
		}
		w.Write([]byte(`{"err_no":0,"err_str":"OK","pic_id":"123","pic_str":"AB3D"}`))
	}))
	defer srv.Close()

	solver := NewChaojiyingSolver("testuser", "testpass", "96001")
	solver.baseURL = srv.URL

	result, err := solver.Solve(context.Background(), []byte("fake-image-data"), "1902")
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if result.Code != "AB3D" {
		t.Errorf("expected AB3D, got %s", result.Code)
	}
	if result.ID != "123" {
		t.Errorf("expected id 123, got %s", result.ID)
	}
}

func TestChaojiyingSolveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"err_no":1001,"err_str":"no balance"}`))
	}))
	defer srv.Close()

	solver := NewChaojiyingSolver("testuser", "testpass", "96001")
	solver.baseURL = srv.URL

	_, err := solver.Solve(context.Background(), []byte("fake"), "1902")
	if err == nil {
		t.Fatal("expected error for no balance")
	}
	if !strings.Contains(err.Error(), "no balance") {
		t.Errorf("expected 'no balance' in error, got: %v", err)
	}
}

func TestChaojiyingReportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"err_no":0,"err_str":"OK"}`))
	}))
	defer srv.Close()

	solver := NewChaojiyingSolver("testuser", "testpass", "96001")
	solver.baseURL = srv.URL

	if err := solver.ReportError(context.Background(), "123"); err != nil {
		t.Fatalf("ReportError failed: %v", err)
	}
}
