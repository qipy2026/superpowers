package engine

import (
	"context"
	"crawler/internal/adapter"
	"crawler/internal/model"
	"errors"
	"testing"
	"time"
)

type stubRepo struct {
	tasks    map[int64]*model.Task
	dataRows []model.DataRow
	configs  map[string]model.CrawlConfig
}

func (s *stubRepo) CreateTask(ctx context.Context, t *model.Task) error {
	t.ID = int64(len(s.tasks) + 1)
	s.tasks[t.ID] = t
	return nil
}
func (s *stubRepo) UpdateTask(ctx context.Context, t *model.Task) error {
	s.tasks[t.ID] = t
	return nil
}
func (s *stubRepo) GetTask(ctx context.Context, id int64) (*model.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}
func (s *stubRepo) BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (int64, int64, error) {
	s.dataRows = append(s.dataRows, rows...)
	return int64(len(rows)), 0, nil
}
func (s *stubRepo) GetLatestCollectedAt(ctx context.Context, name string) (*time.Time, error) {
	return nil, nil
}
func (s *stubRepo) ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error) {
	var cfgs []model.CrawlConfig
	for _, c := range s.configs {
		if c.Enabled {
			cfgs = append(cfgs, c)
		}
	}
	return cfgs, nil
}
func (s *stubRepo) CountByAdapter(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}
func (s *stubRepo) QueryData(ctx context.Context, p QueryParams) (*QueryResult, error) {
	return nil, nil
}
func (s *stubRepo) UpsertConfig(ctx context.Context, c *model.CrawlConfig) error {
	return nil
}
func (s *stubRepo) ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error) {
	return nil, nil
}

type stubAdapter struct {
	name      string
	collectFn func(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error)
}

func (s *stubAdapter) Name() string { return s.name }
func (s *stubAdapter) Validate() error { return nil }
func (s *stubAdapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
	return s.collectFn(ctx, task)
}

func TestEngineCollectSuccess(t *testing.T) {
	repo := &stubRepo{
		tasks:   make(map[int64]*model.Task),
		configs: map[string]model.CrawlConfig{"test": {Adapter: "test", Enabled: true, RateLimit: 10}},
	}
	reg := adapter.NewRegistry()
	reg.Register(&stubAdapter{
		name: "test",
		collectFn: func(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
			return []adapter.DataRow{{SourceURL: "https://x.com/1", Data: map[string]string{"a": "1"}}}, nil
		},
	})
	ml := NewMultiLimiter(100)
	rm := NewRetryManager(RetryConfig{MaxRetries: 1, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond})
	eng := New(repo, reg, ml, rm)

	ctx := context.Background()
	taskID, err := eng.Collect(ctx, "test")
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if taskID <= 0 {
		t.Errorf("expected task ID > 0, got %d", taskID)
	}
	// wait for async collection
	time.Sleep(100 * time.Millisecond)
	task, _ := repo.GetTask(ctx, taskID)
	if task.Status != "done" {
		t.Errorf("expected done, got %s", task.Status)
	}
	if len(repo.dataRows) != 1 {
		t.Errorf("expected 1 data row, got %d", len(repo.dataRows))
	}
}

func TestEngineCollectAdapterNotFound(t *testing.T) {
	repo := &stubRepo{tasks: make(map[int64]*model.Task), configs: make(map[string]model.CrawlConfig)}
	reg := adapter.NewRegistry()
	eng := New(repo, reg, NewMultiLimiter(100), NewRetryManager(RetryConfig{}))
	_, err := eng.Collect(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent adapter")
	}
}
