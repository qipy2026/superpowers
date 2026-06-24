package engine

import (
    "net/http"
    "os"
    "path/filepath"
    "testing"
)

func TestSessionManagerSaveLoad(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)

    cookies := []*http.Cookie{
        {Name: "session", Value: "abc123", Domain: "example.com"},
        {Name: "token", Value: "xyz789", Domain: "example.com"},
    }
    if err := sm.Save("test_site", cookies); err != nil {
        t.Fatalf("Save failed: %v", err)
    }
    loaded, err := sm.Load("test_site")
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if len(loaded) != 2 {
        t.Errorf("expected 2 cookies, got %d", len(loaded))
    }
    if loaded[0].Name != "session" || loaded[0].Value != "abc123" {
        t.Error("cookie mismatch")
    }
}

func TestSessionManagerLoadMissing(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)
    cookies, err := sm.Load("nonexistent")
    if err != nil {
        t.Fatalf("Load should not error on missing file: %v", err)
    }
    if cookies != nil {
        t.Error("expected nil for missing session")
    }
}

func TestSessionManagerFileCreated(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)
    sm.Save("test", []*http.Cookie{{Name: "a", Value: "b"}})
    expected := filepath.Join(dir, "test.json")
    if _, err := os.Stat(expected); os.IsNotExist(err) {
        t.Errorf("session file %s not created", expected)
    }
}
