package repository

import (
	"context"
	"crawler/internal/model"
	"errors"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type QueryParams struct {
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

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

func (r *Repository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.Task{}, &model.DataRow{}, &model.CrawlConfig{})
}

func (r *Repository) DB() *gorm.DB { return r.db }

// --- Task CRUD ---

func (r *Repository) CreateTask(ctx context.Context, task *model.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) UpdateTask(ctx context.Context, task *model.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *Repository) GetTask(ctx context.Context, id int64) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *Repository) ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// --- DataRow CRUD ---

func (r *Repository) BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (inserted, skipped int64, err error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "checksum"}},
		DoNothing: true,
	}).CreateInBatches(rows, 100)
	if result.Error != nil {
		return 0, 0, result.Error
	}
	inserted = result.RowsAffected
	skipped = int64(len(rows)) - inserted
	return inserted, skipped, nil
}

func (r *Repository) GetLatestCollectedAt(ctx context.Context, adapterName string) (*time.Time, error) {
	var row model.DataRow
	err := r.db.WithContext(ctx).
		Where("adapter = ?", adapterName).
		Order("collected_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row.CollectedAt, nil
}

func (r *Repository) QueryData(ctx context.Context, params QueryParams) (*QueryResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}
	q := r.db.WithContext(ctx).Model(&model.DataRow{})
	if params.Adapter != "" {
		q = q.Where("adapter = ?", params.Adapter)
	}
	if params.From != "" {
		q = q.Where("collected_at >= ?", params.From)
	}
	if params.To != "" {
		q = q.Where("collected_at <= ?", params.To)
	}
	if params.Keyword != "" {
		q = q.Where("data_json LIKE ?", "%"+params.Keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.DataRow
	offset := (params.Page - 1) * params.PageSize
	if err := q.Order("collected_at DESC").Offset(offset).Limit(params.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	return &QueryResult{Total: total, Page: params.Page, Rows: rows}, nil
}

func (r *Repository) CountByAdapter(ctx context.Context) (map[string]int64, error) {
	type count struct {
		Adapter string
		Count   int64
	}
	var results []count
	err := r.db.WithContext(ctx).Model(&model.DataRow{}).
		Select("adapter, count(*) as count").
		Group("adapter").Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, c := range results {
		m[c.Adapter] = c.Count
	}
	return m, nil
}

// --- CrawlConfig CRUD ---

func (r *Repository) UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "adapter"}},
		DoUpdates: clause.AssignmentColumns([]string{"cron_expr", "enabled", "rate_limit", "updated_at"}),
	}).Create(cfg).Error
}

func (r *Repository) ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error) {
	var configs []model.CrawlConfig
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}
