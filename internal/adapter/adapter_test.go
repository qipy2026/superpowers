package adapter

import (
	"context"
	"testing"
)

type mockAdapter struct {
	name string
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Validate() error { return nil }
func (m *mockAdapter) Collect(ctx context.Context, task *Task) ([]DataRow, error) {
	return []DataRow{
		{SourceURL: "https://x.com/1", Data: map[string]string{"key": "value"}},
	}, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	a := &mockAdapter{name: "test"}
	reg.Register(a)
	got, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected to find test adapter")
	}
	if got.Name() != "test" {
		t.Errorf("expected 'test', got '%s'", got.Name())
	}
	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent adapter")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockAdapter{name: "a"})
	reg.Register(&mockAdapter{name: "b"})
	names := reg.List()
	if len(names) != 2 {
		t.Errorf("expected 2 adapters, got %d", len(names))
	}
}

func TestTaskAndStats(t *testing.T) {
	task := &Task{
		ID:      "t1",
		Adapter: "test",
		Status:  "running",
	}
	task.Stats.Collected = 100
	if task.Stats.Collected != 100 {
		t.Error("stats field not working")
	}
}
