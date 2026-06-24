package model

import (
	"testing"
	"time"
)

func TestTaskFields(t *testing.T) {
	now := time.Now()
	task := Task{
		Adapter:   "stats_gov",
		Status:    "pending",
		StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
		StartedAt: now,
	}
	if task.Adapter != "stats_gov" {
		t.Errorf("expected stats_gov, got %s", task.Adapter)
	}
	if task.Status != "pending" {
		t.Errorf("expected pending, got %s", task.Status)
	}
}

func TestDataRowChecksum(t *testing.T) {
	row := DataRow{
		Adapter:   "stats_gov",
		SourceURL: "https://example.com/data",
		DataJSON:  `{"indicator":"GDP","value":"100"}`,
	}
	row.SetChecksum()
	if row.Checksum == "" {
		t.Error("checksum should not be empty")
	}
	expected := row.Checksum
	row.SetChecksum()
	if row.Checksum != expected {
		t.Error("checksum should be deterministic for same data")
	}
}

func TestCrawlConfigDefaults(t *testing.T) {
	cfg := CrawlConfig{
		Adapter:   "test",
		Enabled:   true,
		RateLimit: 10,
	}
	if cfg.CronExpr != "" {
		t.Errorf("default CronExpr should be empty, got %s", cfg.CronExpr)
	}
}
