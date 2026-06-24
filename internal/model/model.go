package model

import (
	"crypto/md5"
	"fmt"
	"time"
)

type Task struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Adapter   string     `gorm:"size:50;not null;index" json:"adapter"`
	Status    string     `gorm:"size:20;not null;default:pending" json:"status"`
	Error     string     `gorm:"type:text" json:"error,omitempty"`
	StatsJSON string     `gorm:"type:json" json:"stats_json"`
	StartedAt time.Time  `gorm:"not null" json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func (Task) TableName() string { return "tasks" }

type DataRow struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      int64     `gorm:"not null;index" json:"task_id"`
	Adapter     string    `gorm:"size:50;not null" json:"adapter"`
	SourceURL   string    `gorm:"size:500;not null" json:"source_url"`
	DataJSON    string    `gorm:"type:json;not null" json:"data_json"`
	CollectedAt time.Time `gorm:"not null" json:"collected_at"`
	Checksum    string    `gorm:"size:32;not null;uniqueIndex" json:"checksum"`
}

func (DataRow) TableName() string { return "data_rows" }

func (r *DataRow) SetChecksum() {
	raw := fmt.Sprintf("%s|%s", r.SourceURL, r.DataJSON)
	r.Checksum = fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

type CrawlConfig struct {
	Adapter   string    `gorm:"size:50;primaryKey" json:"adapter"`
	CronExpr  string    `gorm:"size:50;not null;default:''" json:"cron_expr"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	RateLimit int       `gorm:"not null;default:10" json:"rate_limit"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (CrawlConfig) TableName() string { return "crawl_configs" }
