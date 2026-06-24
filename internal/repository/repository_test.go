package repository

import (
	"context"
	"crawler/internal/model"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *Repository {
	t.Helper()
	repo, err := New("crawler:crawler123@tcp(127.0.0.1:3306)/crawler_test?parseTime=true")
	if err != nil {
		t.Skipf("MySQL not available, skipping: %v", err)
	}
	repo.db.AutoMigrate(&model.Task{}, &model.DataRow{}, &model.CrawlConfig{})
	// clean slate
	repo.db.Exec("DELETE FROM data_rows")
	repo.db.Exec("DELETE FROM tasks")
	repo.db.Exec("DELETE FROM crawl_configs")
	return repo
}

func TestCreateAndGetTask(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	task := &model.Task{
		Adapter:   "test_adapter",
		Status:    "pending",
		StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
		StartedAt: time.Now(),
	}
	if err := repo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == 0 {
		t.Error("expected ID to be assigned")
	}
	got, err := repo.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Adapter != "test_adapter" {
		t.Errorf("expected test_adapter, got %s", got.Adapter)
	}
}

func TestUpsertConfig(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	cfg := &model.CrawlConfig{
		Adapter:   "test",
		CronExpr:  "0 */6 * * *",
		Enabled:   true,
		RateLimit: 5,
	}
	if err := repo.UpsertConfig(ctx, cfg); err != nil {
		t.Fatalf("UpsertConfig failed: %v", err)
	}
	cfgs, err := repo.ListEnabledConfigs(ctx)
	if err != nil {
		t.Fatalf("ListEnabledConfigs failed: %v", err)
	}
	found := false
	for _, c := range cfgs {
		if c.Adapter == "test" {
			found = true
			if c.RateLimit != 5 {
				t.Errorf("expected RateLimit=5, got %d", c.RateLimit)
			}
		}
	}
	if !found {
		t.Error("expected test adapter in config list")
	}
}

func TestBatchInsertDataRows(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()
	rows := []model.DataRow{
		{TaskID: 1, Adapter: "test", SourceURL: "https://a.com/1",
			DataJSON: `{"k":"v1"}`, CollectedAt: now},
		{TaskID: 1, Adapter: "test", SourceURL: "https://a.com/2",
			DataJSON: `{"k":"v2"}`, CollectedAt: now},
	}
	rows[0].SetChecksum()
	rows[1].SetChecksum()
	inserted, skipped, err := repo.BatchInsertDataRows(ctx, rows)
	if err != nil {
		t.Fatalf("BatchInsertDataRows failed: %v", err)
	}
	if inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
	// re-insert should skip duplicates
	inserted2, skipped2, _ := repo.BatchInsertDataRows(ctx, rows)
	if inserted2 != 0 {
		t.Errorf("expected 0 inserted on re-run, got %d", inserted2)
	}
	if skipped2 != 2 {
		t.Errorf("expected 2 skipped on re-run, got %d", skipped2)
	}
}

func TestQueryData(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()
	row := model.DataRow{
		TaskID: 1, Adapter: "stats_gov", SourceURL: "https://a.com/gdp",
		DataJSON:    `{"indicator":"GDP 总量","value":"126万亿"}`,
		CollectedAt: now,
	}
	row.SetChecksum()
	repo.BatchInsertDataRows(ctx, []model.DataRow{row})

	result, err := repo.QueryData(ctx, model.QueryParams{
		Adapter:  "stats_gov",
		Keyword:  "GDP",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("QueryData failed: %v", err)
	}
	if result.Total == 0 {
		t.Error("expected to find the GDP row")
	}
}
