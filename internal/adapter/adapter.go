package adapter

import (
	"context"
	"time"
)

type Task struct {
	ID        string    `json:"id"`
	Adapter   string    `json:"adapter"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Error     string    `json:"error,omitempty"`
	Stats     Stats     `json:"stats"`
}

type Stats struct {
	Collected int `json:"collected"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

type DataRow struct {
	SourceURL string            `json:"source_url"`
	Data      map[string]string `json:"data"`
}

type Adapter interface {
	Name() string
	Validate() error
	Collect(ctx context.Context, task *Task) ([]DataRow, error)
}
