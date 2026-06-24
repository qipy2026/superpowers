package engine

import (
	"context"
	"crawler/internal/adapter"
	"crawler/internal/model"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Repository interface {
	CreateTask(ctx context.Context, task *model.Task) error
	UpdateTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, id int64) (*model.Task, error)
	BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (int64, int64, error)
	GetLatestCollectedAt(ctx context.Context, adapterName string) (*time.Time, error)
	ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error)
	CountByAdapter(ctx context.Context) (map[string]int64, error)
	QueryData(ctx context.Context, params QueryParams) (*QueryResult, error)
	UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error
	ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error)
}

type QueryParams = struct {
	Adapter  string
	From     string
	To       string
	Keyword  string
	Page     int
	PageSize int
}

type QueryResult struct {
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Rows  []model.DataRow `json:"rows"`
}

type Engine struct {
	repo    Repository
	reg     *adapter.Registry
	limiter *MultiLimiter
	retry   *RetryManager
	logger  *zap.Logger
}

func New(repo Repository, reg *adapter.Registry, limiter *MultiLimiter, retry *RetryManager) *Engine {
	logger, _ := zap.NewProduction()
	return &Engine{repo: repo, reg: reg, limiter: limiter, retry: retry, logger: logger}
}

func (e *Engine) SetLogger(logger *zap.Logger) { e.logger = logger }

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (e *Engine) Collect(ctx context.Context, adapterName string) (int64, error) {
	a, ok := e.reg.Get(adapterName)
	if !ok {
		return 0, fmt.Errorf("adapter %q not found", adapterName)
	}
	task := &model.Task{
		Adapter:   adapterName,
		Status:    "running",
		StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
		StartedAt: time.Now(),
	}
	if err := e.repo.CreateTask(ctx, task); err != nil {
		return 0, fmt.Errorf("create task: %w", err)
	}
	go e.runCollection(context.Background(), task, a)
	return task.ID, nil
}

func (e *Engine) runCollection(ctx context.Context, dbTask *model.Task, a adapter.Adapter) {
	adTask := &adapter.Task{
		ID:        generateID(),
		Adapter:   a.Name(),
		Status:    "running",
		StartedAt: time.Now(),
	}
	var rows []adapter.DataRow
	var collectErr error

	// apply rate limit, then retry collection
	e.limiter.Allow(a.Name())
	collectErr = e.retry.Do(ctx, func() error {
		var err error
		rows, err = a.Collect(ctx, adTask)
		if err != nil {
			return &RetryableError{Err: err}
		}
		return nil
	})

	now := time.Now()
	if collectErr != nil {
		dbTask.Status = "failed"
		dbTask.Error = collectErr.Error()
		dbTask.EndedAt = &now
		dbTask.StatsJSON = fmt.Sprintf(`{"collected":0,"skipped":0,"errors":1}`)
		e.repo.UpdateTask(ctx, dbTask)
		return
	}

	collected := 0
	skipped := 0
	errors := 0

	// convert to model.DataRow and batch insert
	modelRows := make([]model.DataRow, 0, len(rows))
	for _, r := range rows {
		dataJSON := "{"
		for k, v := range r.Data {
			if len(dataJSON) > 1 {
				dataJSON += ","
			}
			dataJSON += fmt.Sprintf(`"%s":"%s"`, k, v)
		}
		dataJSON += "}"
		mr := model.DataRow{
			TaskID:      dbTask.ID,
			Adapter:     a.Name(),
			SourceURL:   r.SourceURL,
			DataJSON:    dataJSON,
			CollectedAt: now,
		}
		mr.SetChecksum()
		modelRows = append(modelRows, mr)
	}
	ins, skp, err := e.repo.BatchInsertDataRows(ctx, modelRows)
	collected = int(ins)
	skipped = int(skp)
	if err != nil {
		errors++
	}

	dbTask.Status = "done"
	dbTask.EndedAt = &now
	dbTask.StatsJSON = fmt.Sprintf(`{"collected":%d,"skipped":%d,"errors":%d}`, collected, skipped, errors)
	e.repo.UpdateTask(ctx, dbTask)
}
